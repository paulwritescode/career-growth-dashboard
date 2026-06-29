package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestMigrationsApply(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	version, err := st.MigrateVersion(ctx)
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version != 9 {
		t.Fatalf("expected migration version 9, got %d", version)
	}

	// All expected tables should exist.
	for _, tbl := range []string{"sprints", "checklist_items", "daily_logs", "posts", "post_tiers", "adrs", "career_events",
		"users", "sessions", "api_keys", "onboarding_state", "user_blocks", "user_platforms",
		"deliverables", "sprint_templates", "habits", "habit_entries", "weekly_reviews", "todos",
		"metric_points", "trace_spans"} {
		var name string
		err := st.DB().QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", tbl, err)
		}
	}

	// Re-running migrate must be a no-op (idempotent).
	if err := st.migrate(ctx); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
}

func TestSprintCRUDAndChecklist(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	sp, err := st.CreateSprint(ctx, domain.Sprint{
		MonthLabel:       "2026-07",
		SkillName:        "Redis",
		MicroappOneLiner: "a rate limiter",
		CoreFeature:      "sliding window",
		CurrentPhase:     domain.PhaseScopeDeclare,
		Status:           domain.SprintActive,
	})
	if err != nil {
		t.Fatalf("CreateSprint: %v", err)
	}
	if sp.ID == 0 || sp.CreatedAt.IsZero() {
		t.Fatal("expected populated ID and CreatedAt")
	}

	// Single-active partial unique index must block a second active sprint.
	if _, err := st.CreateSprint(ctx, domain.Sprint{
		MonthLabel: "2026-08", SkillName: "Kafka", MicroappOneLiner: "x", CoreFeature: "y",
		CurrentPhase: domain.PhaseScopeDeclare, Status: domain.SprintActive,
	}); err == nil {
		t.Fatal("expected error creating a second active sprint")
	}

	got, err := st.GetCurrentSprint(ctx)
	if err != nil || got.ID != sp.ID {
		t.Fatalf("GetCurrentSprint = %+v, %v", got, err)
	}

	// Checklist seed + list + toggle.
	items := []domain.ChecklistItem{
		{Phase: domain.PhaseScopeDeclare, Label: "skill picked", SortOrder: 0},
		{Phase: domain.PhaseBuild, Label: "core works", SortOrder: 0},
	}
	if err := st.SeedChecklist(ctx, sp.ID, items); err != nil {
		t.Fatalf("SeedChecklist: %v", err)
	}
	list, err := st.ListChecklist(ctx, sp.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListChecklist len=%d err=%v", len(list), err)
	}
	if err := st.ToggleChecklistItem(ctx, list[0].ID, true); err != nil {
		t.Fatalf("Toggle: %v", err)
	}
	list, _ = st.ListChecklist(ctx, sp.ID)
	if !list[0].IsDone || list[0].DoneAt == nil {
		t.Fatal("expected first item done with timestamp")
	}
}

func TestPostTiersAndLog(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	log, err := st.CreateLog(ctx, domain.DailyLog{
		LogDate: "2026-06-26", WorkedOn: "fixed reconnect race",
	})
	if err != nil {
		t.Fatalf("CreateLog: %v", err)
	}

	post, err := st.CreatePost(ctx, domain.Post{
		PostDate: "2026-06-26", PostType: domain.PostDaily, SourceLogID: &log.ID,
		Title: "today",
	})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if len(post.Tiers) != 3 {
		t.Fatalf("expected 3 seeded tiers, got %d", len(post.Tiers))
	}
	if post.Tiers[0].Tier != domain.TierBlog {
		t.Fatalf("expected blog tier first, got %s", post.Tiers[0].Tier)
	}

	// Publish the LinkedIn tier.
	url := "https://linkedin.com/post/1"
	lt := post.Tiers[1]
	lt.Status = domain.TierPublished
	lt.URL = &url
	now := log.CreatedAt
	lt.PublishedAt = &now
	if err := st.UpdateTier(ctx, lt); err != nil {
		t.Fatalf("UpdateTier: %v", err)
	}
	reloaded, err := st.GetPostByDate(ctx, "2026-06-26")
	if err != nil {
		t.Fatalf("GetPostByDate: %v", err)
	}
	if reloaded.Tiers[1].Status != domain.TierPublished || reloaded.Tiers[1].URL == nil {
		t.Fatal("linkedin tier did not persist as published with url")
	}
}

func TestADRAndEvents(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	n, err := st.MaxADRNumber(ctx)
	if err != nil || n != 0 {
		t.Fatalf("MaxADRNumber empty = %d, %v", n, err)
	}
	adr, err := st.CreateADR(ctx, domain.ADR{Number: 1, Title: "use libSQL", Status: domain.ADRDecided})
	if err != nil {
		t.Fatalf("CreateADR: %v", err)
	}
	if adr.ID == 0 {
		t.Fatal("expected ADR id")
	}
	n, _ = st.MaxADRNumber(ctx)
	if n != 1 {
		t.Fatalf("MaxADRNumber = %d, want 1", n)
	}

	if err := st.AppendEvent(ctx, domain.CareerEvent{
		Kind: "adr.created", Source: domain.SourceForm, Summary: "created ADR-1",
	}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	events, err := st.ListEvents(ctx, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("ListEvents len=%d err=%v", len(events), err)
	}
	if events[0].Kind != "adr.created" {
		t.Fatalf("unexpected event kind %q", events[0].Kind)
	}
}
