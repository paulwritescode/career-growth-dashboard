package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

const logCols = `id, sprint_id, log_date, worked_on, what_happened, insight, next_up,
	blocker, blocker_decision, created_at, updated_at`

func scanLog(row interface{ Scan(...any) error }) (domain.DailyLog, error) {
	var l domain.DailyLog
	var (
		sprintID             sql.NullInt64
		blockerDec           sql.NullString
		createdAt, updatedAt string
	)
	if err := row.Scan(&l.ID, &sprintID, &l.LogDate, &l.WorkedOn, &l.WhatHappened,
		&l.Insight, &l.NextUp, &l.Blocker, &blockerDec, &createdAt, &updatedAt); err != nil {
		return domain.DailyLog{}, err
	}
	l.SprintID = intFromNull(sprintID)
	l.LogDate = dateOnly(l.LogDate)
	if blockerDec.Valid {
		bd := domain.BlockerDecision(blockerDec.String)
		l.BlockerDecision = &bd
	}
	l.CreatedAt = parseTime(createdAt)
	l.UpdatedAt = parseTime(updatedAt)
	return l, nil
}

// CreateLog inserts a daily log and returns it with its ID.
func (s *Store) CreateLog(ctx context.Context, in domain.DailyLog) (domain.DailyLog, error) {
	now := nowUTC()
	var bd any
	if in.BlockerDecision != nil {
		bd = string(*in.BlockerDecision)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO daily_logs
		(sprint_id, log_date, worked_on, what_happened, insight, next_up, blocker,
		 blocker_decision, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		nullInt(in.SprintID), in.LogDate, in.WorkedOn, in.WhatHappened, in.Insight,
		in.NextUp, in.Blocker, bd, now, now)
	if err != nil {
		return domain.DailyLog{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.DailyLog{}, err
	}
	return s.GetLog(ctx, id)
}

// GetLog returns a daily log by ID, or ErrNotFound.
func (s *Store) GetLog(ctx context.Context, id int64) (domain.DailyLog, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+logCols+` FROM daily_logs WHERE id = ?`, id)
	l, err := scanLog(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DailyLog{}, ErrNotFound
	}
	return l, err
}

// GetLogByDate returns the log for a sprint on a given local date, or ErrNotFound.
func (s *Store) GetLogByDate(ctx context.Context, sprintID *int64, date string) (domain.DailyLog, error) {
	var row *sql.Row
	if sprintID == nil {
		row = s.db.QueryRowContext(ctx,
			`SELECT `+logCols+` FROM daily_logs WHERE sprint_id IS NULL AND log_date = ?`, date)
	} else {
		row = s.db.QueryRowContext(ctx,
			`SELECT `+logCols+` FROM daily_logs WHERE sprint_id = ? AND log_date = ?`, *sprintID, date)
	}
	l, err := scanLog(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DailyLog{}, ErrNotFound
	}
	return l, err
}

// ListLogs returns the most recent logs across all sprints, newest first.
func (s *Store) ListLogs(ctx context.Context, limit int) ([]domain.DailyLog, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+logCols+` FROM daily_logs ORDER BY log_date DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectLogs(rows)
}

// ListLogsBySprint returns all logs for a sprint, newest first.
func (s *Store) ListLogsBySprint(ctx context.Context, sprintID int64) ([]domain.DailyLog, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+logCols+` FROM daily_logs WHERE sprint_id = ? ORDER BY log_date DESC, id DESC`, sprintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectLogs(rows)
}

// ListLogsByDateRange returns logs for a sprint within [from, to] (inclusive
// YYYY-MM-DD), oldest first. A nil sprintID matches orphan logs (sprint_id NULL).
func (s *Store) ListLogsByDateRange(ctx context.Context, sprintID *int64, from, to string) ([]domain.DailyLog, error) {
	var rows *sql.Rows
	var err error
	if sprintID == nil {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+logCols+` FROM daily_logs WHERE sprint_id IS NULL AND log_date BETWEEN ? AND ?
			 ORDER BY log_date ASC, id ASC`, from, to)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+logCols+` FROM daily_logs WHERE sprint_id = ? AND log_date BETWEEN ? AND ?
			 ORDER BY log_date ASC, id ASC`, *sprintID, from, to)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectLogs(rows)
}

// UpdateLog persists the mutable fields of a daily log.
func (s *Store) UpdateLog(ctx context.Context, l domain.DailyLog) error {
	var bd any
	if l.BlockerDecision != nil {
		bd = string(*l.BlockerDecision)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE daily_logs SET
		sprint_id=?, log_date=?, worked_on=?, what_happened=?, insight=?, next_up=?,
		blocker=?, blocker_decision=?, updated_at=? WHERE id=?`,
		nullInt(l.SprintID), l.LogDate, l.WorkedOn, l.WhatHappened, l.Insight, l.NextUp,
		l.Blocker, bd, nowUTC(), l.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func collectLogs(rows *sql.Rows) ([]domain.DailyLog, error) {
	var out []domain.DailyLog
	for rows.Next() {
		l, err := scanLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// DeleteLog removes a daily log by ID.
func (s *Store) DeleteLog(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM daily_logs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
