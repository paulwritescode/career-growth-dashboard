// Package service holds the business rules for local-scava. Both the web forms
// and the kiro-cli chat bridge call these methods, so a value set by chat is
// identical to one set by a form. Services validate input, enforce the
// career-growth skill rules, persist via the store, and append career events.
//
// Layer rule: service imports domain and store. It is imported by web and
// bridge. It never imports web.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
	"github.com/paulkinyatti/local-scava/internal/store"
)

// Validation/business-rule errors. Handlers map these to user-facing messages.
var (
	ErrValidation         = errors.New("validation error")
	ErrActiveSprintExists = errors.New("an active sprint already exists; ship or abandon it first")
	ErrLiveURLRequired    = errors.New("a live URL is required before a sprint can be marked shipped")
	ErrURLRequired        = errors.New("a URL is required to mark a tier as published")
	ErrNotFound           = store.ErrNotFound
)

// validationf builds a wrapped validation error with a formatted message.
func validationf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}

// Service is the single entry point to all business operations.
type Service struct {
	store *store.Store
	now   func() time.Time // injectable clock for tests
	loc   *time.Location   // local timezone for "today" calculations
}

// New constructs a Service backed by the given store. The local timezone is
// used to decide what "today" means for cadence tracking.
func New(st *store.Store) *Service {
	return &Service{
		store: st,
		now:   func() time.Time { return time.Now() },
		loc:   time.Local,
	}
}

// Today returns the current local calendar date as YYYY-MM-DD.
func (s *Service) Today() string {
	return s.now().In(s.loc).Format("2006-01-02")
}

// appendEvent records a career event; failures are non-fatal to the mutation
// (the audit trail is best-effort), so callers log but do not abort on error.
func (s *Service) appendEvent(ctx context.Context, kind string, src domain.EventSource, sprintID, postID *int64, summary, detail string) {
	if src == "" {
		src = domain.SourceForm
	}
	_ = s.store.AppendEvent(ctx, domain.CareerEvent{
		OccurredAt: s.now().UTC(),
		Kind:       kind,
		Source:     src,
		SprintID:   sprintID,
		PostID:     postID,
		Summary:    summary,
		Detail:     detail,
	})
}

func trim(s string) string { return strings.TrimSpace(s) }

// defaultChecklist returns the phase-gate template seeded for every new sprint,
// taken from weekly-checklist.md in the monthly-skill-sprint skill.
func defaultChecklist() []domain.ChecklistItem {
	type gate struct {
		phase domain.Phase
		label string
	}
	gates := []gate{
		{domain.PhaseScopeDeclare, "Skill picked, and I can explain in one sentence why it's worth a month"},
		{domain.PhaseScopeDeclare, "Microapp scoped to something buildable in ~2 focused weeks"},
		{domain.PhaseScopeDeclare, "One core feature identified as 'the thing that proves it'"},
		{domain.PhaseScopeDeclare, "One thing explicitly marked out of scope"},
		{domain.PhaseScopeDeclare, "Declaration post published"},

		{domain.PhaseBuild, "Core feature works locally, even if rough"},
		{domain.PhaseBuild, "No single blocker has eaten more than 2 days without a decision"},
		{domain.PhaseBuild, "A one-line log exists for most days so far"},

		{domain.PhasePolishDocument, "Deployed at least once already (not saving it for the last week)"},
		{domain.PhasePolishDocument, "README covers what/why/architecture/how-to-run + a screenshot or GIF"},
		{domain.PhasePolishDocument, "Code has consistent naming and passes a basic lint"},
		{domain.PhasePolishDocument, "Core logic has at least minimal tests"},

		{domain.PhaseDeployShowcase, "Live URL works end-to-end for a first-time visitor"},
		{domain.PhaseDeployShowcase, "Demo video/GIF recorded"},
		{domain.PhaseDeployShowcase, "Portfolio updated"},
		{domain.PhaseDeployShowcase, "Resume/LinkedIn skills section updated"},
		{domain.PhaseDeployShowcase, "Recap posted across blog / LinkedIn / X"},
		{domain.PhaseDeployShowcase, "Considered a recruiter/hiring-manager proof-of-work message"},
	}
	items := make([]domain.ChecklistItem, 0, len(gates))
	order := map[domain.Phase]int{}
	for _, g := range gates {
		items = append(items, domain.ChecklistItem{Phase: g.phase, Label: g.label, SortOrder: order[g.phase]})
		order[g.phase]++
	}
	return items
}
