// Package onboarding handles the three-step first-run wizard: role → blocks → confirm.
package onboarding

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// Sentinel errors.
var (
	ErrOnboardingComplete = errors.New("onboarding already completed")
	ErrNoBlocksSelected   = errors.New("select at least one block")
)

// DB is the interface this package needs from the store layer.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Service manages onboarding state.
type Service struct {
	db  DB
	now func() time.Time
}

// New creates an onboarding service.
func New(db DB) *Service {
	return &Service{db: db, now: time.Now}
}

// State returns the current onboarding state for a user.
func (s *Service) State(ctx context.Context, userID int64) (domain.OnboardingState, error) {
	var state domain.OnboardingState
	var updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, status, updated_at FROM onboarding_state WHERE user_id = ?`, userID).
		Scan(&state.UserID, &state.Status, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Create initial state.
			now := s.now().UTC().Format(time.RFC3339)
			_, err := s.db.ExecContext(ctx,
				`INSERT INTO onboarding_state (user_id, status, updated_at) VALUES (?, 'pending', ?)`,
				userID, now)
			if err != nil {
				return domain.OnboardingState{}, fmt.Errorf("onboarding: create state: %w", err)
			}
			return domain.OnboardingState{
				UserID:    userID,
				Status:    domain.OnboardingPending,
				UpdatedAt: s.now().UTC(),
			}, nil
		}
		return domain.OnboardingState{}, fmt.Errorf("onboarding: query state: %w", err)
	}
	state.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return state, nil
}

// IsComplete reports whether onboarding is done for the user.
func (s *Service) IsComplete(ctx context.Context, userID int64) (bool, error) {
	state, err := s.State(ctx, userID)
	if err != nil {
		return false, err
	}
	return state.Status == domain.OnboardingComplete, nil
}

// SetRole advances the wizard from pending to role_done.
func (s *Service) SetRole(ctx context.Context, userID int64) error {
	now := s.now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE onboarding_state SET status = 'role_done', updated_at = ? WHERE user_id = ?`,
		now, userID)
	return err
}

// SetBlocks advances the wizard from role_done to blocks_done.
func (s *Service) SetBlocks(ctx context.Context, userID int64) error {
	now := s.now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE onboarding_state SET status = 'blocks_done', updated_at = ? WHERE user_id = ?`,
		now, userID)
	return err
}

// Complete marks onboarding as complete.
func (s *Service) Complete(ctx context.Context, userID int64) error {
	state, err := s.State(ctx, userID)
	if err != nil {
		return err
	}
	if state.Status == domain.OnboardingComplete {
		return ErrOnboardingComplete
	}
	now := s.now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx,
		`UPDATE onboarding_state SET status = 'complete', updated_at = ? WHERE user_id = ?`,
		now, userID)
	return err
}

// SetPlatforms stores the user's selected post platforms.
func (s *Service) SetPlatforms(ctx context.Context, userID int64, platforms []string) error {
	// Clear existing.
	_, _ = s.db.ExecContext(ctx, `DELETE FROM user_platforms WHERE user_id = ?`, userID)
	for _, p := range platforms {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO user_platforms (user_id, platform) VALUES (?, ?)`, userID, p)
		if err != nil {
			return fmt.Errorf("onboarding: set platform %s: %w", p, err)
		}
	}
	return nil
}

// GetPlatforms returns the user's selected platforms.
func (s *Service) GetPlatforms(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT platform FROM user_platforms WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var platforms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		platforms = append(platforms, p)
	}
	return platforms, rows.Err()
}

// CurrentStep returns the route path for the current onboarding step.
func (s *Service) CurrentStep(ctx context.Context, userID int64) (string, error) {
	state, err := s.State(ctx, userID)
	if err != nil {
		return "", err
	}
	switch state.Status {
	case domain.OnboardingPending:
		return "/onboarding/role", nil
	case domain.OnboardingRoleDone:
		return "/onboarding/blocks", nil
	case domain.OnboardingBlocksDone:
		return "/onboarding/confirm", nil
	default:
		return "/", nil
	}
}

// RequireOnboardingComplete is middleware that redirects to the current
// onboarding step if the wizard is incomplete.
func (s *Service) RequireOnboardingComplete(getUserID func(r *http.Request) (int64, bool)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := getUserID(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			complete, err := s.IsComplete(r.Context(), userID)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if !complete {
				step, _ := s.CurrentStep(r.Context(), userID)
				if step != "" && r.URL.Path != step {
					http.Redirect(w, r, step, http.StatusSeeOther)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
