package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// UpsertWeeklyReview creates or updates a weekly review.
func (s *Store) UpsertWeeklyReview(ctx context.Context, r domain.WeeklyReview) (domain.WeeklyReview, error) {
	now := nowUTC()
	// Try update first.
	res, err := s.db.ExecContext(ctx,
		`UPDATE weekly_reviews SET what_shipped = ?, what_slipped = ?, carry_forward = ?, one_learning = ?, updated_at = ?
		 WHERE user_id = ? AND iso_week = ?`,
		nullStr(r.WhatShipped), nullStr(r.WhatSlipped), nullStr(r.CarryForward), nullStr(r.OneLearning), now,
		r.UserID, r.ISOWeek)
	if err != nil {
		return domain.WeeklyReview{}, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return s.GetWeeklyReview(ctx, r.UserID, r.ISOWeek)
	}

	// Insert new.
	insertRes, err := s.db.ExecContext(ctx,
		`INSERT INTO weekly_reviews (user_id, iso_week, what_shipped, what_slipped, carry_forward, one_learning, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.UserID, r.ISOWeek, nullStr(r.WhatShipped), nullStr(r.WhatSlipped), nullStr(r.CarryForward), nullStr(r.OneLearning), now, now)
	if err != nil {
		return domain.WeeklyReview{}, err
	}
	id, _ := insertRes.LastInsertId()
	r.ID = id
	r.CreatedAt = parseTime(now)
	r.UpdatedAt = parseTime(now)
	return r, nil
}

// GetWeeklyReview returns a review for a specific user + ISO week.
func (s *Store) GetWeeklyReview(ctx context.Context, userID int64, isoWeek string) (domain.WeeklyReview, error) {
	var r domain.WeeklyReview
	var shipped, slipped, carry, learning sql.NullString
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, iso_week, what_shipped, what_slipped, carry_forward, one_learning, created_at, updated_at
		 FROM weekly_reviews WHERE user_id = ? AND iso_week = ?`, userID, isoWeek).
		Scan(&r.ID, &r.UserID, &r.ISOWeek, &shipped, &slipped, &carry, &learning, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WeeklyReview{}, ErrNotFound
		}
		return domain.WeeklyReview{}, err
	}
	r.WhatShipped = strFromNull(shipped)
	r.WhatSlipped = strFromNull(slipped)
	r.CarryForward = strFromNull(carry)
	r.OneLearning = strFromNull(learning)
	r.CreatedAt = parseTime(createdAt)
	r.UpdatedAt = parseTime(updatedAt)
	return r, nil
}

// ListWeeklyReviews returns recent reviews for a user.
func (s *Store) ListWeeklyReviews(ctx context.Context, userID int64, limit int) ([]domain.WeeklyReview, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, iso_week, what_shipped, what_slipped, carry_forward, one_learning, created_at, updated_at
		 FROM weekly_reviews WHERE user_id = ? ORDER BY iso_week DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WeeklyReview
	for rows.Next() {
		var r domain.WeeklyReview
		var shipped, slipped, carry, learning sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&r.ID, &r.UserID, &r.ISOWeek, &shipped, &slipped, &carry, &learning, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		r.WhatShipped = strFromNull(shipped)
		r.WhatSlipped = strFromNull(slipped)
		r.CarryForward = strFromNull(carry)
		r.OneLearning = strFromNull(learning)
		r.CreatedAt = parseTime(createdAt)
		r.UpdatedAt = parseTime(updatedAt)
		out = append(out, r)
	}
	return out, rows.Err()
}
