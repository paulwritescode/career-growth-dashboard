package service

import (
	"context"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// PhaseHealth is the green/amber/red signal for the current phase's checklist,
// encoding the skill's "more than 2 boxes unchecked = address it" rule.
type PhaseHealth string

const (
	HealthGreen PhaseHealth = "green" // <=1 unchecked
	HealthAmber PhaseHealth = "amber" // exactly 2 unchecked
	HealthRed   PhaseHealth = "red"   // >2 unchecked
	HealthNone  PhaseHealth = "none"  // no active sprint
)

// TodaySnapshot is the headline state for the Overview page.
type TodaySnapshot struct {
	Date          string
	IsBufferDay   bool // Friday — no cadence expectation
	HasPost       bool
	Post          *domain.Post // today's post with tiers, if any
	LoggedToday   bool
	CredibilityUp bool // a blog/linkedin tier is published today
	HasSprint     bool
	Sprint        *domain.Sprint
	PhaseHealth   PhaseHealth
	UncheckedNow  int // unchecked gates in the current phase
}

// TodaySnapshot computes the Overview headline state.
func (s *Service) TodaySnapshot(ctx context.Context) (TodaySnapshot, error) {
	today := s.Today()
	snap := TodaySnapshot{Date: today, PhaseHealth: HealthNone}
	snap.IsBufferDay = s.now().In(s.loc).Weekday() == time.Friday

	if post, err := s.store.GetPostByDate(ctx, today); err == nil {
		snap.HasPost = true
		p := post
		snap.Post = &p
		for _, t := range post.Tiers {
			if t.IsPublished() && t.Tier.IsCredibility() {
				snap.CredibilityUp = true
			}
		}
	} else if err != ErrNotFound {
		return snap, err
	}

	if _, err := s.LogForToday(ctx); err == nil {
		snap.LoggedToday = true
	} else if err != ErrNotFound {
		return snap, err
	}

	if sp, err := s.store.GetCurrentSprint(ctx); err == nil {
		snap.HasSprint = true
		spv := sp
		snap.Sprint = &spv
		health, unchecked, err := s.phaseHealth(ctx, sp)
		if err != nil {
			return snap, err
		}
		snap.PhaseHealth = health
		snap.UncheckedNow = unchecked
	} else if err != ErrNotFound {
		return snap, err
	}
	return snap, nil
}

// phaseHealth counts unchecked gates in the sprint's current phase.
func (s *Service) phaseHealth(ctx context.Context, sp domain.Sprint) (PhaseHealth, int, error) {
	items, err := s.store.ListChecklist(ctx, sp.ID)
	if err != nil {
		return HealthNone, 0, err
	}
	unchecked := 0
	for _, it := range items {
		if it.Phase == sp.CurrentPhase && !it.IsDone {
			unchecked++
		}
	}
	switch {
	case unchecked > 2:
		return HealthRed, unchecked, nil
	case unchecked == 2:
		return HealthAmber, unchecked, nil
	default:
		return HealthGreen, unchecked, nil
	}
}

// CadenceRate returns the fraction (0..1) of expected cadence days in the
// trailing window that had a credibility-tier post published. Fridays are
// excluded as buffer days.
func (s *Service) CadenceRate(ctx context.Context, windowDays int) (float64, error) {
	if windowDays <= 0 {
		windowDays = 30
	}
	end := s.now().In(s.loc)
	start := end.AddDate(0, 0, -(windowDays - 1))
	stats, err := s.store.PublishStatsByDate(ctx, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, err
	}
	credByDate := map[string]bool{}
	for _, st := range stats {
		credByDate[st.Date] = st.Credibility > 0
	}
	expected, hit := 0, 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Friday {
			continue // buffer day
		}
		expected++
		if credByDate[d.Format("2006-01-02")] {
			hit++
		}
	}
	if expected == 0 {
		return 0, nil
	}
	return float64(hit) / float64(expected), nil
}

// Streaks holds the current and longest posting streaks (in cadence days).
type Streaks struct {
	Current int
	Longest int
}

// Streaks computes current and longest streaks of cadence days with a
// credibility-tier post, scanning back over the given number of days. Buffer
// Fridays neither count nor break a streak.
func (s *Service) Streaks(ctx context.Context, lookbackDays int) (Streaks, error) {
	if lookbackDays <= 0 {
		lookbackDays = 365
	}
	end := s.now().In(s.loc)
	start := end.AddDate(0, 0, -(lookbackDays - 1))
	stats, err := s.store.PublishStatsByDate(ctx, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return Streaks{}, err
	}
	cred := map[string]bool{}
	for _, st := range stats {
		cred[st.Date] = st.Credibility > 0
	}

	var longest, run, current int
	currentBroken := false
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Friday {
			continue
		}
		if cred[d.Format("2006-01-02")] {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	// Current streak: walk backward from today over non-Friday days.
	for d := end; !d.Before(start); d = d.AddDate(0, 0, -1) {
		if d.Weekday() == time.Friday {
			continue
		}
		if cred[d.Format("2006-01-02")] {
			if !currentBroken {
				current++
			}
		} else {
			currentBroken = true
		}
	}
	return Streaks{Current: current, Longest: longest}, nil
}

// ShipRateResult reports how many ended sprints actually shipped with a URL.
type ShipRateResult struct {
	Ended   int
	Shipped int
	Rate    float64
}

// ShipRate computes the fraction of ended sprints that shipped with a live URL.
func (s *Service) ShipRate(ctx context.Context) (ShipRateResult, error) {
	sprints, err := s.store.ListSprints(ctx)
	if err != nil {
		return ShipRateResult{}, err
	}
	var res ShipRateResult
	for _, sp := range sprints {
		ended := sp.Status == domain.SprintShipped || sp.Status == domain.SprintAbandoned
		if !ended {
			continue
		}
		res.Ended++
		if sp.IsShipped() && sp.HasLiveURL() {
			res.Shipped++
		}
	}
	if res.Ended > 0 {
		res.Rate = float64(res.Shipped) / float64(res.Ended)
	}
	return res, nil
}

// TierMix returns the count of published tiers per tier within the window.
func (s *Service) TierMix(ctx context.Context, windowDays int) (map[domain.Tier]int, error) {
	if windowDays <= 0 {
		windowDays = 30
	}
	end := s.now().In(s.loc)
	start := end.AddDate(0, 0, -(windowDays - 1))
	return s.store.PublishedTierCounts(ctx, start.Format("2006-01-02"), end.Format("2006-01-02"))
}

// CadenceCell is a single day in the cadence heatmap.
type CadenceCell struct {
	Date        string
	Published   int  // tiers published that day (0..3)
	IsBufferDay bool // Friday
}

// CadenceHeatmap returns one cell per day across the trailing window, oldest first.
func (s *Service) CadenceHeatmap(ctx context.Context, windowDays int) ([]CadenceCell, error) {
	if windowDays <= 0 {
		windowDays = 30
	}
	end := s.now().In(s.loc)
	start := end.AddDate(0, 0, -(windowDays - 1))
	stats, err := s.store.PublishStatsByDate(ctx, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	pub := map[string]int{}
	for _, st := range stats {
		pub[st.Date] = st.Published
	}
	var cells []CadenceCell
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		cells = append(cells, CadenceCell{
			Date:        ds,
			Published:   pub[ds],
			IsBufferDay: d.Weekday() == time.Friday,
		})
	}
	return cells, nil
}
