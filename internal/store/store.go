// Package store is the persistence layer for local-scava. It owns all SQL and
// returns domain types; no SQL escapes this package. It uses an embedded libSQL
// database accessed via database/sql with the go-libsql driver.
//
// Layer rule: store imports domain and the driver only. Services call store;
// store never calls services or the web layer.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/tursodatabase/go-libsql"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store wraps the database connection and exposes typed data-access methods.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the libSQL database at path, applies pragmas,
// and runs any pending migrations. The caller must Close the returned Store.
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

type migration struct {
	version int
	name    string
	sql     string
}

// migrate applies all embedded migrations whose version exceeds the highest
// already-applied version, each inside its own transaction.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var current int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read current version: %w", err)
	}

	migs, err := loadMigrations()
	if err != nil {
		return err
	}

	applied := 0
	for _, m := range migs {
		if m.version <= current {
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", m.name, err)
		}
		for _, stmt := range splitStatements(m.sql) {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply %s: %w", m.name, err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`,
			m.version, nowUTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", m.name, err)
		}
		applied++
	}
	return nil
}

// loadMigrations reads and sorts the embedded migration files. File names must
// start with a zero-padded integer version, e.g. "0001_init.sql".
func loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var migs []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		prefix := strings.SplitN(e.Name(), "_", 2)[0]
		v, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("bad migration version in %q: %w", e.Name(), err)
		}
		b, err := migrationsFS.ReadFile(filepath.Join("migrations", e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", e.Name(), err)
		}
		migs = append(migs, migration{version: v, name: e.Name(), sql: string(b)})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })
	return migs, nil
}

// --- shared helpers -------------------------------------------------------

// splitStatements splits a migration file into individual SQL statements. The
// libSQL driver executes only one statement per Exec call, so multi-statement
// files must be split. Line comments (-- ...) are stripped. This relies on the
// migration SQL containing no semicolons inside string literals (true for our
// DDL-only migrations).
func splitStatements(s string) []string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	var out []string
	for _, part := range strings.Split(b.String(), ";") {
		if stmt := strings.TrimSpace(part); stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}

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
