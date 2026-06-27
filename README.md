# local-scava

A local-first, single-binary Go daemon that tracks the **career-growth** routine —
the *Monthly Skill Sprint* and the *Three-Tier Content Cadence* — and serves a
monochrome, SRE-style dashboard at `http://localhost:5500`.

Think of it as a private **Grafana for your career**: instead of CPU and latency,
the "metrics, logs, and traces" are *did I ship a project this month, did I post
today, what tier did I post on, am I on track for this sprint's phase*.

- **Metrics** — posting cadence rate, streaks, ship rate, tier-mix donut, phase health.
- **Logs** — the daily build-log stream + an append-only career-event audit trail.
- **Traces** — a sprint rendered as a phase waterfall (width ∝ time spent per phase).

A built-in chat panel bridges the browser to `kiro-cli` over a WebSocket so you
can create/update entries conversationally.

## Requirements

- **Go 1.25+** (`go version`).
- A C toolchain — the embedded **libSQL** driver (`tursodatabase/go-libsql`) uses
  cgo. On macOS the Xcode command-line tools (`xcode-select --install`) are
  enough; on Linux install `gcc`/`build-essential`.
- `kiro-cli` on your `PATH` **only if** you want the chat panel. The dashboard
  runs fine without it.

## Quick start

```bash
# from the repo root
./run.sh
```

`run.sh` builds the binary into `./bin/local-scava` (stamping the git short SHA as
the version) and starts the daemon. When you see `dashboard ready`, open:

```
http://localhost:5500
```

Press **Ctrl-C** to stop — the daemon shuts down gracefully (drains the HTTP
server, kills any chat child processes, closes the database).

### Passing options

Any flags after `./run.sh` are forwarded to the binary:

```bash
./run.sh --addr 127.0.0.1:5600          # run on a different port
./run.sh --db /tmp/scava.db             # use a throwaway database
./run.sh --log-level debug              # verbose logs
./run.sh --log-format json              # structured JSON logs
./run.sh --migrate-only                 # apply DB migrations, then exit
./run.sh --help                         # full flag list
```

## Running without the script

```bash
# build
go build -o bin/local-scava ./cmd/local-scava

# run (foreground; logs to stderr)
./bin/local-scava

# or run straight from source
go run ./cmd/local-scava --addr 127.0.0.1:5500
```

## Configuration

Resolution order: **command-line flag → environment variable → default.**

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `--addr` | `SCAVA_ADDR` | `127.0.0.1:5500` | bind address (loopback only in v1) |
| `--db` | `SCAVA_DB` | `~/.local/share/local-scava/scava.db` | libSQL database file |
| `--kiro-bin` | `SCAVA_KIRO_BIN` | `kiro-cli` (from `PATH`) | agent binary for the chat bridge |
| `--kiro-trust-all` | `SCAVA_KIRO_TRUST_ALL=1` | `false` | let the chat agent run tools without confirmation (riskier) |
| `--log-level` | `SCAVA_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `--log-format` | `SCAVA_LOG_FORMAT` | `text` | `text` or `json` |
| `--migrate-only` | — | `false` | run migrations then exit (no server) |

The bind address is **restricted to loopback** (`127.0.0.1` / `localhost`); a
non-loopback address is rejected at startup, because the chat bridge can execute
an agent that mutates your data. See `specs/10-security.md` in the spec docs.

The database directory is created `0700` and the `scava.db` file is forced to
`0600` (user-only).

## Health check

```bash
curl http://localhost:5500/healthz
# {"db":"ok","status":"ok"}
```

## Chat panel (kiro-cli bridge)

The dashboard has a collapsible chat drawer (the **chat ▸** button in the top
bar, on every page) that talks to a local `kiro-cli` over the `/ws` WebSocket.
Each message you send runs one non-interactive `kiro-cli chat` turn; the answer
streams back into the drawer as clean text (terminal color/cursor codes are
stripped), and follow-up messages continue the same conversation.

You can also **create entries by talking to it** — e.g. "log today that I wired
the websocket bridge", "start a sprint learning Redis, microapp is a rate
limiter", "mark today's LinkedIn post published, url https://…". The agent emits
a structured `SCAVA-ACTION` directive that the bridge validates against an
allowlist and routes through the **same service methods the web forms use**
(recorded in the audit trail as `source=chat`). The affected dashboard view
refreshes automatically when a chat action succeeds. Unrecognized actions are
shown as plain text and never executed.

- Requires `kiro-cli` on your `PATH` (or point `--kiro-bin` at it). If it isn't
  found, the drawer reports the error and the rest of the dashboard keeps
  working.
- Creating entries via chat does **not** require `--kiro-trust-all` — the intent
  directive is parsed by the bridge, not run as an agent tool. `--kiro-trust-all`
  (or `SCAVA_KIRO_TRUST_ALL=1`) is only needed if you want the agent to run its
  own tools (shell/fs) without confirmation — riskier; a loud warning is logged
  at startup when it's on.
- Security: the bridge launches `kiro-cli` with an explicit args array (never a
  shell), validates WebSocket `Origin` + `Host` against a loopback allowlist, and
  the agent only ever reaches the database through typed, validated, allowlisted
  service methods — never raw SQL. See `specs/07-kiro-cli-chat-bridge.md` and
  `specs/10-security.md`.

## Tests

```bash
go test ./...        # unit tests (service, store, domain, bridge)
go vet ./...         # static checks
gofmt -l internal cmd # formatting (empty output = clean)
```

## Project layout

```
cmd/local-scava/        entry point — wires config, store, services, HTTP server
internal/
  app/                  daemon supervisor: config, lifecycle, graceful shutdown, /healthz
  domain/               core entities + enums (Sprint, Post, PostTier, ADR, …)
  store/                libSQL access, embedded migrations, parameterized queries
  service/              business rules (sprint, content, logbook, metrics, traces)
  web/                  HTTP handlers, html/template views, embedded static assets, SVG charts
  bridge/               WebSocket ↔ kiro-cli stdio proxy (the chat panel)
run.sh                  build + run helper
```

The full specification lives in the spec docs under
`~/Documents/my-vault/local-scava` (`specs/00`–`10`, `schema/`, `naming.md`,
`frontend-templates.md`).

## Routes

| Route | Page |
|---|---|
| `/` | Overview — today status, current sprint, streaks |
| `/sprints`, `/sprints/{id}` | sprint list + 12-month grid; detail with phase stepper, checklist, **trace** |
| `/cadence`, `/posts/{id}` | posting heatmap + post list; per-tier post detail |
| `/logs` | Logbook — career events + daily build logs |
| `/adrs` | Architecture Decision Records |
| `/metrics` | cadence rates, ship rate, streaks, **tier-mix donut** |
| `/new` | quick-create hub (also reachable via **⌘K / Ctrl-K**) |
| `/settings` | read-only config + security posture |
| `/healthz` | JSON health probe |
| `/ws` | WebSocket chat bridge |

> Status: **v1.** Single user, single machine, manual start (no boot-on-startup,
> no Docker, no auth beyond loopback) — all deliberate v1 non-goals.
