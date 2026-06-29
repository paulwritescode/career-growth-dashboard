package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/paulkinyatti/local-scava/internal/block"
	"github.com/paulkinyatti/local-scava/internal/domain"
	"github.com/paulkinyatti/local-scava/internal/service"
)

// MountAPI registers all /api/v1 routes on the given mux.
func (h *Handlers) MountAPI(mux *http.ServeMux) {
	// Health (no auth).
	mux.HandleFunc("GET /api/v1/healthz", h.apiHealthz)

	// Authenticated endpoints.
	mux.HandleFunc("POST /api/v1/logs", h.apiRequireKey(h.apiCreateLog))
	mux.HandleFunc("POST /api/v1/events", h.apiRequireKey(h.apiCreateEvent))
	mux.HandleFunc("POST /api/v1/metrics/push", h.apiRequireKey(h.apiPushMetric))
	mux.HandleFunc("POST /api/v1/traces/span", h.apiRequireKey(h.apiPushSpan))
	mux.HandleFunc("GET /api/v1/sprints/active", h.apiRequireKey(h.apiActiveSprint))
	mux.HandleFunc("GET /api/v1/blocks", h.apiRequireKey(h.apiListBlocks))
}

// apiRequireKey wraps a handler with API key authentication.
func (h *Handlers) apiRequireKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Scava-Key")
		if key == "" {
			apiJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		_, err := h.auth.AuthenticateKey(r.Context(), key)
		if err != nil {
			apiJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

// --- Endpoint handlers ---

func (h *Handlers) apiHealthz(w http.ResponseWriter, r *http.Request) {
	apiJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": h.meta.Version})
}

func (h *Handlers) apiCreateLog(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkedOn     string `json:"worked_on"`
		WhatHappened string `json:"what_happened"`
		Insight      string `json:"insight"`
		NextUp       string `json:"next_up"`
		SprintID     *int64 `json:"sprint_id"`
	}
	if err := readJSON(r, &body); err != nil {
		apiJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(body.WorkedOn) == "" {
		apiJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "worked_on is required"})
		return
	}

	log, err := h.svc.CreateLog(r.Context(), service.CreateLogInput{
		WorkedOn:     body.WorkedOn,
		WhatHappened: body.WhatHappened,
		Insight:      body.Insight,
		NextUp:       body.NextUp,
		SprintID:     body.SprintID,
		Source:       domain.SourceAPI,
	})
	if err != nil {
		apiJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	apiJSON(w, http.StatusCreated, map[string]any{
		"id":         log.ID,
		"log_date":   log.LogDate,
		"sprint_id":  log.SprintID,
		"created_at": log.CreatedAt.Format(time.RFC3339),
	})
}

func (h *Handlers) apiCreateEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind     string `json:"kind"`
		Detail   string `json:"detail"`
		SprintID *int64 `json:"sprint_id"`
	}
	if err := readJSON(r, &body); err != nil {
		apiJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(body.Kind) == "" {
		apiJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "kind is required"})
		return
	}

	event := domain.CareerEvent{
		OccurredAt: time.Now().UTC(),
		Kind:       body.Kind,
		Source:     domain.SourceAPI,
		SprintID:   body.SprintID,
		Summary:    body.Detail,
		Detail:     body.Detail,
	}
	if err := h.svc.AppendEvent(r.Context(), event); err != nil {
		apiJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record event"})
		return
	}
	apiJSON(w, http.StatusCreated, map[string]any{
		"kind":        body.Kind,
		"occurred_at": event.OccurredAt.Format(time.RFC3339),
	})
}

func (h *Handlers) apiPushMetric(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string   `json:"name"`
		Value float64  `json:"value"`
		Tags  *string  `json:"tags"`
		At    *string  `json:"at"`
	}
	if err := readJSON(r, &body); err != nil {
		apiJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		apiJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "name is required"})
		return
	}

	occurredAt := time.Now().UTC()
	if body.At != nil && *body.At != "" {
		if t, err := time.Parse(time.RFC3339, *body.At); err == nil {
			occurredAt = t
		}
	}

	if err := h.svc.PushMetricPoint(r.Context(), domain.MetricPoint{
		Name:       body.Name,
		Value:      body.Value,
		Tags:       body.Tags,
		OccurredAt: occurredAt,
	}); err != nil {
		apiJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to push metric"})
		return
	}
	apiJSON(w, http.StatusAccepted, map[string]any{"name": body.Name, "value": body.Value})
}

func (h *Handlers) apiPushSpan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SprintID   *int64 `json:"sprint_id"`
		Phase      *int   `json:"phase"`
		Name       string `json:"name"`
		DurationMS int64  `json:"duration_ms"`
		StartedAt  string `json:"started_at"`
	}
	if err := readJSON(r, &body); err != nil {
		apiJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		apiJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "name is required"})
		return
	}

	startedAt := time.Now().UTC()
	if body.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, body.StartedAt); err == nil {
			startedAt = t
		}
	}

	id, err := h.svc.PushTraceSpan(r.Context(), domain.TraceSpan{
		SprintID:   body.SprintID,
		Phase:      body.Phase,
		Name:       body.Name,
		DurationMS: body.DurationMS,
		StartedAt:  startedAt,
	})
	if err != nil {
		apiJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to push span"})
		return
	}
	apiJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *Handlers) apiActiveSprint(w http.ResponseWriter, r *http.Request) {
	sp, err := h.svc.CurrentSprint(r.Context())
	if err != nil {
		if err == service.ErrNotFound {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		apiJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed"})
		return
	}

	health, _ := h.svc.SprintHealth(r.Context(), sp.ID)
	done, total, _ := h.svc.DeliverableCounts(r.Context(), sp.ID)

	result := map[string]any{
		"id":      sp.ID,
		"title":   coalesceStr(sp.Title, &sp.SkillName),
		"phase":   sp.CurrentPhase,
		"health":  string(health.Severity),
		"deliverables": map[string]int{
			"done":  done,
			"total": total,
		},
	}
	if sp.StartsOn != nil {
		result["starts_on"] = *sp.StartsOn
	}
	if sp.EndsOn != nil {
		result["ends_on"] = *sp.EndsOn
	}
	apiJSON(w, http.StatusOK, result)
}

func (h *Handlers) apiListBlocks(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		// For API, try getting user from the key (already validated).
		// Since we're single-user, just use user 1.
		userID = 1
	}

	enabled, _ := h.block.Enabled(r.Context(), userID)
	enabledKeys := make(map[string]bool)
	for _, d := range enabled {
		enabledKeys[d.Key] = true
	}

	var blocks []map[string]any
	for _, d := range block.Registry() {
		blocks = append(blocks, map[string]any{
			"key":     d.Key,
			"name":    d.Name,
			"enabled": enabledKeys[d.Key],
		})
	}
	apiJSON(w, http.StatusOK, map[string]any{"blocks": blocks})
}

// --- Helpers ---

func apiJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB max
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func coalesceStr(a *string, b *string) string {
	if a != nil && *a != "" {
		return *a
	}
	if b != nil {
		return *b
	}
	return ""
}
