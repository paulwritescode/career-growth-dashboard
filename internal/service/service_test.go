package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
	"github.com/paulkinyatti/local-scava/internal/store"
)

// fixedClock returns a Service whose "now" is pinned to the given time in UTC.
func newTestService(t *testing.T, now time.Time) *Service {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "svc.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := New(st)
	svc.now = func() time.Time { return now }
	svc.loc = time.UTC
	return svc
}

func mustSprint(t *testing.T, svc *Service) domain.Sprint {
	t.Helper()
	sp, err := svc.CreateSprint(context.Background(), CreateSprintInput{
		SkillName: "Redis", MicroappOneLiner: "a rate limiter", CoreFeature: "sliding window",
	})
	if err != nil {
		t.Fatalf("CreateSprint: %v", err)
	}
	return sp
}

func TestCreateSprintValidationAndRules(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)) // Monday

	if _, err := svc.CreateSprint(ctx, CreateSprintInput{MicroappOneLiner: "x", CoreFeature: "y"}); err == nil {
		t.Fatal("expected error for missing skill name")
	}

	sp := mustSprint(t, svc)
	if sp.Status != domain.SprintActive || sp.CurrentPhase != domain.PhaseScopeDeclare {
		t.Fatalf("unexpected defaults: %+v", sp)
	}
	if sp.StartedOn == nil || *sp.StartedOn != "2026-06-22" {
		t.Fatalf("expected started_on today, got %v", sp.StartedOn)
	}

	// Checklist seeded.
	items, _ := svc.Checklist(ctx, sp.ID)
	if len(items) != 18 {
		t.Fatalf("expected 18 seeded checklist items, got %d", len(items))
	}

	// One active sprint rule.
	if _, err := svc.CreateSprint(ctx, CreateSprintInput{SkillName: "Kafka", MicroappOneLiner: "x", CoreFeature: "y"}); err != ErrActiveSprintExists {
		t.Fatalf("expected ErrActiveSprintExists, got %v", err)
	}
}

func TestShipRequiresLiveURL(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))
	sp := mustSprint(t, svc)

	if _, err := svc.SetStatus(ctx, sp.ID, SetStatusInput{Status: domain.SprintShipped}); err != ErrLiveURLRequired {
		t.Fatalf("expected ErrLiveURLRequired, got %v", err)
	}
	shipped, err := svc.SetStatus(ctx, sp.ID, SetStatusInput{Status: domain.SprintShipped, LiveURL: "https://demo.app"})
	if err != nil {
		t.Fatalf("SetStatus ship: %v", err)
	}
	if !shipped.IsShipped() || !shipped.HasLiveURL() || shipped.EndedOn == nil {
		t.Fatalf("shipped sprint not finalized: %+v", shipped)
	}

	// A shipped sprint frees the active slot.
	if _, err := svc.CreateSprint(ctx, CreateSprintInput{SkillName: "Kafka", MicroappOneLiner: "x", CoreFeature: "y"}); err != nil {
		t.Fatalf("expected to create new active sprint after shipping, got %v", err)
	}
}

func TestSetPhase(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))
	sp := mustSprint(t, svc)
	got, err := svc.SetPhase(ctx, sp.ID, domain.PhaseBuild, domain.SourceForm)
	if err != nil || got.CurrentPhase != domain.PhaseBuild {
		t.Fatalf("SetPhase = %+v, %v", got, err)
	}
	if _, err := svc.SetPhase(ctx, sp.ID, domain.Phase(9), domain.SourceForm); err == nil {
		t.Fatal("expected invalid phase error")
	}
}

func TestPostPublishAndSnapshot(t *testing.T) {
	ctx := context.Background()
	// Monday so it's a cadence day, not a Friday buffer.
	svc := newTestService(t, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))
	mustSprint(t, svc)

	post, err := svc.CreatePost(ctx, CreatePostInput{})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if len(post.Tiers) != 3 {
		t.Fatalf("expected 3 tiers, got %d", len(post.Tiers))
	}

	// MarkPublished requires a URL.
	if _, err := svc.MarkPublished(ctx, post.ID, domain.TierLinkedIn, "", domain.SourceForm); err != ErrURLRequired {
		t.Fatalf("expected ErrURLRequired, got %v", err)
	}
	if _, err := svc.MarkPublished(ctx, post.ID, domain.TierLinkedIn, "https://li.com/1", domain.SourceForm); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}

	snap, err := svc.TodaySnapshot(ctx)
	if err != nil {
		t.Fatalf("TodaySnapshot: %v", err)
	}
	if !snap.HasPost || !snap.CredibilityUp || snap.IsBufferDay {
		t.Fatalf("snapshot wrong: %+v", snap)
	}
	if !snap.HasSprint || snap.PhaseHealth != HealthRed {
		// Phase 1 has 5 gates, all unchecked => >2 => red.
		t.Fatalf("expected red phase health with 5 unchecked, got %s", snap.PhaseHealth)
	}
}

func TestRecordLogUpsert(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))
	mustSprint(t, svc)

	if _, err := svc.RecordLog(ctx, RecordLogInput{}); err == nil {
		t.Fatal("expected error for empty worked_on")
	}
	l1, err := svc.RecordLog(ctx, RecordLogInput{WorkedOn: "first"})
	if err != nil {
		t.Fatalf("RecordLog: %v", err)
	}
	l2, err := svc.RecordLog(ctx, RecordLogInput{WorkedOn: "updated"})
	if err != nil {
		t.Fatalf("RecordLog update: %v", err)
	}
	if l1.ID != l2.ID {
		t.Fatalf("expected upsert to same row, got %d != %d", l1.ID, l2.ID)
	}
	if l2.WorkedOn != "updated" {
		t.Fatalf("expected updated content, got %q", l2.WorkedOn)
	}
	logs, _ := svc.ListLogs(ctx, 10)
	if len(logs) != 1 {
		t.Fatalf("expected 1 log after upsert, got %d", len(logs))
	}
}

func TestPhaseHealthThresholds(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))
	sp := mustSprint(t, svc)

	items, _ := svc.Checklist(ctx, sp.ID)
	// Phase 1 has 5 gates. Check 3 to leave 2 unchecked => amber.
	checked := 0
	for _, it := range items {
		if it.Phase == domain.PhaseScopeDeclare && checked < 3 {
			if err := svc.ToggleChecklistItem(ctx, it.ID, true); err != nil {
				t.Fatalf("toggle: %v", err)
			}
			checked++
		}
	}
	sp, _ = svc.GetSprint(ctx, sp.ID)
	h, n, err := svc.phaseHealth(ctx, sp)
	if err != nil {
		t.Fatalf("phaseHealth: %v", err)
	}
	if n != 2 || h != HealthAmber {
		t.Fatalf("expected amber with 2 unchecked, got %s (%d)", h, n)
	}
}

func TestCadenceAndStreaks(t *testing.T) {
	ctx := context.Background()
	// "Today" = Wednesday 2026-06-24.
	svc := newTestService(t, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	mustSprint(t, svc)

	// Publish a credibility tier on Mon, Tue, Wed (2026-06-22..24).
	for _, d := range []string{"2026-06-22", "2026-06-23", "2026-06-24"} {
		p, err := svc.CreatePost(ctx, CreatePostInput{PostDate: d})
		if err != nil {
			t.Fatalf("CreatePost %s: %v", d, err)
		}
		if _, err := svc.MarkPublished(ctx, p.ID, domain.TierBlog, "https://b/"+d, domain.SourceForm); err != nil {
			t.Fatalf("publish %s: %v", d, err)
		}
	}

	streaks, err := svc.Streaks(ctx, 30)
	if err != nil {
		t.Fatalf("Streaks: %v", err)
	}
	if streaks.Current != 3 || streaks.Longest != 3 {
		t.Fatalf("expected current=3 longest=3, got %+v", streaks)
	}

	// Cadence over the last 7 days (Thu..Wed). Expected non-Friday days with a
	// credibility post: only the 3 we published.
	rate, err := svc.CadenceRate(ctx, 7)
	if err != nil {
		t.Fatalf("CadenceRate: %v", err)
	}
	if rate <= 0 || rate > 1 {
		t.Fatalf("cadence rate out of range: %v", rate)
	}

	mix, err := svc.TierMix(ctx, 30)
	if err != nil {
		t.Fatalf("TierMix: %v", err)
	}
	if mix[domain.TierBlog] != 3 {
		t.Fatalf("expected 3 blog publishes, got %d", mix[domain.TierBlog])
	}
}
