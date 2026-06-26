package domain

import "testing"

func TestPhaseValidAndLabel(t *testing.T) {
	if !PhaseBuild.Valid() {
		t.Fatal("PhaseBuild should be valid")
	}
	if Phase(0).Valid() || Phase(5).Valid() {
		t.Fatal("out-of-range phases should be invalid")
	}
	want := map[Phase]string{
		PhaseScopeDeclare:   "Scope & Declare",
		PhaseBuild:          "Build",
		PhasePolishDocument: "Polish & Document",
		PhaseDeployShowcase: "Deploy & Showcase",
	}
	for p, label := range want {
		if got := p.Label(); got != label {
			t.Errorf("Phase(%d).Label() = %q, want %q", p, got, label)
		}
	}
	if len(AllPhases()) != 4 {
		t.Fatalf("AllPhases() len = %d, want 4", len(AllPhases()))
	}
}

func TestSprintStatusValid(t *testing.T) {
	for _, s := range []SprintStatus{SprintPlanned, SprintActive, SprintShipped, SprintAbandoned} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if SprintStatus("bogus").Valid() {
		t.Error("bogus status should be invalid")
	}
}

func TestTierHelpers(t *testing.T) {
	if len(AllTiers()) != 3 {
		t.Fatalf("AllTiers() len = %d, want 3", len(AllTiers()))
	}
	if !TierBlog.IsCredibility() || !TierLinkedIn.IsCredibility() {
		t.Error("blog and linkedin must be credibility tiers")
	}
	if TierX.IsCredibility() {
		t.Error("x must not be a credibility tier")
	}
	if !TierLinkedIn.Valid() || Tier("bogus").Valid() {
		t.Error("tier validity check failed")
	}
}

func TestOtherEnumValidity(t *testing.T) {
	if !TierPublished.Valid() || TierStatus("x").Valid() {
		t.Error("tier status validity failed")
	}
	if !VisualADR.Valid() || VisualKind("x").Valid() {
		t.Error("visual kind validity failed")
	}
	if !PostRecap.Valid() || PostType("x").Valid() {
		t.Error("post type validity failed")
	}
	if !BlockerCut.Valid() || BlockerDecision("x").Valid() {
		t.Error("blocker decision validity failed")
	}
	if !ADRDecided.Valid() || ADRStatus("x").Valid() {
		t.Error("adr status validity failed")
	}
	if !SourceChat.Valid() || EventSource("x").Valid() {
		t.Error("event source validity failed")
	}
}

func TestSprintHelpers(t *testing.T) {
	url := "https://example.com"
	s := Sprint{Status: SprintShipped, LiveURL: &url}
	if !s.IsShipped() || !s.HasLiveURL() {
		t.Error("shipped sprint with url should report both true")
	}
	empty := ""
	s2 := Sprint{Status: SprintActive, LiveURL: &empty}
	if s2.IsShipped() || s2.HasLiveURL() {
		t.Error("active sprint with empty url should report both false")
	}
}
