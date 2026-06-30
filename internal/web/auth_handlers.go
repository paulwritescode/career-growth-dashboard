package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/paulkinyatti/local-scava/internal/auth"
)

// --- Setup ---

func (h *Handlers) handleSetupForm(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := h.auth.HasAdmin(r.Context())
	if err != nil {
		h.serverError(w, err)
		return
	}
	if hasAdmin {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	h.render(w, "setup", map[string]any{"Error": ""})
}

func (h *Handlers) handleSetupSubmit(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := h.auth.HasAdmin(r.Context())
	if err != nil {
		h.serverError(w, err)
		return
	}
	if hasAdmin {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.serverError(w, err)
		return
	}

	user, err := h.auth.SetupAdmin(r.Context(), auth.SetupInput{
		Password:    r.FormValue("password"),
		Confirm:     r.FormValue("confirm"),
		DisplayName: r.FormValue("display_name"),
	})
	if err != nil {
		errorMsg := "Setup failed."
		if errors.Is(err, auth.ErrWeakPassword) {
			errorMsg = "Password must be at least 8 characters."
		} else if errors.Is(err, auth.ErrPasswordMismatch) {
			errorMsg = "Passwords do not match."
		}
		h.render(w, "setup", map[string]any{"Error": errorMsg})
		return
	}

	// Auto-login: create session and set cookie.
	sess, err := h.auth.Authenticate(r.Context(), "admin", r.FormValue("password"), false)
	if err != nil {
		h.serverError(w, err)
		return
	}
	setSessionCookie(w, sess.ID, false)

	_ = user // silence unused
	http.Redirect(w, r, "/onboarding/role", http.StatusSeeOther)
}

// --- Login ---

func (h *Handlers) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := h.auth.HasAdmin(r.Context())
	if err != nil {
		h.serverError(w, err)
		return
	}
	if !hasAdmin {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	h.render(w, "login", map[string]any{"Error": "", "Next": r.URL.Query().Get("next")})
}

func (h *Handlers) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.serverError(w, err)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	if username == "" {
		username = "admin"
	}
	password := r.FormValue("password")
	remember := r.FormValue("remember") == "on"

	sess, err := h.auth.Authenticate(r.Context(), username, password, remember)
	if err != nil {
		h.render(w, "login", map[string]any{
			"Error": "Incorrect username or password.",
			"Next":  r.FormValue("next"),
		})
		return
	}

	setSessionCookie(w, sess.ID, remember)

	// Check onboarding status.
	complete, _ := h.onboarding.IsComplete(r.Context(), sess.UserID)
	if !complete {
		step, _ := h.onboarding.CurrentStep(r.Context(), sess.UserID)
		http.Redirect(w, r, step, http.StatusSeeOther)
		return
	}

	next := r.FormValue("next")
	if next == "" || next == "/login" || next == "/setup" {
		next = "/"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// --- Logout ---

func (h *Handlers) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("scava_session")
	if err == nil && cookie.Value != "" {
		_ = h.auth.Logout(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "scava_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// --- Profile & Password (Settings) ---

func (h *Handlers) handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.serverError(w, err)
		return
	}

	_, err := h.auth.UpdateProfile(r.Context(), auth.UpdateProfileInput{
		UserID:         userID,
		DisplayName:    r.FormValue("display_name"),
		Username:       r.FormValue("username"),
		AvatarInitials: r.FormValue("avatar_initials"),
	})
	if err != nil {
		h.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/settings?msg=profile_updated", http.StatusSeeOther)
}

func (h *Handlers) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	sessID, _ := h.currentSessionID(r)
	if err := r.ParseForm(); err != nil {
		h.serverError(w, err)
		return
	}

	err := h.auth.ChangePassword(r.Context(), auth.ChangePasswordInput{
		CurrentPassword: r.FormValue("current_password"),
		NewPassword:     r.FormValue("new_password"),
		Confirm:         r.FormValue("confirm_password"),
		UserID:          userID,
		SessionID:       sessID,
	})
	if err != nil {
		http.Redirect(w, r, "/settings?err=password_change_failed", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings?msg=password_changed", http.StatusSeeOther)
}

// --- Block toggle ---

func (h *Handlers) handleBlockToggle(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	key := r.PathValue("key")
	_, err := h.block.Toggle(r.Context(), userID, key)
	if err != nil {
		h.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// --- Helpers ---

// requireAuth wraps a handler and redirects unauthenticated requests to /login.
// The original URL is preserved in the ?next= query param so the login handler
// can send the user back after a successful sign-in.
func (h *Handlers) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := h.currentUserID(r)
		if !ok {
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (h *Handlers) currentUserID(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie("scava_session")
	if err != nil || cookie.Value == "" {
		return 0, false
	}
	sess, err := h.auth.ValidateSession(r.Context(), cookie.Value)
	if err != nil {
		return 0, false
	}
	return sess.UserID, true
}

func (h *Handlers) currentSessionID(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("scava_session")
	if err != nil || cookie.Value == "" {
		return "", false
	}
	return cookie.Value, true
}

func setSessionCookie(w http.ResponseWriter, sessionID string, remember bool) {
	cookie := &http.Cookie{
		Name:     "scava_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if remember {
		cookie.MaxAge = 14 * 24 * 60 * 60 // 14 days
	}
	http.SetCookie(w, cookie)
}
