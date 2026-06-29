#!/usr/bin/env bash
#
# dev.sh — unified development script for local-scava.
#
# Modes:
#   ./dev.sh              # build + run (one-shot, like the old run.sh)
#   ./dev.sh watch        # live-reload via air (auto-restarts on file changes)
#   ./dev.sh fresh        # delete the DB and start clean (forces /setup flow)
#   ./dev.sh migrate      # apply pending migrations and exit
#   ./dev.sh status       # show migration status
#   ./dev.sh down         # roll back the last migration
#   ./dev.sh build        # just build the binary, don't run
#
# Any extra flags are forwarded to the binary:
#   ./dev.sh --log-level debug --addr 127.0.0.1:5500
#   ./dev.sh watch --log-level debug
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

BIN_DIR="$ROOT/bin"
BIN="$BIN_DIR/local-scava"
PKG="./cmd/local-scava"
DEFAULT_DB="$HOME/.local/share/local-scava/scava.db"

# Stamp the build with a version (git short sha if available, else "dev").
VERSION="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo dev)"

# --- Helper functions ---

build() {
  echo ">> building local-scava (version=$VERSION)"
  mkdir -p "$BIN_DIR"
  go build -ldflags "-X main.version=$VERSION" -o "$BIN" "$PKG"
}

run_server() {
  echo ">> starting local-scava — open http://127.0.0.1:3000"
  echo ">> press Ctrl-C to stop (graceful shutdown)"
  exec "$BIN" "$@"
}

ensure_air() {
  export PATH="${GOPATH:-$HOME/go}/bin:$PATH"
  if ! command -v air &>/dev/null; then
    echo ">> air not found — installing..."
    go install github.com/air-verse/air@latest
  fi
}

# --- Main ---

MODE="${1:-run}"

case "$MODE" in
  watch)
    shift
    ensure_air
    echo ">> starting local-scava with live-reload (air)"
    echo ">> edit any .go, .html, .css, or .js file and the server will restart"
    exec air -- "$@"
    ;;

  fresh)
    shift || true
    echo ">> removing old database at $DEFAULT_DB"
    rm -f "$DEFAULT_DB" "${DEFAULT_DB}-wal" "${DEFAULT_DB}-shm"
    echo ">> database cleared — will run /setup on next start"
    build
    run_server "$@"
    ;;

  migrate)
    shift || true
    build
    exec "$BIN" --migrate-only "$@"
    ;;

  status)
    shift || true
    build
    exec "$BIN" --migrate-status "$@"
    ;;

  down)
    shift || true
    build
    exec "$BIN" --migrate-down "$@"
    ;;

  build)
    build
    echo ">> binary at $BIN"
    ;;

  run|*)
    # If first arg is not a known mode, treat everything as flags to the binary.
    if [[ "$MODE" != "run" ]]; then
      # Not a mode — pass it as a flag.
      set -- "$MODE" "${@:2}"
    else
      shift || true
    fi
    build
    run_server "$@"
    ;;
esac
