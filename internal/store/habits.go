package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// CreateHabit inserts a new habit.
func (s *Store) CreateHabit(ctx context.Context, h domain.Habit) (domain.Habit, error) {
	now := nowUTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO habits (user_id, name, icon, sprint_linked, archived, created_at)
		 VALUES (?, ?, ?, ?, 0, ?)`,
		h.UserID, h.Name, nullStr(h.Icon), b2i(h.SprintLinked), now)
	if err != nil {
		return domain.Habit{}, err
	}
	id, _ := res.LastInsertId()
	h.ID = id
	h.CreatedAt = parseTime(now)
	return h, nil
}

// ListHabits returns all non-archived habits for a user.
func (s *Store) ListHabits(ctx context.Context, userID int64) ([]domain.Habit, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, icon, sprint_linked, archived, created_at
		 FROM habits WHERE user_id = ? AND archived = 0 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Habit
	for rows.Next() {
		var h domain.Habit
		var icon sql.NullString
		var sprintLinked, archived int
		var createdAt string
		if err := rows.Scan(&h.ID, &h.UserID, &h.Name, &icon, &sprintLinked, &archived, &createdAt); err != nil {
			return nil, err
		}
		h.Icon = strFromNull(icon)
		h.SprintLinked = sprintLinked == 1
		h.Archived = archived == 1
		h.CreatedAt = parseTime(createdAt)
		out = append(out, h)
	}
	return out, rows.Err()
}

// ToggleHabitEntry marks or unmarks a habit for a given date.
func (s *Store) ToggleHabitEntry(ctx context.Context, habitID int64, date string) (bool, error) {
	// Check if entry exists.
	var exists int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM habit_entries WHERE habit_id = ? AND entry_date = ?`,
		habitID, date).Scan(&exists)
	if err != nil {
		return false, err
	}

	if exists > 0 {
		_, err := s.db.ExecContext(ctx,
			`DELETE FROM habit_entries WHERE habit_id = ? AND entry_date = ?`, habitID, date)
		return false, err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO habit_entries (habit_id, entry_date) VALUES (?, ?)`, habitID, date)
	return true, err
}

// HabitEntries returns the entries for a habit within a date range.
func (s *Store) HabitEntries(ctx context.Context, habitID int64, from, to string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT entry_date FROM habit_entries WHERE habit_id = ? AND entry_date >= ? AND entry_date <= ? ORDER BY entry_date`,
		habitID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, dateOnly(d))
	}
	return dates, rows.Err()
}

// HabitStreak returns the current streak length for a habit (consecutive days ending today or yesterday).
func (s *Store) HabitStreak(ctx context.Context, habitID int64, today string) (int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT entry_date FROM habit_entries WHERE habit_id = ? ORDER BY entry_date DESC LIMIT 90`,
		habitID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0, err
		}
		dates = append(dates, dateOnly(d))
	}
	if len(dates) == 0 {
		return 0, nil
	}

	streak := 0
	// Start from today or yesterday.
	checkDate := today
	if len(dates) > 0 && dates[0] != today {
		// Allow streak to start from yesterday.
		checkDate = dates[0]
	}

	for _, d := range dates {
		if d == checkDate {
			streak++
			// Move to previous day.
			t, _ := time.Parse("2006-01-02", checkDate)
			checkDate = t.AddDate(0, 0, -1).Format("2006-01-02")
		} else {
			break
		}
	}
	return streak, nil
}

// ArchiveHabit soft-deletes a habit.
func (s *Store) ArchiveHabit(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE habits SET archived = 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetHabit returns a single habit by ID.
func (s *Store) GetHabit(ctx context.Context, id int64) (domain.Habit, error) {
	var h domain.Habit
	var icon sql.NullString
	var sprintLinked, archived int
	var createdAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, icon, sprint_linked, archived, created_at
		 FROM habits WHERE id = ?`, id).
		Scan(&h.ID, &h.UserID, &h.Name, &icon, &sprintLinked, &archived, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Habit{}, ErrNotFound
		}
		return domain.Habit{}, err
	}
	h.Icon = strFromNull(icon)
	h.SprintLinked = sprintLinked == 1
	h.Archived = archived == 1
	h.CreatedAt = parseTime(createdAt)
	return h, nil
}
