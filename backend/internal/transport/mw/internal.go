package mw

import "net/http"

// InternalTokenHeader authorises calls coming from AvitoBackend rather than from
// a browser. Without it POST /rights/{token}/events is open to anyone, and a
// stranger could mark someone else's right as paid.
const InternalTokenHeader = "X-Internal-Token" //nolint:gosec // header name, not a credential

// InternalAuth rejects service-to-service calls that do not carry the shared
// secret. An empty token disables the check — convenient locally, and the reason
// the value is required in every deployed environment.
func InternalAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" && r.Header.Get(InternalTokenHeader) != token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
