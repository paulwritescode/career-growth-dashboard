package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// PhaseSpan is one segment of a sprint's "trace": the time spent in a single
// phase, with the checklist completion reached in that phase. It is the
// career-growth analogue of a span in a distributed trace.
type PhaseSpan struct {
	Phase        domain.Phase
	Label        string
	StartedAt    time.Time
	EndedAt      time.Time
	Duration     time.Duration
	DurationText string  // human-readable, e.g. "5d 3h"
	OffsetPct    float64 // left offset within the trace (0..100)
	WidthPct     float64 // width within the trace (0..100)
	DoneGates    int
	TotalGates   int
	IsCurrent    bool // the phase the sprint is in now (open span)
}

// SprintTrace reconstructs a sprint as a left-to-right waterfall of phase
// spans, with each span's width proportional to the time spent in that phase
// (spec 09 — "a sprint traced through its four phases"). Phase boundaries come
// from sprint.created (phase 1 start) and sprint.phase_changed events; the
// final span is open until ended_on/now.
func (s *Service) SprintTrace(ctx context.Context, sprintID int64) ([]PhaseSpan, error) {
	sp, err := s.store.GetSprint(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	events, err := s.store.ListEventsBySprint(ctx, sprintID)
	if err != nil {
		return nil, err
	}

	// Build phase boundaries: (phase, startedAt) in chronological order. Phase 1
	// begins when the sprint was created. Prefer the sprint.created event time
	// (recorded with the service clock) and fall back to the row's created_at.
	type boundary struct {
		phase domain.Phase
		at    time.Time
	}
	phase1Start := sp.CreatedAt
	for _, e := range events {
		if e.Kind == "sprint.created" {
			phase1Start = e.OccurredAt
			break
		}
	}
	bounds := []boundary{{phase: domain.PhaseScopeDeclare, at: phase1Start}}
	for _, e := range events {
		if e.Kind != "sprint.phase_changed" {
			continue
		}
		var d struct {
			From int `json:"from"`
			To   int `json:"to"`
		}
		if err := json.Unmarshal([]byte(e.Detail), &d); err != nil || d.To == 0 {
			continue
		}
		bounds = append(bounds, boundary{phase: domain.Phase(d.To), at: e.OccurredAt})
	}

	// Trace end: ended_on for finished sprints, else now.
	end := s.now().UTC()
	if (sp.Status == domain.SprintShipped || sp.Status == domain.SprintAbandoned) && sp.EndedOn != nil {
		if t, perr := time.Parse("2006-01-02", *sp.EndedOn); perr == nil {
			// Use end-of-day so a same-day ship still shows a non-zero span.
			et := t.Add(24*time.Hour - time.Second)
			if et.After(bounds[len(bounds)-1].at) {
				end = et
			}
		}
	}

	// Checklist completion per phase (the span's "health" colour).
	items, err := s.store.ListChecklist(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	done := map[domain.Phase]int{}
	total := map[domain.Phase]int{}
	for _, it := range items {
		total[it.Phase]++
		if it.IsDone {
			done[it.Phase]++
		}
	}

	// Materialize spans from consecutive boundaries.
	spans := make([]PhaseSpan, 0, len(bounds))
	var totalDur time.Duration
	for i, b := range bounds {
		spanEnd := end
		if i+1 < len(bounds) {
			spanEnd = bounds[i+1].at
		}
		if spanEnd.Before(b.at) {
			spanEnd = b.at
		}
		dur := spanEnd.Sub(b.at)
		totalDur += dur
		spans = append(spans, PhaseSpan{
			Phase:        b.phase,
			Label:        b.phase.Label(),
			StartedAt:    b.at,
			EndedAt:      spanEnd,
			Duration:     dur,
			DurationText: humanDuration(dur),
			DoneGates:    done[b.phase],
			TotalGates:   total[b.phase],
			IsCurrent:    i == len(bounds)-1 && sp.Status == domain.SprintActive,
		})
	}

	// Compute offset/width percentages across the full trace duration.
	if totalDur <= 0 {
		// Degenerate (everything in one instant): equal widths.
		w := 100.0 / float64(len(spans))
		for i := range spans {
			spans[i].OffsetPct = float64(i) * w
			spans[i].WidthPct = w
		}
		return spans, nil
	}
	var acc time.Duration
	for i := range spans {
		spans[i].OffsetPct = float64(acc) / float64(totalDur) * 100
		spans[i].WidthPct = float64(spans[i].Duration) / float64(totalDur) * 100
		acc += spans[i].Duration
	}
	return spans, nil
}

// humanDuration renders a duration as a compact "Nd Nh" / "Nh Nm" / "Nm" string.
func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return itoa(days) + "d " + itoa(hours) + "h"
	case hours > 0:
		return itoa(hours) + "h " + itoa(mins) + "m"
	default:
		return itoa(mins) + "m"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
