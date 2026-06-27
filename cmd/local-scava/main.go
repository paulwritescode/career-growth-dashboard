// Command local-scava is a local-first daemon that tracks the career-growth
// routine (Monthly Skill Sprint + Three-Tier Content Cadence) and serves a
// monochrome, SRE-style dashboard at http://localhost:5500.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/paulkinyatti/local-scava/internal/app"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "0.1.0-dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "local-scava:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := app.Load(args)
	if err != nil {
		return err
	}
	cfg.Version = version

	// Root context cancelled on SIGINT/SIGTERM; everything derives from it so a
	// single signal cascades a graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.MigrateOnly {
		return app.MigrateOnly(ctx, cfg)
	}

	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	a.Logger().Info("starting local-scava", "version", version, "addr", cfg.Addr)
	return a.Run(ctx)
}
