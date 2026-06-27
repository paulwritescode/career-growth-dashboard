package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

const adrCols = `id, sprint_id, number, title, status, decided_on, problem, options,
	decision, why, consequences, created_at, updated_at`

func scanADR(row interface{ Scan(...any) error }) (domain.ADR, error) {
	var a domain.ADR
	var (
		sprintID             sql.NullInt64
		decidedOn            sql.NullString
		createdAt, updatedAt string
	)
	if err := row.Scan(&a.ID, &sprintID, &a.Number, &a.Title, &a.Status, &decidedOn,
		&a.Problem, &a.Options, &a.Decision, &a.Why, &a.Consequences, &createdAt, &updatedAt); err != nil {
		return domain.ADR{}, err
	}
	a.SprintID = intFromNull(sprintID)
	a.DecidedOn = dateStrFromNull(decidedOn)
	a.CreatedAt = parseTime(createdAt)
	a.UpdatedAt = parseTime(updatedAt)
	return a, nil
}

// CreateADR inserts an ADR and returns it with its ID.
func (s *Store) CreateADR(ctx context.Context, in domain.ADR) (domain.ADR, error) {
	now := nowUTC()
	res, err := s.db.ExecContext(ctx, `INSERT INTO adrs
		(sprint_id, number, title, status, decided_on, problem, options, decision, why, consequences, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		nullInt(in.SprintID), in.Number, in.Title, in.Status, nullStr(in.DecidedOn),
		in.Problem, in.Options, in.Decision, in.Why, in.Consequences, now, now)
	if err != nil {
		return domain.ADR{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.ADR{}, err
	}
	return s.GetADR(ctx, id)
}

// GetADR returns an ADR by ID, or ErrNotFound.
func (s *Store) GetADR(ctx context.Context, id int64) (domain.ADR, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+adrCols+` FROM adrs WHERE id = ?`, id)
	a, err := scanADR(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ADR{}, ErrNotFound
	}
	return a, err
}

// ListADRs returns all ADRs, newest first.
func (s *Store) ListADRs(ctx context.Context) ([]domain.ADR, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+adrCols+` FROM adrs ORDER BY number DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ADR
	for rows.Next() {
		a, err := scanADR(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// MaxADRNumber returns the highest ADR number, or 0 if none exist.
func (s *Store) MaxADRNumber(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(number), 0) FROM adrs`).Scan(&n)
	return n, err
}

// UpdateADR persists all mutable fields of an ADR.
func (s *Store) UpdateADR(ctx context.Context, a domain.ADR) error {
	res, err := s.db.ExecContext(ctx, `UPDATE adrs SET
		sprint_id=?, number=?, title=?, status=?, decided_on=?, problem=?, options=?,
		decision=?, why=?, consequences=?, updated_at=? WHERE id=?`,
		nullInt(a.SprintID), a.Number, a.Title, a.Status, nullStr(a.DecidedOn),
		a.Problem, a.Options, a.Decision, a.Why, a.Consequences, nowUTC(), a.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteADR removes an ADR by ID.
func (s *Store) DeleteADR(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM adrs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- career events --------------------------------------------------------

// AppendEvent inserts a career event (append-only audit/log stream).
func (s *Store) AppendEvent(ctx context.Context, e domain.CareerEvent) error {
	occurred := e.OccurredAt
	if occurred.IsZero() {
		occurred = parseTime(nowUTC())
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO career_events
		(occurred_at, kind, source, sprint_id, post_id, summary, detail) VALUES (?,?,?,?,?,?,?)`,
		occurred.UTC().Format(timeLayout), e.Kind, e.Source,
		nullInt(e.SprintID), nullInt(e.PostID), e.Summary, e.Detail)
	return err
}

// ListEvents returns recent career events, newest first, up to limit.
func (s *Store) ListEvents(ctx context.Context, limit int) ([]domain.CareerEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, occurred_at, kind, source, sprint_id, post_id, summary, detail
		 FROM career_events ORDER BY occurred_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CareerEvent
	for rows.Next() {
		var e domain.CareerEvent
		var sprintID, postID sql.NullInt64
		var occurredAt string
		if err := rows.Scan(&e.ID, &occurredAt, &e.Kind, &e.Source, &sprintID,
			&postID, &e.Summary, &e.Detail); err != nil {
			return nil, err
		}
		e.OccurredAt = parseTime(occurredAt)
		e.SprintID = intFromNull(sprintID)
		e.PostID = intFromNull(postID)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListEventsBySprint returns all career events for a sprint, oldest first. Used
// to reconstruct the sprint's phase timeline for the trace view.
func (s *Store) ListEventsBySprint(ctx context.Context, sprintID int64) ([]domain.CareerEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, occurred_at, kind, source, sprint_id, post_id, summary, detail
		 FROM career_events WHERE sprint_id = ? ORDER BY occurred_at ASC, id ASC`, sprintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CareerEvent
	for rows.Next() {
		var e domain.CareerEvent
		var sprintID, postID sql.NullInt64
		var occurredAt string
		if err := rows.Scan(&e.ID, &occurredAt, &e.Kind, &e.Source, &sprintID,
			&postID, &e.Summary, &e.Detail); err != nil {
			return nil, err
		}
		e.OccurredAt = parseTime(occurredAt)
		e.SprintID = intFromNull(sprintID)
		e.PostID = intFromNull(postID)
		out = append(out, e)
	}
	return out, rows.Err()
}
