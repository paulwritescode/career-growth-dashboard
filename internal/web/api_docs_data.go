package web

// apiEndpoint describes one REST endpoint for the /api/docs page.
type apiEndpoint struct {
	Method      string
	Path        string
	Description string
	Auth        bool
	Example     string
}

// apiEndpointList returns the static list of documented API endpoints.
func apiEndpointList() []apiEndpoint {
	return []apiEndpoint{
		{
			Method:      "GET",
			Path:        "/api/v1/healthz",
			Description: "Health check. Returns status and version.",
			Auth:        false,
			Example:     `200 { "status": "ok", "version": "0.2.0" }`,
		},
		{
			Method:      "POST",
			Path:        "/api/v1/logs",
			Description: "Append a daily build log entry. Attaches to the active sprint by default.",
			Auth:        true,
			Example: `POST /api/v1/logs
X-Scava-Key: sk_live_...
Content-Type: application/json

{ "worked_on": "shipped auth flow",
  "what_happened": "cookie sessions wired",
  "insight": "argon2id params matter",
  "next_up": "onboarding wizard" }

→ 201 { "id": 142, "log_date": "2026-06-29" }`,
		},
		{
			Method:      "POST",
			Path:        "/api/v1/events",
			Description: "Record a career event (audit log entry).",
			Auth:        true,
			Example: `POST /api/v1/events
{ "kind": "deploy.succeeded", "detail": "ReplyLoop v0.3 to Fly.io" }

→ 201 { "kind": "deploy.succeeded", "occurred_at": "..." }`,
		},
		{
			Method:      "POST",
			Path:        "/api/v1/metrics/push",
			Description: "Push a metric data point (name + numeric value + optional tags).",
			Auth:        true,
			Example: `POST /api/v1/metrics/push
{ "name": "commits", "value": 12, "tags": { "repo": "replyloop" } }

→ 202 { "name": "commits", "value": 12 }`,
		},
		{
			Method:      "POST",
			Path:        "/api/v1/traces/span",
			Description: "Push a trace span linked to a sprint phase.",
			Auth:        true,
			Example: `POST /api/v1/traces/span
{ "sprint_id": 7, "phase": 2, "name": "build: limiter",
  "duration_ms": 5400000, "started_at": "2026-06-26T09:00:00Z" }

→ 201 { "id": 31 }`,
		},
		{
			Method:      "GET",
			Path:        "/api/v1/sprints/active",
			Description: "Get the current active sprint with health status and deliverable progress.",
			Auth:        true,
			Example: `200 { "id": 7, "title": "Build ReplyLoop Auth", "phase": 2,
      "deliverables": { "done": 3, "total": 6 }, "health": "warning" }
204 No Content  ← when no sprint is active`,
		},
		{
			Method:      "GET",
			Path:        "/api/v1/blocks",
			Description: "List all blocks with their enabled/disabled state.",
			Auth:        true,
			Example: `200 { "blocks": [
  { "key": "sprint", "name": "Sprint", "enabled": true },
  { "key": "logs", "name": "Logs", "enabled": true },
  ...
] }`,
		},
	}
}
