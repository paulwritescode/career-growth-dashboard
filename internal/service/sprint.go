package service

import (
	"context"
	"fmt"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// CreateSprintInput is the data needed to start a sprint (Phase-1 scope fields).
type CreateSprintInput struct {
	MonthLabel       string
	SkillName        string
	SkillRationale   string
	MicroappOneLiner string
	CoreFeature      string
	OutOfScope       string
	DeployPlatform   string
	Status           domain.SprintStatus // defaults to active
	Source           domain.EventSource
}

// CreateSprint validates and creates a sprint, seeds its phase checklist, and
// records a career event. Enforces: one active sprint at a time, and the
// required scope fields from the monthly-skill-sprint skill.
func (s *Service) CreateSprint(ctx context.Context, in CreateSprintInput) (domain.Sprint, error) {
	in.SkillName = trim(in.SkillName)
	in.MicroappOneLiner = trim(in.MicroappOneLiner)
	in.CoreFeature = trim(in.CoreFeature)
	in.MonthLabel = trim(in.MonthLabel)

	if in.SkillName == "" {
		return domain.Sprint{}, validationf("skill name is required")
	}
	if in.MicroappOneLiner == "" {
		return domain.Sprint{}, validationf("microapp one-liner is required")
	}
	if in.CoreFeature == "" {
		return domain.Sprint{}, validationf("core feature is required")
	}
	if in.MonthLabel == "" {
		in.MonthLabel = s.now().In(s.loc).Format("2006-01")
	}
	if in.Status == "" {
		in.Status = domain.SprintActive
	}
	if !in.Status.Valid() {
		return domain.Sprint{}, validationf("invalid status %q", in.Status)
	}

	// Enforce one active sprint (friendly error before the DB unique index fires).
	if in.Status == domain.SprintActive {
		if _, err := s.store.GetCurrentSprint(ctx); err == nil {
			return domain.Sprint{}, ErrActiveSprintExists
		} else if err != ErrNotFound {
			return domain.Sprint{}, err
		}
	}

	today := s.Today()
	sp := domain.Sprint{
		MonthLabel:       in.MonthLabel,
		SkillName:        in.SkillName,
		SkillRationale:   trim(in.SkillRationale),
		MicroappOneLiner: in.MicroappOneLiner,
		CoreFeature:      in.CoreFeature,
		OutOfScope:       trim(in.OutOfScope),
		DeployPlatform:   trim(in.DeployPlatform),
		CurrentPhase:     domain.PhaseScopeDeclare,
		Status:           in.Status,
		StartedOn:        &today,
	}
	created, err := s.store.CreateSprint(ctx, sp)
	if err != nil {
		return domain.Sprint{}, err
	}
	if err := s.store.SeedChecklist(ctx, created.ID, defaultChecklist()); err != nil {
		return domain.Sprint{}, err
	}
	s.appendEvent(ctx, "sprint.created", in.Source, &created.ID, nil,
		"Started sprint: "+created.SkillName+" — "+created.MicroappOneLiner, "")
	return created, nil
}

// CurrentSprint returns the active sprint, or ErrNotFound if none.
func (s *Service) CurrentSprint(ctx context.Context) (domain.Sprint, error) {
	return s.store.GetCurrentSprint(ctx)
}

// GetSprint returns a sprint by ID.
func (s *Service) GetSprint(ctx context.Context, id int64) (domain.Sprint, error) {
	return s.store.GetSprint(ctx, id)
}

// ListSprints returns all sprints, newest first.
func (s *Service) ListSprints(ctx context.Context) ([]domain.Sprint, error) {
	return s.store.ListSprints(ctx)
}

// SetPhase moves a sprint to a new phase. Phase is set manually (sprints drift;
// it is never derived from the calendar). Records a career event.
func (s *Service) SetPhase(ctx context.Context, id int64, phase domain.Phase, src domain.EventSource) (domain.Sprint, error) {
	if !phase.Valid() {
		return domain.Sprint{}, validationf("invalid phase %d", phase)
	}
	sp, err := s.store.GetSprint(ctx, id)
	if err != nil {
		return domain.Sprint{}, err
	}
	old := sp.CurrentPhase
	sp.CurrentPhase = phase
	if err := s.store.UpdateSprint(ctx, sp); err != nil {
		return domain.Sprint{}, err
	}
	s.appendEvent(ctx, "sprint.phase_changed", src, &sp.ID, nil,
		sp.SkillName+": phase "+old.Label()+" → "+phase.Label(),
		fmt.Sprintf(`{"from":%d,"to":%d}`, old, phase))
	return s.store.GetSprint(ctx, id)
}

// SetStatusInput carries a status change and any required accompanying data.
type SetStatusInput struct {
	Status  domain.SprintStatus
	LiveURL string // required when transitioning to shipped (if not already set)
	Source  domain.EventSource
}

// SetStatus changes a sprint's lifecycle status. Enforces the skill's core
// rule: a sprint cannot be marked shipped without a live URL ("a live link
// beats a finished-looking codebase"). Shipping stamps ended_on.
func (s *Service) SetStatus(ctx context.Context, id int64, in SetStatusInput) (domain.Sprint, error) {
	if !in.Status.Valid() {
		return domain.Sprint{}, validationf("invalid status %q", in.Status)
	}
	sp, err := s.store.GetSprint(ctx, id)
	if err != nil {
		return domain.Sprint{}, err
	}

	if url := trim(in.LiveURL); url != "" {
		sp.LiveURL = &url
	}

	if in.Status == domain.SprintShipped {
		if !sp.HasLiveURL() {
			return domain.Sprint{}, ErrLiveURLRequired
		}
		ended := s.Today()
		sp.EndedOn = &ended
	}
	if in.Status == domain.SprintActive {
		// Guard against two active sprints.
		if cur, err := s.store.GetCurrentSprint(ctx); err == nil && cur.ID != sp.ID {
			return domain.Sprint{}, ErrActiveSprintExists
		} else if err != nil && err != ErrNotFound {
			return domain.Sprint{}, err
		}
	}

	sp.Status = in.Status
	if err := s.store.UpdateSprint(ctx, sp); err != nil {
		return domain.Sprint{}, err
	}
	kind := "sprint.status_changed"
	summary := sp.SkillName + ": status → " + string(in.Status)
	if in.Status == domain.SprintShipped {
		kind = "sprint.shipped"
		summary = "Shipped " + sp.SkillName + " → " + *sp.LiveURL
	}
	s.appendEvent(ctx, kind, in.Source, &sp.ID, nil, summary, "")
	return s.store.GetSprint(ctx, id)
}

// RecordRetro saves the post-ship retro fields.
func (s *Service) RecordRetro(ctx context.Context, id int64, worked, differently, learned, liveLink string, src domain.EventSource) (domain.Sprint, error) {
	sp, err := s.store.GetSprint(ctx, id)
	if err != nil {
		return domain.Sprint{}, err
	}
	sp.RetroWorked = trim(worked)
	sp.RetroDifferently = trim(differently)
	sp.RetroLearned = trim(learned)
	sp.RetroLiveLink = trim(liveLink)
	if err := s.store.UpdateSprint(ctx, sp); err != nil {
		return domain.Sprint{}, err
	}
	s.appendEvent(ctx, "sprint.retro", src, &sp.ID, nil, sp.SkillName+": retro recorded", "")
	return s.store.GetSprint(ctx, id)
}

// Checklist returns the phase-gate checklist for a sprint.
func (s *Service) Checklist(ctx context.Context, sprintID int64) ([]domain.ChecklistItem, error) {
	return s.store.ListChecklist(ctx, sprintID)
}

// ToggleChecklistItem flips a checklist gate's done state.
func (s *Service) ToggleChecklistItem(ctx context.Context, id int64, done bool) error {
	return s.store.ToggleChecklistItem(ctx, id, done)
}

// DeleteSprint removes a sprint and its checklist. Records a career event.
func (s *Service) DeleteSprint(ctx context.Context, id int64, src domain.EventSource) error {
	sp, err := s.store.GetSprint(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteSprint(ctx, id); err != nil {
		return err
	}
	s.appendEvent(ctx, "sprint.deleted", src, &id, nil, "Deleted sprint: "+sp.SkillName, "")
	return nil
}
