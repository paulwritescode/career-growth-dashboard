package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/paulkinyatti/local-scava/internal/auth"
	"github.com/paulkinyatti/local-scava/internal/block"
	"github.com/paulkinyatti/local-scava/internal/bridge"
	"github.com/paulkinyatti/local-scava/internal/export"
	"github.com/paulkinyatti/local-scava/internal/onboarding"
	"github.com/paulkinyatti/local-scava/internal/service"
	"github.com/paulkinyatti/local-scava/internal/store"
	"github.com/paulkinyatti/local-scava/internal/web"
)

// App is the assembled daemon: config, logger, store, service, and HTTP server.
type App struct {
	cfg        Config
	log        *slog.Logger
	store      *store.Store
	svc        *service.Service
	auth       *auth.Service
	block      *block.Service
	onboarding *onboarding.Service
	web        *web.Handlers
	bridge     *bridge.Bridge
	server     *http.Server
}

// New constructs an App: sets up logging, opens the store (running migrations),
// builds the service, and prepares the HTTP server. The caller must call Run.
func New(ctx context.Context, cfg Config) (*App, error) {
	log := newLogger(cfg)

	if err := ensureDBDir(cfg.DBPath); err != nil {
		return nil, err
	}

	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return nil, err
	}
	// Security (spec 10): the DB file holds the user's career data and must be
	// user-only readable/writable. Tighten perms after the driver creates it.
	if err := os.Chmod(cfg.DBPath, 0o600); err != nil && !os.IsNotExist(err) {
		log.Warn("could not set db file permissions", "db", cfg.DBPath, "err", err)
	}
	log.Info("migrations applied", "db", cfg.DBPath)

	svc := service.New(st)

	authSvc := auth.New(st.DB())
	blockSvc := block.New(st.DB())
	onboardingSvc := onboarding.New(st.DB())
	exportSvc := export.New(svc, st)

	webHandlers, err := web.New(svc, authSvc, blockSvc, onboardingSvc, log, web.Meta{
		Addr:    cfg.Addr,
		DBPath:  cfg.DBPath,
		KiroBin: cfg.KiroBin,
		Version: cfg.Version,
	})
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	webHandlers.SetExport(exportSvc)

	// Allowed Origin/Host values for the chat bridge: the configured bind addr
	// plus its localhost alias.
	allowedHosts := []string{cfg.Addr}
	if h, p, ok := splitHostPort(cfg.Addr); ok {
		if h == "127.0.0.1" {
			allowedHosts = append(allowedHosts, "localhost:"+p)
		} else if h == "localhost" {
			allowedHosts = append(allowedHosts, "127.0.0.1:"+p)
		}
	}
	if cfg.KiroTrustAll {
		log.Warn("chat agent runs tools WITHOUT confirmation (--kiro-trust-all); the dashboard chat can mutate your system")
	}
	br := bridge.New(cfg.KiroBin, allowedHosts, cfg.KiroTrustAll, svc, log)

	a := &App{cfg: cfg, log: log, store: st, svc: svc, auth: authSvc, block: blockSvc, onboarding: onboardingSvc, web: webHandlers, bridge: br}
	a.server = &http.Server{
		Addr:              cfg.Addr,
		Handler:           a.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return a, nil
}

// Service exposes the service layer (used when wiring the web UI and bridge).
func (a *App) Service() *service.Service { return a.svc }

// Logger exposes the structured logger.
func (a *App) Logger() *slog.Logger { return a.log }

// routes builds the HTTP handler. The auth middleware chain ensures setup comes
// first, then session validation, then onboarding check, then block gating.
func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.HandleFunc("GET /ws", a.bridge.HandleWS)
	a.web.Mount(mux)

	// Wrap with setup redirect (forces /setup when no users exist).
	handler := a.auth.RequireSetup(a.validateHost(mux))
	return handler
}

// validateHost rejects any request whose Host header is not a loopback host
// (spec 10: anti DNS-rebinding). The bind is already loopback-only, but a
// rebinding attack can still reach 127.0.0.1 with an attacker-controlled Host;
// this is the defense for the dashboard routes the way the bridge guards /ws.
func (a *App) validateHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostAllowed(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// hostAllowed reports whether a Host header is a loopback host (with or without
// a port). Anything else is rejected.
func hostAllowed(host string) bool {
	h := host
	if i := strings.LastIndex(host, ":"); i >= 0 {
		h = host[:i]
	}
	h = strings.Trim(h, "[]") // strip IPv6 brackets
	switch h {
	case "127.0.0.1", "localhost", "::1", "":
		return true
	default:
		return strings.HasPrefix(h, "127.")
	}
}

// handleHealthz reports daemon and database health. Loopback-only, no auth.
func (a *App) handleHealthz(w http.ResponseWriter, r *http.Request) {
	status := map[string]string{"status": "ok", "db": "ok"}
	code := http.StatusOK
	if err := a.store.Ping(r.Context()); err != nil {
		status["status"] = "degraded"
		status["db"] = "error"
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(status)
}

// Run starts the HTTP server and blocks until ctx is cancelled, then shuts down
// gracefully (drains the server, closes the database).
func (a *App) Run(ctx context.Context) error {
	defer func() { _ = a.store.Close() }()

	errCh := make(chan error, 1)
	go func() {
		a.log.Info("dashboard ready", "url", "http://"+a.cfg.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		a.log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.log.Error("graceful shutdown failed", "err", err)
		return err
	}
	a.log.Info("graceful shutdown done", "duration", time.Since(start).String())
	return nil
}

// newLogger builds a slog logger from the config.
func newLogger(cfg Config) *slog.Logger {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if cfg.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(h)
}

// splitHostPort splits "host:port"; ok is false if it cannot be parsed.
func splitHostPort(addr string) (host, port string, ok bool) {
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", false
	}
	return h, p, true
}

// ensureDBDir creates the parent directory of the database file with
// user-only permissions (0700), per the security spec.
func ensureDBDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}

// MigrateOnly opens the store (running migrations) and closes it. Used by the
// --migrate-only flag.
func MigrateOnly(ctx context.Context, cfg Config) error {
	if err := ensureDBDir(cfg.DBPath); err != nil {
		return err
	}
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	v, _ := st.MigrateVersion(ctx)
	fmt.Fprintf(os.Stdout, "migrations applied, current version: %d\n", v)
	return st.Close()
}

// MigrateStatusCmd prints the status of all migrations.
func MigrateStatusCmd(ctx context.Context, cfg Config) error {
	if err := ensureDBDir(cfg.DBPath); err != nil {
		return err
	}
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	return st.MigrateStatus(ctx)
}

// MigrateDownCmd rolls back the last migration.
func MigrateDownCmd(ctx context.Context, cfg Config) error {
	if err := ensureDBDir(cfg.DBPath); err != nil {
		return err
	}
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.MigrateDown(ctx); err != nil {
		return err
	}
	v, _ := st.MigrateVersion(ctx)
	fmt.Fprintf(os.Stdout, "rolled back, current version: %d\n", v)
	return nil
}
