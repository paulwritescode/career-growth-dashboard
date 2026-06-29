package web

import (
	"net/http"

	"github.com/paulkinyatti/local-scava/internal/export"
)

// exportSvc is set during app initialization. We store it on the Handlers struct
// but since the export service was added after initial construction, we add a
// SetExport method.
func (h *Handlers) SetExport(e *export.Service) { h.export = e }

func (h *Handlers) handleSprintExport(w http.ResponseWriter, r *http.Request) {
	if h.export == nil {
		http.Error(w, "export not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data, filename, err := h.export.SprintReport(r.Context(), id)
	if err != nil {
		h.serverError(w, err)
		return
	}
	servePDF(w, data, filename)
}

func (h *Handlers) handleADRExport(w http.ResponseWriter, r *http.Request) {
	if h.export == nil {
		http.Error(w, "export not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data, filename, err := h.export.ADR(r.Context(), id)
	if err != nil {
		h.serverError(w, err)
		return
	}
	servePDF(w, data, filename)
}

func (h *Handlers) handleLogExport(w http.ResponseWriter, r *http.Request) {
	if h.export == nil {
		http.Error(w, "export not configured", http.StatusServiceUnavailable)
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	data, filename, err := h.export.LogRange(r.Context(), from, to)
	if err != nil {
		h.serverError(w, err)
		return
	}
	servePDF(w, data, filename)
}

func (h *Handlers) handleMetricsExport(w http.ResponseWriter, r *http.Request) {
	if h.export == nil {
		http.Error(w, "export not configured", http.StatusServiceUnavailable)
		return
	}
	window := r.URL.Query().Get("window")
	if window == "" {
		window = "30d"
	}
	data, filename, err := h.export.MetricsSnapshot(r.Context(), window)
	if err != nil {
		h.serverError(w, err)
		return
	}
	servePDF(w, data, filename)
}

func servePDF(w http.ResponseWriter, data []byte, filename string) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Write(data)
}
