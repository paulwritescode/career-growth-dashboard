package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/paulkinyatti/local-scava/internal/domain"
	"github.com/paulkinyatti/local-scava/internal/service"
)

// redirect sends the browser to url, honoring htmx (HX-Redirect) when present.
func (h *Handlers) redirect(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// userError reports a validation/business-rule error to the client (400).
func (h *Handlers) userError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	if errors.Is(err, service.ErrNotFound) {
		code = http.StatusNotFound
	}
	http.Error(w, err.Error(), code)
}

func formStr(r *http.Request, key string) string { return r.FormValue(key) }

func formInt64Ptr(r *http.Request, key string) *int64 {
	v := r.FormValue(key)
	if v == "" || v == "0" {
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// --- sprints --------------------------------------------------------------

func (h *Handlers) handleSprintCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.userError(w, err)
		return
	}
	status := domain.SprintStatus(formStr(r, "status"))
	sp, err := h.svc.CreateSprint(r.Context(), service.CreateSprintInput{
		MonthLabel:       formStr(r, "month_label"),
		SkillName:        formStr(r, "skill_name"),
		SkillRationale:   formStr(r, "skill_rationale"),
		MicroappOneLiner: formStr(r, "microapp_one_liner"),
		CoreFeature:      formStr(r, "core_feature"),
		OutOfScope:       formStr(r, "out_of_scope"),
		DeployPlatform:   formStr(r, "deploy_platform"),
		Status:           status,
		Source:           domain.SourceForm,
	})
	if err != nil {
		h.userError(w, err)
		return
	}
	h.redirect(w, r, "/sprints/"+strconv.FormatInt(sp.ID, 10))
}

func (h *Handlers) handleSprintPhase(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.userError(w, err)
		return
	}
	phase, _ := strconv.Atoi(formStr(r, "phase"))
	if _, err := h.svc.SetPhase(r.Context(), id, domain.Phase(phase), domain.SourceForm); err != nil {
		h.userError(w, err)
		return
	}
	h.redirect(w, r, "/sprints/"+strconv.FormatInt(id, 10))
}

func (h *Handlers) handleSprintStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.userError(w, err)
		return
	}
	if _, err := h.svc.SetStatus(r.Context(), id, service.SetStatusInput{
		Status:  domain.SprintStatus(formStr(r, "status")),
		LiveURL: formStr(r, "live_url"),
		Source:  domain.SourceForm,
	}); err != nil {
		h.userError(w, err)
		return
	}
	h.redirect(w, r, "/sprints/"+strconv.FormatInt(id, 10))
}

func (h *Handlers) handleSprintRetro(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.userError(w, err)
		return
	}
	if _, err := h.svc.RecordRetro(r.Context(), id,
		formStr(r, "retro_worked"), formStr(r, "retro_differently"),
		formStr(r, "retro_learned"), formStr(r, "retro_live_link"), domain.SourceForm); err != nil {
		h.userError(w, err)
		return
	}
	h.redirect(w, r, "/sprints/"+strconv.FormatInt(id, 10))
}

func (h *Handlers) handleChecklistToggle(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.userError(w, err)
		return
	}
	done := formStr(r, "done") == "true" || formStr(r, "done") == "on" || formStr(r, "done") == "1"
	if err := h.svc.ToggleChecklistItem(r.Context(), id, done); err != nil {
		h.userError(w, err)
		return
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/sprints"
	}
	h.redirect(w, r, ref)
}

// --- logs -----------------------------------------------------------------

func (h *Handlers) handleLogCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.userError(w, err)
		return
	}
	in := service.RecordLogInput{
		SprintID:     formInt64Ptr(r, "sprint_id"),
		LogDate:      formStr(r, "log_date"),
		WorkedOn:     formStr(r, "worked_on"),
		WhatHappened: formStr(r, "what_happened"),
		Insight:      formStr(r, "insight"),
		NextUp:       formStr(r, "next_up"),
		Blocker:      formStr(r, "blocker"),
		Source:       domain.SourceForm,
	}
	if bd := formStr(r, "blocker_decision"); bd != "" {
		d := domain.BlockerDecision(bd)
		in.BlockerDecision = &d
	}
	if _, err := h.svc.RecordLog(r.Context(), in); err != nil {
		h.userError(w, err)
		return
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	h.redirect(w, r, ref)
}

// --- posts ----------------------------------------------------------------

func (h *Handlers) handlePostCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.userError(w, err)
		return
	}
	post, err := h.svc.CreatePost(r.Context(), service.CreatePostInput{
		PostDate:      formStr(r, "post_date"),
		PostType:      domain.PostType(formStr(r, "post_type")),
		SprintID:      formInt64Ptr(r, "sprint_id"),
		SourceLogID:   formInt64Ptr(r, "source_log_id"),
		Title:         formStr(r, "title"),
		IsDeclaration: formStr(r, "is_declaration") != "",
		Source:        domain.SourceForm,
	})
	if err != nil {
		h.userError(w, err)
		return
	}
	h.redirect(w, r, "/posts/"+strconv.FormatInt(post.ID, 10))
}

// handleTierUpdate handles draft, publish, and visual changes for a tier based
// on the "action" field.
func (h *Handlers) handleTierUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.userError(w, err)
		return
	}
	ctx := r.Context()
	tier := domain.Tier(formStr(r, "tier"))
	switch formStr(r, "action") {
	case "publish":
		_, err = h.svc.MarkPublished(ctx, id, tier, formStr(r, "url"), domain.SourceForm)
	case "draft":
		_, err = h.svc.DraftTier(ctx, id, tier, formStr(r, "content"), domain.SourceForm)
	case "visual":
		_, err = h.svc.SetTierVisual(ctx, id, tier, domain.VisualKind(formStr(r, "visual_kind")),
			formInt64Ptr(r, "adr_id"), domain.SourceForm)
	default:
		err = errors.New("unknown tier action")
	}
	if err != nil {
		h.userError(w, err)
		return
	}
	h.redirect(w, r, "/posts/"+strconv.FormatInt(id, 10))
}

// --- ADRs -----------------------------------------------------------------

func (h *Handlers) handleADRCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.userError(w, err)
		return
	}
	num, _ := strconv.Atoi(formStr(r, "number"))
	_, err := h.svc.CreateADR(r.Context(), service.CreateADRInput{
		Number:       num,
		Title:        formStr(r, "title"),
		Status:       domain.ADRStatus(formStr(r, "status")),
		DecidedOn:    formStr(r, "decided_on"),
		Problem:      formStr(r, "problem"),
		Options:      formStr(r, "options"),
		Decision:     formStr(r, "decision"),
		Why:          formStr(r, "why"),
		Consequences: formStr(r, "consequences"),
		SprintID:     formInt64Ptr(r, "sprint_id"),
		Source:       domain.SourceForm,
	})
	if err != nil {
		h.userError(w, err)
		return
	}
	h.redirect(w, r, "/adrs")
}
