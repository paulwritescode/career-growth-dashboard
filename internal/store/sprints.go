package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

const sprintCols = `id, month_label, skill_name, skill_rationale, microapp_one_liner,
	core_feature, out_of_scope, deploy_platform, current_phase, status, live_url,
	declaration_post_id, retro_worked, retro_differently, retro_learned, retro_live_link,
	started_on, ended_on, created_at, updated_at`

func scanSprint(row interface{ Scan(...any) error }) (domain.Sprint, error) {
	var s domain.Sprint
	var (
		liveURL, startedOn, endedOn sql.NullString
		declPostID                  sql.NullInt64
		createdAt, updatedAt        string
	)
	if err := row.Scan(
		&s.ID, &s.MonthLabel, &s.SkillName, &s.SkillRationale, &s.MicroappOneLiner,
		&s.CoreFeature, &s.OutOfScope, &s.DeployPlatform, &s.CurrentPhase, &s.Status, &liveURL,
		&declPostID, &s.RetroWorked, &s.RetroDifferently, &s.RetroLearned, &s.RetroLiveLink,
		&startedOn, &endedOn, &createdAt, &updatedAt,
	); err != nil {
		return domain.Sprint{}, err
	}
	s.LiveURL = strFromNull(liveURL)
	s.DeclarationPostID = intFromNull(declPostID)
	s.StartedOn = dateStrFromNull(startedOn)
	s.EndedOn = dateStrFromNull(endedOn)
	s.CreatedAt = parseTime(createdAt)
	s.UpdatedAt = parseTime(updatedAt)
	return s, nil
}

// CreateSprint inserts a new sprint and returns it with its assigned ID.
func (s *Store) CreateSprint(ctx context.Context, in domain.Sprint) (domain.Sprint, error) {
	now := nowUTC()
	res, err := s.db.ExecContext(ctx, `INSERT INTO sprints
		(month_label, skill_name, skill_rationale, microapp_one_liner, core_feature,
		 out_of_scope, deploy_platform, current_phase, status, live_url,
		 retro_worked, retro_differently, retro_learned, retro_live_link,
		 started_on, ended_on, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.MonthLabel, in.SkillName, in.SkillRationale, in.MicroappOneLiner, in.CoreFeature,
		in.OutOfScope, in.DeployPlatform, in.CurrentPhase, in.Status, nullStr(in.LiveURL),
		in.RetroWorked, in.RetroDifferently, in.RetroLearned, in.RetroLiveLink,
		nullStr(in.StartedOn), nullStr(in.EndedOn), now, now)
	if err != nil {
		return domain.Sprint{}, fmt.Errorf("insert sprint: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Sprint{}, err
	}
	return s.GetSprint(ctx, id)
}

// GetSprint returns a sprint by ID, or ErrNotFound.
func (s *Store) GetSprint(ctx context.Context, id int64) (domain.Sprint, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+sprintCols+` FROM sprints WHERE id = ?`, id)
	sp, err := scanSprint(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Sprint{}, ErrNotFound
	}
	return sp, err
}

// GetCurrentSprint returns the single active sprint, or ErrNotFound if none.
func (s *Store) GetCurrentSprint(ctx context.Context) (domain.Sprint, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+sprintCols+` FROM sprints WHERE status = 'active' ORDER BY id DESC LIMIT 1`)
	sp, err := scanSprint(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Sprint{}, ErrNotFound
	}
	return sp, err
}

// ListSprints returns all sprints, newest first.
func (s *Store) ListSprints(ctx context.Context) ([]domain.Sprint, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+sprintCols+` FROM sprints ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Sprint
	for rows.Next() {
		sp, err := scanSprint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// UpdateSprint persists all mutable fields of a sprint.
func (s *Store) UpdateSprint(ctx context.Context, sp domain.Sprint) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sprints SET
		month_label=?, skill_name=?, skill_rationale=?, microapp_one_liner=?, core_feature=?,
		out_of_scope=?, deploy_platform=?, current_phase=?, status=?, live_url=?,
		declaration_post_id=?, retro_worked=?, retro_differently=?, retro_learned=?, retro_live_link=?,
		started_on=?, ended_on=?, updated_at=? WHERE id=?`,
		sp.MonthLabel, sp.SkillName, sp.SkillRationale, sp.MicroappOneLiner, sp.CoreFeature,
		sp.OutOfScope, sp.DeployPlatform, sp.CurrentPhase, sp.Status, nullStr(sp.LiveURL),
		nullInt(sp.DeclarationPostID), sp.RetroWorked, sp.RetroDifferently, sp.RetroLearned, sp.RetroLiveLink,
		nullStr(sp.StartedOn), nullStr(sp.EndedOn), nowUTC(), sp.ID)
	return err
}

// --- checklist items ------------------------------------------------------

// SeedChecklist inserts checklist items for a sprint in a single transaction.
func (s *Store) SeedChecklist(ctx context.Context, sprintID int64, items []domain.ChecklistItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	now := nowUTC()
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO checklist_items
			(sprint_id, phase, label, is_done, sort_order, created_at) VALUES (?,?,?,?,?,?)`,
			sprintID, it.Phase, it.Label, b2i(it.IsDone), it.SortOrder, now); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ListChecklist returns the checklist items for a sprint, ordered by phase then sort_order.
func (s *Store) ListChecklist(ctx context.Context, sprintID int64) ([]domain.ChecklistItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, sprint_id, phase, label, is_done, sort_order, done_at, created_at
		 FROM checklist_items WHERE sprint_id = ? ORDER BY phase, sort_order, id`, sprintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ChecklistItem
	for rows.Next() {
		var it domain.ChecklistItem
		var isDone int
		var doneAt, createdAt sql.NullString
		if err := rows.Scan(&it.ID, &it.SprintID, &it.Phase, &it.Label, &isDone,
			&it.SortOrder, &doneAt, &createdAt); err != nil {
			return nil, err
		}
		it.IsDone = isDone == 1
		it.DoneAt = timeFromNull(doneAt)
		it.CreatedAt = parseTime(createdAt.String)
		out = append(out, it)
	}
	return out, rows.Err()
}

// ToggleChecklistItem sets the done state of a checklist item.
func (s *Store) ToggleChecklistItem(ctx context.Context, id int64, done bool) error {
	var doneAt any
	if done {
		doneAt = nowUTC()
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE checklist_items SET is_done=?, done_at=? WHERE id=?`, b2i(done), doneAt, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// DeleteSprint removes a sprint and its associated checklist items.
func (s *Store) DeleteSprint(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM checklist_items WHERE sprint_id = ?`, id); err != nil {
		_ = tx.Rollback()
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM sprints WHERE id = ?`, id)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		_ = tx.Rollback()
		return ErrNotFound
	}
	return tx.Commit()
}
