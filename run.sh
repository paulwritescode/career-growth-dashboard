#!/usr/bin/env bash
#
# run.sh — build and start the local-scava daemon.
#
# Usage:
#   ./run.sh                 # build, then run on http://127.0.0.1:5500
#   ./run.sh --addr 127.0.0.1:5600
#   ./run.sh --db /tmp/scava.db --log-level debug
#   ./run.sh --migrate-only  # apply migrations and exit (no server)
#
# Any flags you pass are forwarded straight to the binary. See `./run.sh --help`.
#
set -euo pipefail

# Resolve the repo root (the directory this script lives in) so it works from
# anywhere.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

BIN_DIR="$ROOT/bin"
BIN="$BIN_DIR/local-scava"
PKG="./cmd/local-scava"

# Stamp the build with a version (git short sha if available, else "dev").
VERSION="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo dev)"

echo ">> building local-scava (version=$VERSION)"
mkdir -p "$BIN_DIR"
go build -ldflags "-X main.version=$VERSION" -o "$BIN" "$PKG"

echo ">> starting local-scava — open http://127.0.0.1:5500 once you see 'dashboard ready'"
echo ">> press Ctrl-C to stop (graceful shutdown)"
exec "$BIN" "$@"
