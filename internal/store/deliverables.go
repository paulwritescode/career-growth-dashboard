package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// CreateDeliverable inserts a new deliverable for a sprint.
func (s *Store) CreateDeliverable(ctx context.Context, sprintID int64, text string) (domain.Deliverable, error) {
	now := nowUTC()
	// Get next sort_order.
	var maxOrder int
	_ = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sort_order), -1) FROM deliverables WHERE sprint_id = ?`, sprintID).Scan(&maxOrder)

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO deliverables (sprint_id, text, is_done, sort_order, created_at)
		 VALUES (?, ?, 0, ?, ?)`, sprintID, text, maxOrder+1, now)
	if err != nil {
		return domain.Deliverable{}, err
	}
	id, _ := res.LastInsertId()
	return domain.Deliverable{
		ID:        id,
		SprintID:  sprintID,
		Text:      text,
		IsDone:    false,
		SortOrder: maxOrder + 1,
		CreatedAt: parseTime(now),
	}, nil
}

// ListDeliverables returns all deliverables for a sprint, ordered by sort_order.
func (s *Store) ListDeliverables(ctx context.Context, sprintID int64) ([]domain.Deliverable, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, sprint_id, text, is_done, done_at, sort_order, created_at
		 FROM deliverables WHERE sprint_id = ? ORDER BY sort_order, id`, sprintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Deliverable
	for rows.Next() {
		var d domain.Deliverable
		var isDone int
		var doneAt, createdAt sql.NullString
		if err := rows.Scan(&d.ID, &d.SprintID, &d.Text, &isDone, &doneAt, &d.SortOrder, &createdAt); err != nil {
			return nil, err
		}
		d.IsDone = isDone == 1
		d.DoneAt = timeFromNull(doneAt)
		d.CreatedAt = parseTime(createdAt.String)
		out = append(out, d)
	}
	return out, rows.Err()
}

// ToggleDeliverable flips the done state of a deliverable.
func (s *Store) ToggleDeliverable(ctx context.Context, id int64) (domain.Deliverable, error) {
	var isDone int
	err := s.db.QueryRowContext(ctx,
		`SELECT is_done FROM deliverables WHERE id = ?`, id).Scan(&isDone)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Deliverable{}, ErrNotFound
		}
		return domain.Deliverable{}, err
	}

	newDone := 1 - isDone
	var doneAt any
	if newDone == 1 {
		doneAt = nowUTC()
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE deliverables SET is_done = ?, done_at = ? WHERE id = ?`, newDone, doneAt, id)
	if err != nil {
		return domain.Deliverable{}, err
	}

	return s.GetDeliverable(ctx, id)
}

// GetDeliverable returns a single deliverable by ID.
func (s *Store) GetDeliverable(ctx context.Context, id int64) (domain.Deliverable, error) {
	var d domain.Deliverable
	var isDone int
	var doneAt, createdAt sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, sprint_id, text, is_done, done_at, sort_order, created_at
		 FROM deliverables WHERE id = ?`, id).
		Scan(&d.ID, &d.SprintID, &d.Text, &isDone, &doneAt, &d.SortOrder, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Deliverable{}, ErrNotFound
		}
		return domain.Deliverable{}, err
	}
	d.IsDone = isDone == 1
	d.DoneAt = timeFromNull(doneAt)
	d.CreatedAt = parseTime(createdAt.String)
	return d, nil
}

// DeleteDeliverable removes a deliverable.
func (s *Store) DeleteDeliverable(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM deliverables WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountDeliverables returns done and total counts for a sprint.
func (s *Store) CountDeliverables(ctx context.Context, sprintID int64) (done, total int, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(is_done), 0), COUNT(*) FROM deliverables WHERE sprint_id = ?`, sprintID).
		Scan(&done, &total)
	return
}
