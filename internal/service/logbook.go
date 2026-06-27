package service

import (
	"context"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// RecordLogInput is a daily build note. It is the raw material recaps draw from.
type RecordLogInput struct {
	SprintID        *int64
	LogDate         string // defaults to today
	WorkedOn        string
	WhatHappened    string
	Insight         string
	NextUp          string
	Blocker         string
	BlockerDecision *domain.BlockerDecision
	Source          domain.EventSource
}

// RecordLog creates or updates the daily log for a date (one log per sprint per
// day). Defaults the sprint to the current active sprint and the date to today.
func (s *Service) RecordLog(ctx context.Context, in RecordLogInput) (domain.DailyLog, error) {
	if trim(in.WorkedOn) == "" {
		return domain.DailyLog{}, validationf("'worked on' is required for a daily log")
	}
	if in.LogDate == "" {
		in.LogDate = s.Today()
	}
	if in.BlockerDecision != nil && !in.BlockerDecision.Valid() {
		return domain.DailyLog{}, validationf("invalid blocker decision %q", *in.BlockerDecision)
	}
	if in.SprintID == nil {
		if cur, err := s.store.GetCurrentSprint(ctx); err == nil {
			in.SprintID = &cur.ID
		}
	}

	// Upsert: update if a log already exists for this sprint+date.
	existing, err := s.store.GetLogByDate(ctx, in.SprintID, in.LogDate)
	if err == nil {
		existing.WorkedOn = trim(in.WorkedOn)
		existing.WhatHappened = in.WhatHappened
		existing.Insight = in.Insight
		existing.NextUp = in.NextUp
		existing.Blocker = in.Blocker
		existing.BlockerDecision = in.BlockerDecision
		if err := s.store.UpdateLog(ctx, existing); err != nil {
			return domain.DailyLog{}, err
		}
		s.appendEvent(ctx, "log.updated", in.Source, in.SprintID, nil, "Updated log for "+in.LogDate, "")
		return s.store.GetLog(ctx, existing.ID)
	} else if err != ErrNotFound {
		return domain.DailyLog{}, err
	}

	log, err := s.store.CreateLog(ctx, domain.DailyLog{
		SprintID: in.SprintID, LogDate: in.LogDate, WorkedOn: trim(in.WorkedOn),
		WhatHappened: in.WhatHappened, Insight: in.Insight, NextUp: in.NextUp,
		Blocker: in.Blocker, BlockerDecision: in.BlockerDecision,
	})
	if err != nil {
		return domain.DailyLog{}, err
	}
	s.appendEvent(ctx, "log.recorded", in.Source, in.SprintID, nil, "Logged: "+log.WorkedOn, "")
	return log, nil
}

// LogForToday returns today's log for the current sprint, or ErrNotFound.
func (s *Service) LogForToday(ctx context.Context) (domain.DailyLog, error) {
	var sprintID *int64
	if cur, err := s.store.GetCurrentSprint(ctx); err == nil {
		sprintID = &cur.ID
	}
	return s.store.GetLogByDate(ctx, sprintID, s.Today())
}

// ListLogs returns recent daily logs across all sprints.
func (s *Service) ListLogs(ctx context.Context, limit int) ([]domain.DailyLog, error) {
	return s.store.ListLogs(ctx, limit)
}

// ListLogsBySprint returns a sprint's logs, newest first.
func (s *Service) ListLogsBySprint(ctx context.Context, sprintID int64) ([]domain.DailyLog, error) {
	return s.store.ListLogsBySprint(ctx, sprintID)
}

// ListEvents returns the recent career-event stream (the SRE "logs" view).
func (s *Service) ListEvents(ctx context.Context, limit int) ([]domain.CareerEvent, error) {
	return s.store.ListEvents(ctx, limit)
}

// WeekMaterial returns the daily logs for the calendar week containing the
// given date, for a sprint. This is the raw material a Sunday recap is built
// from (spec 05: the recap "pulls from the week's daily logs rather than
// starting from scratch"). The window is Monday..Sunday of that week. A nil
// sprintID falls back to the current active sprint.
func (s *Service) WeekMaterial(ctx context.Context, sprintID *int64, anyDateInWeek string) ([]domain.DailyLog, error) {
	if sprintID == nil {
		if cur, err := s.store.GetCurrentSprint(ctx); err == nil {
			sprintID = &cur.ID
		}
	}
	ref := s.now().In(s.loc)
	if anyDateInWeek != "" {
		if t, err := time.ParseInLocation("2006-01-02", anyDateInWeek, s.loc); err == nil {
			ref = t
		}
	}
	// Monday as the start of the week.
	offset := (int(ref.Weekday()) + 6) % 7 // Mon=0 ... Sun=6
	monday := ref.AddDate(0, 0, -offset)
	sunday := monday.AddDate(0, 0, 6)
	return s.store.ListLogsByDateRange(ctx, sprintID,
		monday.Format("2006-01-02"), sunday.Format("2006-01-02"))
}

// DeleteLog removes a daily log by ID. Records a career event.
func (s *Service) DeleteLog(ctx context.Context, id int64, src domain.EventSource) error {
	log, err := s.store.GetLog(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteLog(ctx, id); err != nil {
		return err
	}
	s.appendEvent(ctx, "log.deleted", src, log.SprintID, nil, "Deleted log for "+log.LogDate, "")
	return nil
}
