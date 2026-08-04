// Package queue is a scripted stub of the queue business logic, written so the
// API can be exercised end to end before the real implementation exists. It keeps
// no stock accounting and no FIFO queue — the outcome depends only on the request
// and on how much time passed since the user entered:
//
//	product_id "sold-out"   → 409 SOLD_OUT
//	quantity > offerLimit   → OFFER_PENDING (offerLimit units offered)
//	otherwise               → QUEUED, and RIGHT_ACTIVE after queueDelay
//	PATCH on an offer       → RIGHT_ACTIVE
//	DELETE on an offer      → DECLINED, on anything else → membership dropped
//	POST /rights/…/events   → PURCHASED
//	right left to expire    → back to QUEUED, then RIGHT_ACTIVE again
//
// Everything lives in a map behind one mutex and is lost on restart. The real
// implementation (PostgreSQL for product_count, Redis for the FIFO queue — see
// docs/c4_container.md) replaces this type without touching the handlers.
package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backend/internal/models"
)

const (
	// queueDelay is how long a user stays QUEUED before the right is handed out,
	// so the waiting screen has something to show.
	queueDelay = 5 * time.Second
	// offerLimit turns any larger request into a partial offer of this size.
	offerLimit = 3
	// soldOutProductID is the product that always answers SOLD_OUT.
	soldOutProductID = "sold-out"
)

type member struct {
	models.Membership
	waitingSince time.Time
}

// Service is the scripted stub of the queue business logic.
type Service struct {
	mu       sync.Mutex
	rightTTL time.Duration
	offerTTL time.Duration

	members map[string]*member
	rights  map[string]*member
}

// New creates the stub. rightTTL is the lifetime of an issued purchase right,
// offerTTL the lifetime of a partial offer awaiting a decision.
func New(rightTTL, offerTTL time.Duration) *Service {
	return &Service{
		rightTTL: rightTTL,
		offerTTL: offerTTL,
		members:  make(map[string]*member),
		rights:   make(map[string]*member),
	}
}

// Join implements "add me to the queue for this product". A repeated call by a
// user who already has a membership returns the current state unchanged.
func (s *Service) Join(_ context.Context, productID, userID string, quantity int) (models.Membership, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if productID == soldOutProductID {
		return models.Membership{}, models.ErrSoldOut
	}
	if quantity < 1 {
		return models.Membership{}, fmt.Errorf("%w: %d", models.ErrInvalidQuantity, quantity)
	}

	if m, ok := s.members[key(productID, userID)]; ok {
		s.advance(m)
		return m.Membership, nil
	}

	m := &member{
		Membership: models.Membership{
			ProductID: productID,
			UserID:    userID,
			Requested: quantity,
			Status:    models.StatusQueued,
		},
		waitingSince: time.Now(),
	}
	if quantity > offerLimit {
		s.grantOffer(m)
	}
	s.members[key(productID, userID)] = m

	return m.Membership, nil
}

// Status returns the user's current membership, moving the script forward first.
func (s *Service) Status(_ context.Context, productID, userID string) (models.Membership, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.members[key(productID, userID)]
	if !ok {
		return models.Membership{}, models.ErrMembershipNotFound
	}
	s.advance(m)

	return m.Membership, nil
}

// AcceptOffer turns an OFFER_PENDING membership into a right for the given quantity.
func (s *Service) AcceptOffer(_ context.Context, productID, userID string, quantity int) (models.Membership, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.members[key(productID, userID)]
	if !ok {
		return models.Membership{}, models.ErrMembershipNotFound
	}
	s.advance(m)

	if m.Status != models.StatusOfferPending {
		return models.Membership{}, models.ErrNoPendingOffer
	}
	if quantity < 1 || quantity > m.AvailableQuantity {
		return models.Membership{}, fmt.Errorf("%w: want %d, offered %d", models.ErrInvalidQuantity, quantity, m.AvailableQuantity)
	}

	m.AvailableQuantity = 0
	s.grantRight(m, quantity)

	return m.Membership, nil
}

// Leave ends the user's participation: turning down an offer is the terminal
// DECLINED, anything else simply drops the membership.
func (s *Service) Leave(_ context.Context, productID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.members[key(productID, userID)]
	if !ok {
		return models.ErrMembershipNotFound
	}
	s.advance(m)

	delete(s.rights, m.Token)

	if m.Status == models.StatusOfferPending {
		m.Membership = models.Membership{
			ProductID: productID,
			UserID:    userID,
			Status:    models.StatusDeclined,
		}

		return nil
	}

	delete(s.members, key(productID, userID))

	return nil
}

// ReportPayment is the effect of the Order Service reporting a successful payment:
// the right is used up and the membership reaches the terminal PURCHASED.
func (s *Service) ReportPayment(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.rights[token]
	if !ok {
		return models.ErrRightNotFound
	}
	delete(s.rights, token)

	quantity := m.Quantity
	m.Membership = models.Membership{
		ProductID: m.ProductID,
		UserID:    m.UserID,
		Status:    models.StatusPurchased,
		Quantity:  quantity,
	}

	return nil
}

// advance moves the script forward. The asymmetry is deliberate: an expired right
// sends the user back to waiting, while silence on a partial offer is terminal —
// it reads as "not interested" (docs/design_context.md, пп. 3–4).
func (s *Service) advance(m *member) {
	switch m.Status {
	case models.StatusQueued:
		if time.Since(m.waitingSince) >= queueDelay {
			s.grantRight(m, m.Requested)
		}
	case models.StatusRightActive:
		if time.Now().After(m.ExpiresAt) {
			delete(s.rights, m.Token)

			m.Token = ""
			m.Quantity = 0
			m.ExpiresAt = time.Time{}
			m.Status = models.StatusQueued
			m.waitingSince = time.Now()
		}
	case models.StatusOfferPending:
		if time.Now().After(m.ExpiresAt) {
			m.AvailableQuantity = 0
			m.ExpiresAt = time.Time{}
			m.Status = models.StatusDeclined
		}
	case models.StatusDeclined, models.StatusPurchased, models.StatusSoldOut:
	}
}

// grantRight and grantOffer assume the caller already holds s.mu.
func (s *Service) grantRight(m *member, quantity int) {
	m.Status = models.StatusRightActive
	m.Quantity = quantity
	m.Token = newToken()
	m.ExpiresAt = time.Now().Add(s.rightTTL)

	s.rights[m.Token] = m
}

func (s *Service) grantOffer(m *member) {
	m.Status = models.StatusOfferPending
	m.AvailableQuantity = offerLimit
	m.ExpiresAt = time.Now().Add(s.offerTTL)
}

func key(productID, userID string) string {
	return productID + "|" + userID
}
