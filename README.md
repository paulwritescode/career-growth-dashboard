# local-scava

A local-first, single-binary Go daemon that tracks the **career-growth** routine
and serves a Grafana-style dark dashboard at `http://localhost:3000`.

Think of it as a private **Grafana / Datadog for your career**: instead of CPU
and latency, the "metrics, logs, and traces" are *did I ship a project, did I
post today, what tier did I post on, am I on track for this sprint's
deliverables*.

## What it does

- **Auth** — single operator login, cookie sessions, password hashing, API keys.
- **Onboarding** — first-run wizard: pick your role, choose your blocks, launch a
  dashboard shaped to what *you* track.
- **Blocks** — toggleable feature modules. Enable/disable from Settings; the
  sidebar, router, and API all respect your choice.
- **Sprints** — bounded, deliverable-based sprints with computed health alerts
  (on track / warning / at-risk / overdue).
- **Metrics** — posting cadence rate, streaks, ship rate, tier-mix, health.
- **Logs** — daily build-log stream + append-only career-event audit trail.
- **Traces** — sprint as a phase waterfall (width ∝ time per phase).
- **ADRs** — Architecture Decision Records, exportable as PDF.
- **Posts** — content cadence across platforms (LinkedIn, X, Blog, Instagram, TikTok).
- **Todos** — task management with priority, due dates, sprint linkage.
- **Habits** — daily binary habits with heatmap and streak tracking.
- **Weekly Review** — prompted end-of-week reflection, auto-populated from logs.
- **PDF Export** — server-side PDF generation for sprint reports, ADRs, logs, metrics.
- **REST API** — documented `POST /api/v1/logs`, metrics push, trace spans, etc.
- **Command Palette** — `⌘K` / `Ctrl-K` fuzzy search across all entries.
- **Chat** — built-in panel bridges the browser to `kiro-cli` over WebSocket.

## Requirements

- **Go 1.25+** (`go version`).
- A C toolchain — the embedded **libSQL** driver uses cgo. On macOS the Xcode
  CLI tools (`xcode-select --install`) suffice; on Linux install `build-essential`.
- `kiro-cli` on your `PATH` **only if** you want the chat panel.

## Quick start

```bash
./dev.sh           # build + run → http://127.0.0.1:3000
```

On first run you'll hit the setup screen → set a password → onboarding wizard →
dashboard.

## dev.sh — the unified dev script

```bash
./dev.sh              # build + run (default mode)
./dev.sh watch        # live-reload via air (restarts on file changes)
./dev.sh fresh        # delete DB + start clean (forces /setup again)
./dev.sh migrate      # apply pending migrations, then exit
./dev.sh status       # show goose migration status
./dev.sh down         # rollback the last migration
./dev.sh build        # just compile the binary

# Pass any flags to the binary:
./dev.sh --log-level debug
./dev.sh --addr 127.0.0.1:5500
./dev.sh watch --log-level debug
```

## Running without the script

```bash
go build -o bin/local-scava ./cmd/local-scava
./bin/local-scava
# or straight from source:
go run ./cmd/local-scava
```

## Configuration

Resolution order: **flag → env var → default.**

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `--addr` | `SCAVA_ADDR` | `127.0.0.1:3000` | bind address (loopback only) |
| `--db` | `SCAVA_DB` | `~/.local/share/local-scava/scava.db` | libSQL database file |
| `--kiro-bin` | `SCAVA_KIRO_BIN` | `kiro-cli` | agent binary for the chat bridge |
| `--kiro-trust-all` | `SCAVA_KIRO_TRUST_ALL=1` | `false` | agent tools without confirmation |
| `--log-level` | `SCAVA_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `--log-format` | `SCAVA_LOG_FORMAT` | `text` | `text` or `json` |
| `--migrate-only` | — | `false` | run migrations then exit |
| `--migrate-status` | — | `false` | print migration status then exit |
| `--migrate-down` | — | `false` | rollback last migration then exit |

## Migrations

Managed by [goose](https://github.com/pressly/goose) with embedded SQL files.
Migrations apply automatically on startup. CLI commands for manual control:

```bash
./dev.sh migrate      # apply all pending
./dev.sh status       # show applied vs pending
./dev.sh down         # rollback one
```

Current schema: 9 migrations (`0001_init` through `0009_extend_enums`).

## REST API

Base URL: `http://localhost:3000/api/v1`

All endpoints (except `/healthz`) require the `X-Scava-Key` header. Generate a
key in Settings → API.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/healthz` | Health check (no auth) |
| `POST` | `/api/v1/logs` | Append a build log |
| `POST` | `/api/v1/events` | Record a career event |
| `POST` | `/api/v1/metrics/push` | Push a metric data point |
| `POST` | `/api/v1/traces/span` | Push a trace span |
| `GET` | `/api/v1/sprints/active` | Current active sprint + health |
| `GET` | `/api/v1/blocks` | List blocks with enabled state |

Interactive docs at `/api/docs` (session-authenticated).

## Auth flow

```
First run → /setup (set password) → /onboarding (role → blocks → confirm) → /
Subsequent → /login → /
```

- Single operator account, cookie sessions, 14-day idle expiry.
- Password hashed with SHA-256 + random salt, constant-time comparison.
- API keys stored hashed; plaintext shown once at generation.

## Tests

```bash
go test ./...        # unit tests (service, store, domain, bridge)
go vet ./...         # static checks
```

## Project layout

```
cmd/local-scava/        entry point
internal/
  app/                  daemon: config, lifecycle, graceful shutdown
  auth/                 login, sessions, API keys, middleware
  block/                block registry + per-user enable/disable
  onboarding/           three-step first-run wizard
  export/               server-side PDF generation (fpdf)
  domain/               entities + enums (Sprint, User, Todo, Habit, …)
  store/                libSQL access, goose migrations, typed queries
  service/              business rules (sprint health, cadence, metrics)
  web/                  HTTP handlers, templates, static assets, REST API
  bridge/               WebSocket ↔ kiro-cli chat proxy
dev.sh                  unified build/run/migrate script
.air.toml               live-reload config for `air`
```

## Routes

| Route | Page |
|---|---|
| `/` | Overview (dynamic based on enabled blocks) |
| `/sprints`, `/sprints/{id}` | Sprint list + detail with health, deliverables, trace |
| `/cadence`, `/posts/{id}` | Posting heatmap + post detail |
| `/logs` | Logbook — career events + daily logs |
| `/adrs`, `/adrs/{id}` | Architecture Decision Records |
| `/metrics` | Cadence rates, ship rate, streaks, tier-mix |
| `/todos` | Todo list with priority + sprint linkage |
| `/habits` | Habit tracker with heatmap + streaks |
| `/review` | Weekly review (auto-populated) |
| `/new` | Quick-create hub (⌘K / Ctrl-K) |
| `/settings` | Profile, password, blocks toggle, daemon info |
| `/api/docs` | REST API reference |
| `/setup` | First-run password setup |
| `/login` | Sign in |
| `/onboarding/*` | First-run wizard (role → blocks → confirm) |
| `/healthz` | JSON health probe |
| `/ws` | WebSocket chat bridge |

## Design

- **Grafana-style dark theme** — dark canvas, semantic color for signal only
  (green = healthy, yellow = watch, red = act).
- **Dynamic sidebar** — only enabled blocks appear; toggles take effect immediately.
- **Single binary** — all assets (CSS, JS, templates, migrations) embedded via
  `embed.FS`. No external CDN, no Node runtime.
- **Local-first** — loopback-bound, no cloud, no subscriptions, SQLite/libSQL.
- **~22 MB stripped** binary, ~10,000 lines of Go.
