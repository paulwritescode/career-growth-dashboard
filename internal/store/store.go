// Package store is the persistence layer for local-scava. It owns all SQL and
// returns domain types; no SQL escapes this package. It uses an embedded libSQL
// database accessed via database/sql with the go-libsql driver.
//
// Migrations are managed by goose (github.com/pressly/goose/v3) using embedded
// SQL files. This gives us rollback, status, dry-run, and Go-migration support
// when needed.
//
// Layer rule: store imports domain and the driver only. Services call store;
// store never calls services or the web layer.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	_ "github.com/tursodatabase/go-libsql"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store wraps the database connection and exposes typed data-access methods.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the libSQL database at path, applies pragmas,
// and runs any pending migrations via goose. The caller must Close the returned Store.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("libsql", "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("open libsql: %w", err)
	}

	// SQLite-family engines are single-writer; cap connections to avoid
	// "database is locked" on the write path.
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, p := range pragmas {
		// Some pragmas (journal_mode, busy_timeout) return a row, which the
		// libSQL driver rejects under Exec. Use Query and discard the result.
		rows, err := db.QueryContext(ctx, p)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
		_ = rows.Close()
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// Ping verifies connectivity (used by /healthz).
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// DB exposes the raw handle for the rare caller that needs it (e.g. metrics
// aggregations). Prefer the typed methods.
func (s *Store) DB() *sql.DB { return s.db }

// migrate runs all pending migrations using goose with embedded SQL files.
func (s *Store) migrate(ctx context.Context) error {
	goose.SetBaseFS(migrationsFS)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}

	// Silence goose's default logging in normal operation.
	goose.SetLogger(goose.NopLogger())

	if err := goose.UpContext(ctx, s.db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// MigrateStatus prints the current migration status (for CLI use).
func (s *Store) MigrateStatus(ctx context.Context) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.StatusContext(ctx, s.db, "migrations")
}

// MigrateDown rolls back the last migration.
func (s *Store) MigrateDown(ctx context.Context) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.DownContext(ctx, s.db, "migrations")
}

// MigrateVersion returns the current schema version.
func (s *Store) MigrateVersion(ctx context.Context) (int64, error) {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return 0, err
	}
	return goose.GetDBVersionContext(ctx, s.db)
}

// --- shared helpers -------------------------------------------------------

const timeLayout = time.RFC3339

// nowUTC returns the current time formatted as RFC3339 UTC text.
func nowUTC() string { return time.Now().UTC().Format(timeLayout) }

// parseTime parses an RFC3339 timestamp; zero time on empty/invalid input.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// nullStr converts a *string to a sql null-aware value for binding.
func nullStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// dateOnly normalizes a date column value to YYYY-MM-DD. The go-libsql driver
// coerces date-like string parameters to RFC3339 (e.g. "2026-06-22" becomes
// "2026-06-22T00:00:00Z") on write, so reads come back with a time component
// that must be stripped for bare-date comparisons.
func dateOnly(s string) string {
	if i := strings.IndexByte(s, 'T'); i >= 0 {
		return s[:i]
	}
	return s
}

// dateStrFromNull converts a nullable date column to a normalized *string.
func dateStrFromNull(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := dateOnly(ns.String)
	return &v
}

// nullInt converts a *int64 to a sql null-aware value for binding.
func nullInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

// strFromNull converts a sql.NullString to a *string.
func strFromNull(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

// intFromNull converts a sql.NullInt64 to a *int64.
func intFromNull(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	v := ni.Int64
	return &v
}

// timeFromNull converts a sql.NullString timestamp to a *time.Time.
func timeFromNull(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t := parseTime(ns.String)
	return &t
}

// envOr returns the environment variable value or fallback (used by CLI subcommands).
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
