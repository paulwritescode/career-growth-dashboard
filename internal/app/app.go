package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/paulkinyatti/local-scava/internal/service"
	"github.com/paulkinyatti/local-scava/internal/store"
	"github.com/paulkinyatti/local-scava/internal/web"
)

// App is the assembled daemon: config, logger, store, service, and HTTP server.
type App struct {
	cfg    Config
	log    *slog.Logger
	store  *store.Store
	svc    *service.Service
	web    *web.Handlers
	server *http.Server
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
	log.Info("migrations applied", "db", cfg.DBPath)

	svc := service.New(st)

	webHandlers, err := web.New(svc, log)
	if err != nil {
		_ = st.Close()
		return nil, err
	}

	a := &App{cfg: cfg, log: log, store: st, svc: svc, web: webHandlers}
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

// routes builds the HTTP handler. The dashboard UI and chat bridge are mounted
// in later layers; v1 of this layer serves health and readiness only.
func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	a.web.Mount(mux)
	return mux
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
	return st.Close()
}
