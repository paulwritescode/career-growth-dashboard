package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// SprintHealth computes the current health state of a sprint based on elapsed
// time vs deliverable completion. This is always computed on read (no background
// worker) and is the single source of truth for banners, dashboard cards, and
// the chat "how's my sprint?" response.
func (s *Service) SprintHealth(ctx context.Context, sprintID int64) (domain.SprintHealth, error) {
	sp, err := s.store.GetSprint(ctx, sprintID)
	if err != nil {
		return domain.SprintHealth{}, err
	}

	done, total, err := s.store.CountDeliverables(ctx, sprintID)
	if err != nil {
		return domain.SprintHealth{}, err
	}

	health := domain.SprintHealth{
		DoneCount:       done,
		TotalCount:      total,
		IncompleteCount: total - done,
	}

	// If no deliverables, treat as informational.
	if total == 0 {
		health.Severity = domain.SeverityNudge
		health.Message = "No deliverables defined yet. Add at least one to track progress."
		return health, nil
	}

	health.DeliverablesPct = float64(done) / float64(total)

	// If sprint is not active, just report the ratio.
	if sp.Status != domain.SprintActive {
		health.Severity = domain.SeveritySuccess
		health.Message = fmt.Sprintf("%d of %d deliverables done", done, total)
		return health, nil
	}

	// Compute elapsed percentage from starts_on/ends_on (phase-2 columns) or
	// fall back to started_on/ended_on (phase-1 columns).
	startsOn := coalesceDate(sp.StartsOn, sp.StartedOn)
	endsOn := sp.EndsOn

	// If we don't have bounded dates, derive from duration_days.
	if startsOn != nil && endsOn == nil && sp.DurationDays != nil {
		end := parseDate(*startsOn).AddDate(0, 0, *sp.DurationDays)
		endStr := end.Format("2006-01-02")
		endsOn = &endStr
	}

	if startsOn == nil || endsOn == nil {
		// Unbounded sprint (phase-1 month model) — just use deliverable ratio.
		if health.DeliverablesPct >= 0.75 {
			health.Severity = domain.SeveritySuccess
			health.Message = fmt.Sprintf("On track · %d of %d deliverables done", done, total)
		} else if health.DeliverablesPct >= 0.25 {
			health.Severity = domain.SeverityWarning
			health.Message = fmt.Sprintf("%d of %d deliverables complete — keep going.", done, total)
		} else {
			health.Severity = domain.SeverityWarning
			health.Message = fmt.Sprintf("Only %d of %d deliverables done so far.", done, total)
		}
		return health, nil
	}

	// Bounded sprint — compute elapsed time vs deliverable completion.
	start := parseDate(*startsOn)
	end := parseDate(*endsOn)
	today := s.now().In(s.loc)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	totalDuration := end.Sub(start).Hours() / 24
	if totalDuration <= 0 {
		totalDuration = 1
	}
	elapsed := todayDate.Sub(start).Hours() / 24
	health.ElapsedPct = math.Min(elapsed/totalDuration, 1.5)
	health.DaysRemaining = int(math.Max(0, math.Ceil(end.Sub(todayDate).Hours()/24)))

	overdue := todayDate.After(end) && done < total

	switch {
	case overdue:
		health.Severity = domain.SeverityAlarm
		health.Message = fmt.Sprintf("Sprint deadline passed · %d deliverables incomplete. Complete or archive before starting a new sprint.", total-done)
	case health.ElapsedPct >= 0.8 && health.DeliverablesPct < 0.5:
		health.Severity = domain.SeverityAlert
		health.Message = fmt.Sprintf("Sprint ends in %d day(s) · %d deliverables still incomplete.", health.DaysRemaining, total-done)
	case health.ElapsedPct >= 0.5 && health.DeliverablesPct < 0.25:
		health.Severity = domain.SeverityWarning
		health.Message = fmt.Sprintf("Sprint halfway through — only %.0f%% of deliverables complete. Pick up the pace.", health.DeliverablesPct*100)
	default:
		health.Severity = domain.SeveritySuccess
		health.Message = fmt.Sprintf("On track · %d of %d deliverables done · %d days remaining", done, total, health.DaysRemaining)
	}

	return health, nil
}

// NudgeHealth returns a "no active sprint" nudge if appropriate.
func (s *Service) NudgeHealth(ctx context.Context) (domain.SprintHealth, bool) {
	// Check if any sprint ended more than 7 days ago with no current active sprint.
	if _, err := s.store.GetCurrentSprint(ctx); err == nil {
		return domain.SprintHealth{}, false // there's an active sprint, no nudge needed
	}

	// Look for the most recent completed/shipped sprint.
	sprints, err := s.store.ListSprints(ctx)
	if err != nil || len(sprints) == 0 {
		return domain.SprintHealth{
			Severity: domain.SeverityNudge,
			Message:  "No sprints yet. Ready to plan your first?",
		}, true
	}

	// Find the most recent ended sprint.
	for _, sp := range sprints {
		if sp.EndedOn != nil && *sp.EndedOn != "" {
			ended := parseDate(*sp.EndedOn)
			daysSince := int(s.now().Sub(ended).Hours() / 24)
			if daysSince > 7 {
				return domain.SprintHealth{
					Severity: domain.SeverityNudge,
					Message:  fmt.Sprintf("Last sprint ended %d days ago. Ready to plan your next week?", daysSince),
				}, true
			}
			return domain.SprintHealth{}, false
		}
	}

	return domain.SprintHealth{
		Severity: domain.SeverityNudge,
		Message:  "No active sprint. Ready to plan your next week?",
	}, true
}

// Deliverables returns the deliverables for a sprint.
func (s *Service) Deliverables(ctx context.Context, sprintID int64) ([]domain.Deliverable, error) {
	return s.store.ListDeliverables(ctx, sprintID)
}

// AddDeliverable adds a deliverable to a sprint.
func (s *Service) AddDeliverable(ctx context.Context, sprintID int64, text string, src domain.EventSource) (domain.Deliverable, error) {
	text = trim(text)
	if text == "" {
		return domain.Deliverable{}, validationf("deliverable text is required")
	}
	d, err := s.store.CreateDeliverable(ctx, sprintID, text)
	if err != nil {
		return domain.Deliverable{}, err
	}
	s.appendEvent(ctx, "deliverable.added", src, &sprintID, nil, "Added deliverable: "+text, "")
	return d, nil
}

// ToggleDeliverable flips the done state.
func (s *Service) ToggleDeliverable(ctx context.Context, id int64, src domain.EventSource) (domain.Deliverable, error) {
	d, err := s.store.ToggleDeliverable(ctx, id)
	if err != nil {
		return domain.Deliverable{}, err
	}
	s.appendEvent(ctx, "deliverable.toggled", src, &d.SprintID, nil,
		fmt.Sprintf("Deliverable %q → done=%v", d.Text, d.IsDone), "")
	return d, nil
}

// DeleteDeliverable removes a deliverable.
func (s *Service) DeleteDeliverable(ctx context.Context, id int64) error {
	return s.store.DeleteDeliverable(ctx, id)
}

func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func coalesceDate(a, b *string) *string {
	if a != nil && *a != "" {
		return a
	}
	return b
}
