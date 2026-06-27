// Package app wires together configuration, the store, services, the HTTP
// server, and graceful shutdown into the long-running local-scava daemon.
package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the daemon's runtime configuration. Resolution order is
// command-line flag > environment variable > default.
type Config struct {
	Addr         string // bind address; loopback only in v1
	DBPath       string // libSQL database file
	KiroBin      string // kiro-cli binary for the chat bridge
	KiroTrustAll bool   // allow the chat agent to run tools without confirmation
	LogLevel     string // debug|info|warn|error
	LogFormat    string // text|json
	MigrateOnly  bool   // run migrations then exit
	Version      string // build version, for the settings/status display
}

// envOr returns the environment variable value or a fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// defaultDBPath returns ~/.local/share/local-scava/scava.db, falling back to
// the current directory if the home dir cannot be resolved.
func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "scava.db"
	}
	return filepath.Join(home, ".local", "share", "local-scava", "scava.db")
}

// Load parses flags (and env vars / defaults) from the given argument slice.
func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("local-scava", flag.ContinueOnError)
	cfg := Config{}
	fs.StringVar(&cfg.Addr, "addr", envOr("SCAVA_ADDR", "127.0.0.1:5500"), "bind address (loopback only in v1)")
	fs.StringVar(&cfg.DBPath, "db", envOr("SCAVA_DB", defaultDBPath()), "libSQL database file path")
	fs.StringVar(&cfg.KiroBin, "kiro-bin", envOr("SCAVA_KIRO_BIN", "kiro-cli"), "kiro-cli binary for the chat bridge")
	fs.BoolVar(&cfg.KiroTrustAll, "kiro-trust-all", envOr("SCAVA_KIRO_TRUST_ALL", "") == "1", "let the chat agent run tools without confirmation (riskier)")
	fs.StringVar(&cfg.LogLevel, "log-level", envOr("SCAVA_LOG_LEVEL", "info"), "log level: debug|info|warn|error")
	fs.StringVar(&cfg.LogFormat, "log-format", envOr("SCAVA_LOG_FORMAT", "text"), "log format: text|json")
	fs.BoolVar(&cfg.MigrateOnly, "migrate-only", false, "run migrations then exit")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	return cfg.validate()
}

// validate normalizes and checks the configuration.
func (c Config) validate() (Config, error) {
	c.Addr = strings.TrimSpace(c.Addr)
	if c.Addr == "" {
		return Config{}, fmt.Errorf("addr must not be empty")
	}
	if !isLoopback(c.Addr) {
		// v1 hard-restricts to loopback; exposing the command-executing chat
		// bridge to the network is gated behind future explicit work.
		return Config{}, fmt.Errorf("addr %q is not a loopback address; v1 binds 127.0.0.1/localhost only (see specs/10-security.md)", c.Addr)
	}
	if c.DBPath == "" {
		return Config{}, fmt.Errorf("db path must not be empty")
	}
	return c, nil
}

// isLoopback reports whether the host part of addr is a loopback host.
func isLoopback(addr string) bool {
	host := addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
	}
	host = strings.Trim(host, "[]") // strip IPv6 brackets
	switch host {
	case "127.0.0.1", "localhost", "::1", "":
		return true
	default:
		return strings.HasPrefix(host, "127.")
	}
}
