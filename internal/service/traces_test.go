package service

import (
	"context"
	"testing"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// TestSprintTracePhaseSpans verifies the sprint-as-trace reconstruction: phase
// boundaries come from sprint.created + phase_changed events, widths sum to
// ~100%, and the last span of an active sprint is the open/current one.
func TestSprintTracePhaseSpans(t *testing.T) {
	ctx := context.Background()
	// Use a movable clock so phase changes land on different days.
	clock := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC) // Monday
	svc := newTestService(t, clock)
	svc.now = func() time.Time { return clock }

	sp := mustSprint(t, svc)

	// Day 3: move to Build.
	clock = time.Date(2026, 6, 25, 9, 0, 0, 0, time.UTC)
	if _, err := svc.SetPhase(ctx, sp.ID, domain.PhaseBuild, domain.SourceForm); err != nil {
		t.Fatalf("SetPhase build: %v", err)
	}
	// Day 8: move to Polish.
	clock = time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)
	if _, err := svc.SetPhase(ctx, sp.ID, domain.PhasePolishDocument, domain.SourceForm); err != nil {
		t.Fatalf("SetPhase polish: %v", err)
	}
	// "Now" a couple days later, still active.
	clock = time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)

	spans, err := svc.SprintTrace(ctx, sp.ID)
	if err != nil {
		t.Fatalf("SprintTrace: %v", err)
	}
	if len(spans) != 3 {
		t.Fatalf("expected 3 phase spans, got %d", len(spans))
	}
	if spans[0].Phase != domain.PhaseScopeDeclare || spans[1].Phase != domain.PhaseBuild || spans[2].Phase != domain.PhasePolishDocument {
		t.Fatalf("unexpected phase ordering: %+v", spans)
	}
	if !spans[2].IsCurrent {
		t.Fatalf("expected last span to be the current/open phase")
	}
	if spans[0].IsCurrent {
		t.Fatalf("first span should not be current")
	}
	// Widths should sum to ~100.
	var sum float64
	for _, s := range spans {
		sum += s.WidthPct
		if s.Duration <= 0 {
			t.Fatalf("phase %d has non-positive duration", s.Phase)
		}
	}
	if sum < 99.5 || sum > 100.5 {
		t.Fatalf("widths should sum to ~100, got %.2f", sum)
	}
	// Scope&Declare ran ~3 days, Build ~5 days; Build should be the wider span.
	if spans[1].WidthPct <= spans[0].WidthPct {
		t.Fatalf("expected Build span wider than Scope&Declare; got %.1f vs %.1f", spans[1].WidthPct, spans[0].WidthPct)
	}
}

// TestWeekMaterial verifies the Sunday-recap week aggregation returns the
// Mon–Sun logs for the week containing the given date.
func TestWeekMaterial(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC) // Wednesday
	svc := newTestService(t, clock)
	svc.now = func() time.Time { return clock }
	sp := mustSprint(t, svc)

	// Logs across the week (Mon 22, Wed 24) and one outside the week (next Mon 29).
	for _, d := range []string{"2026-06-22", "2026-06-24", "2026-06-29"} {
		if _, err := svc.RecordLog(ctx, RecordLogInput{SprintID: &sp.ID, LogDate: d, WorkedOn: "work " + d}); err != nil {
			t.Fatalf("RecordLog %s: %v", d, err)
		}
	}

	mat, err := svc.WeekMaterial(ctx, &sp.ID, "2026-06-24")
	if err != nil {
		t.Fatalf("WeekMaterial: %v", err)
	}
	if len(mat) != 2 {
		t.Fatalf("expected 2 logs in the week of 2026-06-22..28, got %d", len(mat))
	}
	if mat[0].LogDate != "2026-06-22" || mat[1].LogDate != "2026-06-24" {
		t.Fatalf("unexpected week material order/content: %+v", mat)
	}
}

// TestDeclarationBackLink verifies a declaration post links back to its sprint
// via sprints.declaration_post_id.
func TestDeclarationBackLink(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))
	sp := mustSprint(t, svc)

	post, err := svc.CreatePost(ctx, CreatePostInput{
		SprintID: &sp.ID, IsDeclaration: true, Source: domain.SourceForm,
	})
	if err != nil {
		t.Fatalf("CreatePost declaration: %v", err)
	}
	got, err := svc.GetSprint(ctx, sp.ID)
	if err != nil {
		t.Fatalf("GetSprint: %v", err)
	}
	if got.DeclarationPostID == nil || *got.DeclarationPostID != post.ID {
		t.Fatalf("expected declaration_post_id=%d, got %v", post.ID, got.DeclarationPostID)
	}
}
