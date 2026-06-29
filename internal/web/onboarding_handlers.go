package web

import (
	"net/http"

	"github.com/paulkinyatti/local-scava/internal/block"
	"github.com/paulkinyatti/local-scava/internal/domain"
)

// --- Step 1: Role ---

func (h *Handlers) handleOnboardingRole(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get current user to pre-select their role if already set.
	user, _ := h.auth.GetUser(r.Context(), userID)

	h.render(w, "onboarding_role", map[string]any{
		"CurrentRole": string(user.Role),
		"RoleOther":   derefStrPtr(user.RoleOther),
		"Step":        1,
	})
}

func (h *Handlers) handleOnboardingRoleSubmit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.serverError(w, err)
		return
	}

	role := domain.UserRole(r.FormValue("role"))
	if !role.Valid() {
		role = domain.RoleOther
	}
	other := r.FormValue("role_other")

	if err := h.auth.SetRole(r.Context(), userID, role, other); err != nil {
		h.serverError(w, err)
		return
	}
	if err := h.onboarding.SetRole(r.Context(), userID); err != nil {
		h.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/onboarding/blocks", http.StatusSeeOther)
}

// --- Step 2: Blocks ---

func (h *Handlers) handleOnboardingBlocks(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, _ := h.auth.GetUser(r.Context(), userID)
	defaults := block.DefaultBlocksForRole(string(user.Role))
	defaultsMap := make(map[string]bool, len(defaults))
	for _, k := range defaults {
		defaultsMap[k] = true
	}

	h.render(w, "onboarding_blocks", map[string]any{
		"Blocks":   block.Registry(),
		"Defaults": defaultsMap,
		"Step":     2,
	})
}

func (h *Handlers) handleOnboardingBlocksSubmit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.serverError(w, err)
		return
	}

	selectedBlocks := r.Form["blocks"]
	if len(selectedBlocks) == 0 {
		// Re-render with error.
		user, _ := h.auth.GetUser(r.Context(), userID)
		defaults := block.DefaultBlocksForRole(string(user.Role))
		defaultsMap := make(map[string]bool, len(defaults))
		for _, k := range defaults {
			defaultsMap[k] = true
		}
		h.render(w, "onboarding_blocks", map[string]any{
			"Blocks":   block.Registry(),
			"Defaults": defaultsMap,
			"Step":     2,
			"Error":    "Select at least one block.",
		})
		return
	}

	if err := h.block.SetBlocks(r.Context(), userID, selectedBlocks); err != nil {
		h.serverError(w, err)
		return
	}

	// Handle platforms if posts block selected.
	platforms := r.Form["platforms"]
	if len(platforms) > 0 {
		_ = h.onboarding.SetPlatforms(r.Context(), userID, platforms)
	}

	if err := h.onboarding.SetBlocks(r.Context(), userID); err != nil {
		h.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/onboarding/confirm", http.StatusSeeOther)
}

// --- Step 3: Confirm ---

func (h *Handlers) handleOnboardingConfirm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, _ := h.auth.GetUser(r.Context(), userID)
	enabledBlocks, _ := h.block.Enabled(r.Context(), userID)
	platforms, _ := h.onboarding.GetPlatforms(r.Context(), userID)

	h.render(w, "onboarding_confirm", map[string]any{
		"User":      user,
		"Blocks":    enabledBlocks,
		"Platforms": platforms,
		"Step":      3,
	})
}

func (h *Handlers) handleOnboardingConfirmSubmit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := h.onboarding.Complete(r.Context(), userID); err != nil {
		h.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func derefStrPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
