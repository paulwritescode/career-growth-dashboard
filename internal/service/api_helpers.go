package service

import (
	"context"
	"fmt"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// CreateLogInput is a simplified input for the REST API log creation.
type CreateLogInput struct {
	WorkedOn     string
	WhatHappened string
	Insight      string
	NextUp       string
	SprintID     *int64
	Source       domain.EventSource
}

// CreateLog is a thin wrapper around RecordLog for the REST API.
func (s *Service) CreateLog(ctx context.Context, in CreateLogInput) (domain.DailyLog, error) {
	return s.RecordLog(ctx, RecordLogInput{
		WorkedOn:     in.WorkedOn,
		WhatHappened: in.WhatHappened,
		Insight:      in.Insight,
		NextUp:       in.NextUp,
		SprintID:     in.SprintID,
		Source:       in.Source,
	})
}

// AppendEvent is a public wrapper so the API layer can record events directly.
func (s *Service) AppendEvent(ctx context.Context, event domain.CareerEvent) error {
	return s.store.AppendEvent(ctx, event)
}

// PushMetricPoint stores an API-pushed metric data point.
func (s *Service) PushMetricPoint(ctx context.Context, mp domain.MetricPoint) error {
	_, err := s.store.DB().ExecContext(ctx,
		`INSERT INTO metric_points (name, value, tags, occurred_at) VALUES (?, ?, ?, ?)`,
		mp.Name, mp.Value, mp.Tags, mp.OccurredAt.UTC().Format("2006-01-02T15:04:05Z07:00"))
	if err != nil {
		return fmt.Errorf("push metric: %w", err)
	}
	return nil
}

// PushTraceSpan stores an API-pushed trace span and returns its ID.
func (s *Service) PushTraceSpan(ctx context.Context, span domain.TraceSpan) (int64, error) {
	res, err := s.store.DB().ExecContext(ctx,
		`INSERT INTO trace_spans (sprint_id, phase, name, duration_ms, started_at) VALUES (?, ?, ?, ?, ?)`,
		nullInt64Ptr(span.SprintID), nullIntPtr(span.Phase), span.Name, span.DurationMS,
		span.StartedAt.UTC().Format("2006-01-02T15:04:05Z07:00"))
	if err != nil {
		return 0, fmt.Errorf("push span: %w", err)
	}
	return res.LastInsertId()
}

// DeliverableCounts returns done and total deliverable counts for a sprint.
func (s *Service) DeliverableCounts(ctx context.Context, sprintID int64) (done, total int, err error) {
	return s.store.CountDeliverables(ctx, sprintID)
}

func nullInt64Ptr(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullIntPtr(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
