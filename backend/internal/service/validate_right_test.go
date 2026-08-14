package service_test

import (
	"errors"
	"time"

	"backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateRight_Success_CacheHit verifies the happy path where a valid token
// is quickly found in the Redis cache and successfully validated.
func (s *QueueServiceTestSuite) TestValidateRight_Success_CacheHit() {
	validRight := &models.Right{
		Token:     "valid-token",
		UserID:    "user-1",
		Status:    models.RightStatusActive,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	s.mockCache.EXPECT().GetRight(s.ctx, "valid-token").Return(validRight, nil)

	right, err := s.srv.ValidateRight(s.ctx, "valid-token", "user-1")

	require.NoError(s.T(), err)
	assert.Equal(s.T(), validRight, right)
}

// TestValidateRight_Success_CacheMiss_DBHit verifies the fallback mechanism
// where a cache miss forces a successful lookup in the durable PostgreSQL database.
func (s *QueueServiceTestSuite) TestValidateRight_Success_CacheMiss_DBHit() {
	validRight := &models.Right{
		Token:     "valid-token",
		UserID:    "user-1",
		Status:    models.RightStatusActive,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	s.mockCache.EXPECT().GetRight(s.ctx, "valid-token").Return(nil, models.ErrTokenNotFound)
	s.mockDurable.EXPECT().GetRightByToken(s.ctx, "valid-token").Return(validRight, nil)
	s.mockCache.EXPECT().SetRight(s.ctx, validRight).Return(nil)

	right, err := s.srv.ValidateRight(s.ctx, "valid-token", "user-1")

	require.NoError(s.T(), err)
	assert.Equal(s.T(), validRight, right)
}

// TestValidateRight_NotFound verifies that a token missing from both the cache
// and the database returns a not found error, preventing progression.
func (s *QueueServiceTestSuite) TestValidateRight_NotFound() {
	s.mockCache.EXPECT().GetRight(s.ctx, "invalid-token").Return(nil, models.ErrTokenNotFound)
	s.mockDurable.EXPECT().GetRightByToken(s.ctx, "invalid-token").Return(nil, models.ErrTokenNotFound)

	right, err := s.srv.ValidateRight(s.ctx, "invalid-token", "user-1")

	require.ErrorIs(s.T(), err, models.ErrTokenNotFound)
	assert.Nil(s.T(), right)
}

// TestValidateRight_Forbidden verifies access control, ensuring a user cannot
// validate and use a token that was issued to a different user ID.
func (s *QueueServiceTestSuite) TestValidateRight_Forbidden() {
	stolenRight := &models.Right{
		Token:     "stolen-token",
		UserID:    "victim-user",
		Status:    models.RightStatusActive,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	s.mockCache.EXPECT().GetRight(s.ctx, "stolen-token").Return(stolenRight, nil)

	right, err := s.srv.ValidateRight(s.ctx, "stolen-token", "thief-user")

	require.ErrorIs(s.T(), err, models.ErrForbidden)
	assert.Nil(s.T(), right)
}

// TestValidateRight_AlreadyUsed verifies that a token which has already successfully
// passed the payment process cannot be validated again.
func (s *QueueServiceTestSuite) TestValidateRight_AlreadyUsed() {
	usedRight := &models.Right{
		Token:     "used-token",
		UserID:    "user-1",
		Status:    models.RightStatusUsed,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	s.mockCache.EXPECT().GetRight(s.ctx, "used-token").Return(usedRight, nil)

	right, err := s.srv.ValidateRight(s.ctx, "used-token", "user-1")

	require.ErrorIs(s.T(), err, models.ErrTokenUsed)
	assert.Nil(s.T(), right)
}

// TestValidateRight_Expired verifies that a token whose TTL has passed is strictly
// rejected, even if the background expiration worker hasn't processed it yet.
func (s *QueueServiceTestSuite) TestValidateRight_Expired() {
	expiredRight := &models.Right{
		Token:     "expired-token",
		UserID:    "user-1",
		Status:    models.RightStatusActive,
		ExpiresAt: time.Now().UTC().Add(-1 * time.Minute),
	}

	s.mockCache.EXPECT().GetRight(s.ctx, "expired-token").Return(expiredRight, nil)

	right, err := s.srv.ValidateRight(s.ctx, "expired-token", "user-1")

	require.ErrorIs(s.T(), err, models.ErrTokenExpired)
	assert.Nil(s.T(), right)
}

// TestValidateRight_InvalidStatus verifies that tokens with corrupted or unexpected
// statuses are rejected by the state machine validations.
func (s *QueueServiceTestSuite) TestValidateRight_InvalidStatus() {
	invalidRight := &models.Right{
		Token:     "invalid-status-token",
		UserID:    "user-1",
		Status:    models.RightStatus("CORRUPTED_STATUS"),
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	s.mockCache.EXPECT().GetRight(s.ctx, "invalid-status-token").Return(invalidRight, nil)

	right, err := s.srv.ValidateRight(s.ctx, "invalid-status-token", "user-1")

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
	assert.Nil(s.T(), right)
}

// TestValidateRight_CacheError verifies that a critical infrastructure error in Redis
// immediately aborts the validation process and bubbles up to the caller.
func (s *QueueServiceTestSuite) TestValidateRight_CacheError() {
	expectedErr := errors.New("redis connection refused")
	s.mockCache.EXPECT().GetRight(s.ctx, "error-token").Return(nil, expectedErr)

	right, err := s.srv.ValidateRight(s.ctx, "error-token", "user-1")

	require.ErrorIs(s.T(), err, expectedErr)
	assert.Nil(s.T(), right)
}

// TestValidateRight_DBError verifies that if the cache misses and the database
// experiences an infrastructure error, the error is properly propagated.
func (s *QueueServiceTestSuite) TestValidateRight_DBError() {
	expectedErr := errors.New("postgres connection lost")
	s.mockCache.EXPECT().GetRight(s.ctx, "db-error-token").Return(nil, models.ErrTokenNotFound)
	s.mockDurable.EXPECT().GetRightByToken(s.ctx, "db-error-token").Return(nil, expectedErr)

	right, err := s.srv.ValidateRight(s.ctx, "db-error-token", "user-1")

	require.ErrorIs(s.T(), err, expectedErr)
	assert.Nil(s.T(), right)
}
func (s *QueueServiceTestSuite) TestValidateRight_ExpiredStatus() {
	expiredRight := &models.Right{
		Token:     "expired-status-token",
		UserID:    "user-1",
		Status:    models.RightStatusExpired,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	s.mockCache.EXPECT().GetRight(s.ctx, "expired-status-token").Return(expiredRight, nil)

	right, err := s.srv.ValidateRight(s.ctx, "expired-status-token", "user-1")

	require.ErrorIs(s.T(), err, models.ErrTokenExpired)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestValidateRightForCheckout_Success() {
	validRight := &models.Right{
		Token:     "valid-token",
		ProductID: "prod-1",
		Status:    models.RightStatusActive,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	s.mockCache.EXPECT().GetRight(s.ctx, "valid-token").Return(validRight, nil)

	right, err := s.srv.ValidateRightForCheckout(s.ctx, "valid-token", "prod-1")

	require.NoError(s.T(), err)
	assert.Equal(s.T(), validRight, right)
}

func (s *QueueServiceTestSuite) TestValidateRightForCheckout_ProductMismatch() {
	validRight := &models.Right{
		Token:     "valid-token",
		ProductID: "prod-1",
		Status:    models.RightStatusActive,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	s.mockCache.EXPECT().GetRight(s.ctx, "valid-token").Return(validRight, nil)

	right, err := s.srv.ValidateRightForCheckout(s.ctx, "valid-token", "prod-2")

	require.ErrorIs(s.T(), err, models.ErrForbidden)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestValidateRightForCheckout_CacheMissDBHit() {
	validRight := &models.Right{
		Token:     "valid-token",
		ProductID: "prod-1",
		Status:    models.RightStatusActive,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	s.mockCache.EXPECT().GetRight(s.ctx, "valid-token").Return(nil, models.ErrTokenNotFound)
	s.mockDurable.EXPECT().GetRightByToken(s.ctx, "valid-token").Return(validRight, nil)
	s.mockCache.EXPECT().SetRight(s.ctx, validRight).Return(nil)

	right, err := s.srv.ValidateRightForCheckout(s.ctx, "valid-token", "prod-1")

	require.NoError(s.T(), err)
	assert.Equal(s.T(), validRight, right)
}
