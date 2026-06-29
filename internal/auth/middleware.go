package auth

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is a private type to avoid collisions in context values.
type contextKey int

const (
	ctxUserID    contextKey = iota
	ctxSessionID contextKey = 2
)

// UserIDFromContext returns the authenticated user ID from the request context.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(ctxUserID).(int64)
	return v, ok
}

// SessionIDFromContext returns the session ID from the request context.
func SessionIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxSessionID).(string)
	return v, ok
}

// RequireSession is middleware that enforces a valid session cookie.
// Unauthenticated requests are redirected to /login.
func (s *Service) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("scava_session")
		if err != nil || cookie.Value == "" {
			redirectToLogin(w, r)
			return
		}

		sess, err := s.ValidateSession(r.Context(), cookie.Value)
		if err != nil {
			// Clear stale cookie.
			http.SetCookie(w, &http.Cookie{
				Name:     "scava_session",
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			redirectToLogin(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, sess.UserID)
		ctx = context.WithValue(ctx, ctxSessionID, sess.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireSetup is middleware that redirects to /setup when no users exist,
// and away from /setup once a user does exist.
func (s *Service) RequireSetup(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasAdmin, err := s.HasAdmin(r.Context())
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		path := r.URL.Path
		isSetupPath := path == "/setup"
		isStaticPath := strings.HasPrefix(path, "/static/")
		isHealthz := path == "/healthz" || path == "/api/v1/healthz"

		if !hasAdmin {
			// No user yet — force to /setup.
			if !isSetupPath && !isStaticPath && !isHealthz {
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			}
		} else {
			// User exists — /setup is no longer reachable.
			if isSetupPath {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// RequireAPIKey is middleware for REST API endpoints. It validates the
// X-Scava-Key header instead of a session cookie.
func (s *Service) RequireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Scava-Key")
		if key == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		apiKey, err := s.AuthenticateKey(r.Context(), key)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, apiKey.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	target := "/login"
	if r.URL.Path != "/" && r.URL.Path != "/login" {
		target = "/login?next=" + r.URL.Path
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Simple JSON encoding for error responses.
	if m, ok := data.(map[string]string); ok {
		w.Write([]byte(`{`))
		first := true
		for k, v := range m {
			if !first {
				w.Write([]byte(`,`))
			}
			w.Write([]byte(`"` + k + `":"` + v + `"`))
			first = false
		}
		w.Write([]byte(`}`))
	}
}
