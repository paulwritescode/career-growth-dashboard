package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// CreateTodo inserts a new todo.
func (s *Store) CreateTodo(ctx context.Context, t domain.Todo) (domain.Todo, error) {
	now := nowUTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO todos (user_id, sprint_id, text, priority, status, due_on, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.UserID, nullInt(t.SprintID), t.Text, t.Priority, t.Status, nullStr(t.DueOn), now)
	if err != nil {
		return domain.Todo{}, err
	}
	id, _ := res.LastInsertId()
	t.ID = id
	t.CreatedAt = parseTime(now)
	return t, nil
}

// ListTodos returns todos for a user, filtered by status.
func (s *Store) ListTodos(ctx context.Context, userID int64, status string) ([]domain.Todo, error) {
	query := `SELECT id, user_id, sprint_id, text, priority, status, due_on, created_at, done_at
	           FROM todos WHERE user_id = ?`
	args := []any{userID}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY CASE priority WHEN 'high' THEN 0 WHEN 'normal' THEN 1 ELSE 2 END, created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Todo
	for rows.Next() {
		var t domain.Todo
		var sprintID sql.NullInt64
		var dueOn, doneAt sql.NullString
		var createdAt string
		if err := rows.Scan(&t.ID, &t.UserID, &sprintID, &t.Text, &t.Priority, &t.Status, &dueOn, &createdAt, &doneAt); err != nil {
			return nil, err
		}
		t.SprintID = intFromNull(sprintID)
		t.DueOn = strFromNull(dueOn)
		t.DoneAt = timeFromNull(doneAt)
		t.CreatedAt = parseTime(createdAt)
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateTodoStatus changes a todo's status (open/done/dropped).
func (s *Store) UpdateTodoStatus(ctx context.Context, id int64, status domain.TodoStatus) error {
	var doneAt any
	if status == domain.TodoDone {
		doneAt = nowUTC()
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE todos SET status = ?, done_at = ? WHERE id = ?`, status, doneAt, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteTodo removes a todo.
func (s *Store) DeleteTodo(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM todos WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetTodo returns a single todo by ID.
func (s *Store) GetTodo(ctx context.Context, id int64) (domain.Todo, error) {
	var t domain.Todo
	var sprintID sql.NullInt64
	var dueOn, doneAt sql.NullString
	var createdAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, sprint_id, text, priority, status, due_on, created_at, done_at
		 FROM todos WHERE id = ?`, id).
		Scan(&t.ID, &t.UserID, &sprintID, &t.Text, &t.Priority, &t.Status, &dueOn, &createdAt, &doneAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Todo{}, ErrNotFound
		}
		return domain.Todo{}, err
	}
	t.SprintID = intFromNull(sprintID)
	t.DueOn = strFromNull(dueOn)
	t.DoneAt = timeFromNull(doneAt)
	t.CreatedAt = parseTime(createdAt)
	return t, nil
}
