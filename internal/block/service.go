package block

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sentinel errors.
var (
	ErrUnknownBlock = errors.New("unknown block key")
)

// DB is the interface this package needs from the store layer.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Service manages per-user block enable/disable state.
type Service struct {
	db  DB
	now func() time.Time
}

// New creates a block service.
func New(db DB) *Service {
	return &Service{db: db, now: time.Now}
}

// Enabled returns the list of block definitions that are enabled for a user.
func (s *Service) Enabled(ctx context.Context, userID int64) ([]Def, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT block_key FROM user_blocks WHERE user_id = ? AND enabled = 1`, userID)
	if err != nil {
		return nil, fmt.Errorf("block: query enabled: %w", err)
	}
	defer rows.Close()

	enabledKeys := make(map[string]bool)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		enabledKeys[key] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var defs []Def
	for _, d := range registry {
		if enabledKeys[d.Key] {
			defs = append(defs, d)
		}
	}
	return defs, nil
}

// IsEnabled checks whether a specific block is enabled for a user.
func (s *Service) IsEnabled(ctx context.Context, userID int64, key string) (bool, error) {
	if _, ok := ByKey(key); !ok {
		return false, ErrUnknownBlock
	}
	var enabled int
	err := s.db.QueryRowContext(ctx,
		`SELECT enabled FROM user_blocks WHERE user_id = ? AND block_key = ?`,
		userID, key).Scan(&enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("block: check enabled: %w", err)
	}
	return enabled == 1, nil
}

// Enable activates a block for a user.
func (s *Service) Enable(ctx context.Context, userID int64, key string) error {
	if _, ok := ByKey(key); !ok {
		return ErrUnknownBlock
	}
	now := s.now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_blocks (user_id, block_key, enabled, updated_at) VALUES (?, ?, 1, ?)
		 ON CONFLICT(user_id, block_key) DO UPDATE SET enabled = 1, updated_at = ?`,
		userID, key, now, now)
	return err
}

// Disable deactivates a block for a user (data is preserved).
func (s *Service) Disable(ctx context.Context, userID int64, key string) error {
	if _, ok := ByKey(key); !ok {
		return ErrUnknownBlock
	}
	now := s.now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_blocks (user_id, block_key, enabled, updated_at) VALUES (?, ?, 0, ?)
		 ON CONFLICT(user_id, block_key) DO UPDATE SET enabled = 0, updated_at = ?`,
		userID, key, now, now)
	return err
}

// Toggle flips the enabled state of a block. Returns the new enabled state.
func (s *Service) Toggle(ctx context.Context, userID int64, key string) (bool, error) {
	enabled, err := s.IsEnabled(ctx, userID, key)
	if err != nil {
		return false, err
	}
	if enabled {
		return false, s.Disable(ctx, userID, key)
	}
	return true, s.Enable(ctx, userID, key)
}

// SetBlocks sets the enabled blocks for a user (used during onboarding).
// All registry keys not in the list are set to disabled.
func (s *Service) SetBlocks(ctx context.Context, userID int64, keys []string) error {
	now := s.now().UTC().Format(time.RFC3339)
	enabledSet := make(map[string]bool, len(keys))
	for _, k := range keys {
		enabledSet[k] = true
	}
	for _, d := range registry {
		enabled := 0
		if enabledSet[d.Key] {
			enabled = 1
		}
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO user_blocks (user_id, block_key, enabled, updated_at) VALUES (?, ?, ?, ?)
			 ON CONFLICT(user_id, block_key) DO UPDATE SET enabled = ?, updated_at = ?`,
			userID, d.Key, enabled, now, enabled, now)
		if err != nil {
			return fmt.Errorf("block: set %s: %w", d.Key, err)
		}
	}
	return nil
}

// IsRouteEnabled checks if the route belongs to an enabled block.
// Returns (blockKey, enabled, error). If the route is not block-owned, returns ("", true, nil).
func (s *Service) IsRouteEnabled(ctx context.Context, userID int64, path string) (string, bool, error) {
	for _, d := range registry {
		for _, route := range d.Routes {
			if pathMatchesBlock(path, route) {
				enabled, err := s.IsEnabled(ctx, userID, d.Key)
				return d.Key, enabled, err
			}
		}
	}
	return "", true, nil // not block-owned
}

// pathMatchesBlock checks if a request path belongs to a block's route pattern.
func pathMatchesBlock(path, pattern string) bool {
	// Pattern ending with "/" means prefix match (e.g. "/sprints/" matches "/sprints/123").
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern) || path == strings.TrimSuffix(pattern, "/")
	}
	return path == pattern
}

// Metrics returns a map of block_key → human-readable metric string.
func (s *Service) Metrics(ctx context.Context, userID int64) (map[string]string, error) {
	metrics := make(map[string]string)

	// Sprint count.
	var sprintCount, activeCount int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sprints`).Scan(&sprintCount)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sprints WHERE status = 'active'`).Scan(&activeCount)
	if sprintCount > 0 {
		metrics["sprint"] = fmt.Sprintf("%d sprints · %d active", sprintCount, activeCount)
	}

	// ADR count.
	var adrCount int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM adrs`).Scan(&adrCount)
	if adrCount > 0 {
		metrics["adr"] = fmt.Sprintf("%d ADRs", adrCount)
	}

	// Log count.
	var logCount int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM daily_logs`).Scan(&logCount)
	if logCount > 0 {
		metrics["logs"] = fmt.Sprintf("%d log entries", logCount)
	}

	// Post count.
	var postCount, publishedCount int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM posts`).Scan(&postCount)
	s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT post_id) FROM post_tiers WHERE status = 'published'`).Scan(&publishedCount)
	if postCount > 0 {
		metrics["posts"] = fmt.Sprintf("%d posts · %d published", postCount, publishedCount)
	}

	// Todo count.
	var todoOpen, todoDone int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM todos WHERE status = 'open'`).Scan(&todoOpen)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM todos WHERE status = 'done'`).Scan(&todoDone)
	if todoOpen+todoDone > 0 {
		metrics["todo"] = fmt.Sprintf("%d open · %d done", todoOpen, todoDone)
	}

	return metrics, nil
}
