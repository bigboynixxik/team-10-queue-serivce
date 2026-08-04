package mw

import (
	"context"
	"net/http"
)

// UserHeader carries the caller identity. Real authentication is out of the case
// scope (requirements.md, FR-D1): the frontend just sends whichever user it acts
// as, which is also what makes two-buyer races testable by hand.
const UserHeader = "X-User-Id"

type userContextKey struct{}

// UserMiddleware pulls the user id out of the request header and puts it into the
// context. Requests without the header are rejected with 401 — handlers may then
// assume a caller is always present.
func UserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get(UserHeader)
		if userID == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), userID)))
	})
}

// WithUser returns a context carrying the given user id.
func WithUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userContextKey{}, userID)
}

// UserFromContext retrieves the user id put into the context by UserMiddleware.
func UserFromContext(ctx context.Context) string {
	userID, _ := ctx.Value(userContextKey{}).(string)
	return userID
}
