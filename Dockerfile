# ─────────────────────────────────────────────────────────────────────────────
# local-scava — Dockerized single-binary daemon
#
# Design mirrors Grafana's official image conventions:
#   • Exposes a single port (3000) with 0.0.0.0 binding inside the container
#   • All settings overridable via environment variables (SCAVA_*)
#   • Stateless by default — data persists only if you mount /var/lib/scava
#   • Runs as a non-root user for security
#
# Quick start:
#   docker build -t local-scava .
#   docker run -d -p 3000:3000 -v scava-data:/var/lib/scava local-scava
#
# ─────────────────────────────────────────────────────────────────────────────

# ── Stage 1: Build ───────────────────────────────────────────────────────────
FROM golang:1.24-bookworm AS builder

# CGO is required by go-libsql (tursodatabase driver)
ENV CGO_ENABLED=1

WORKDIR /src

# Cache dependency downloads separately from source changes
COPY go.mod go.sum ./
RUN go mod download

# Copy full source — templates and static assets are embedded at compile time
COPY . .

ARG VERSION=0.1.0-docker

RUN go build -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/local-scava ./cmd/local-scava

# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Non-root user (mirrors Grafana's 472:472 pattern)
RUN groupadd -r scava && useradd -r -g scava -d /home/scava -m scava

COPY --from=builder /out/local-scava /usr/local/bin/local-scava

# ── Data directory ───────────────────────────────────────────────────────────
# The SQLite database (scava.db) lives here. Like Grafana's /var/lib/grafana,
# this path is ephemeral unless you mount a volume.
RUN mkdir -p /var/lib/scava && chown scava:scava /var/lib/scava
VOLUME ["/var/lib/scava"]

# ── Configuration via environment ────────────────────────────────────────────
# Override any setting at runtime with -e flags, just like Grafana's GF_* vars.
#
#   SCAVA_ADDR        Bind address       (default: 0.0.0.0:3000)
#   SCAVA_DB          Database file path  (default: /var/lib/scava/scava.db)
#   SCAVA_LOG_LEVEL   Log verbosity       (default: info)
#   SCAVA_LOG_FORMAT  Log output format   (default: json)
#   SCAVA_KIRO_BIN    Path to kiro-cli    (default: kiro-cli)
#
ENV SCAVA_CONTAINER="1"
ENV SCAVA_ADDR="0.0.0.0:3000"
ENV SCAVA_DB="/var/lib/scava/scava.db"
ENV SCAVA_LOG_LEVEL="info"
ENV SCAVA_LOG_FORMAT="json"

USER scava
WORKDIR /home/scava

EXPOSE 3000

ENTRYPOINT ["local-scava"]
