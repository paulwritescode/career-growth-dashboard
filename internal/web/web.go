// Package web serves the monochrome, SRE-style dashboard. It renders
// server-side HTML with html/template and progressively enhances forms with
// htmx. Templates and static assets are embedded so the daemon ships as a
// single binary.
//
// Layer rule: web imports service and domain. Handlers parse requests, call a
// service method, and render a view. No SQL or business rules live here.
package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
	"github.com/paulkinyatti/local-scava/internal/service"
)

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Meta holds read-only daemon facts surfaced on the Settings page and footer.
type Meta struct {
	Addr    string
	DBPath  string
	KiroBin string
	Version string
}

// Handlers holds the dependencies shared by all HTTP handlers.
type Handlers struct {
	svc       *service.Service
	log       *slog.Logger
	meta      Meta
	templates map[string]*template.Template // page name -> parsed template set
	static    http.Handler
}

// New builds the web handlers, parsing all page templates against the shared
// layout and partials.
func New(svc *service.Service, log *slog.Logger, meta Meta) (*Handlers, error) {
	tmpls, err := buildTemplates()
	if err != nil {
		return nil, err
	}
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	return &Handlers{
		svc:       svc,
		log:       log,
		meta:      meta,
		templates: tmpls,
		static:    http.StripPrefix("/static/", http.FileServer(http.FS(sub))),
	}, nil
}

// Mount registers all dashboard routes on the given mux.
func (h *Handlers) Mount(mux *http.ServeMux) {
	mux.Handle("GET /static/", h.static)

	mux.HandleFunc("GET /{$}", h.handleOverview)
	mux.HandleFunc("GET /sprints", h.handleSprintList)
	mux.HandleFunc("GET /sprints/{id}", h.handleSprintDetail)
	mux.HandleFunc("GET /cadence", h.handleCadence)
	mux.HandleFunc("GET /posts/{id}", h.handlePostDetail)
	mux.HandleFunc("GET /logs", h.handleLogbook)
	mux.HandleFunc("GET /adrs", h.handleADRList)
	mux.HandleFunc("GET /metrics", h.handleMetrics)
	mux.HandleFunc("GET /new", h.handleNew)
	mux.HandleFunc("GET /settings", h.handleSettings)

	// Mutating form routes (task 8 wires the bodies; declared here so the nav works).
	mux.HandleFunc("POST /sprints", h.handleSprintCreate)
	mux.HandleFunc("POST /sprints/{id}/phase", h.handleSprintPhase)
	mux.HandleFunc("POST /sprints/{id}/status", h.handleSprintStatus)
	mux.HandleFunc("POST /sprints/{id}/retro", h.handleSprintRetro)
	mux.HandleFunc("POST /checklist/{id}/toggle", h.handleChecklistToggle)
	mux.HandleFunc("POST /logs", h.handleLogCreate)
	mux.HandleFunc("POST /posts", h.handlePostCreate)
	mux.HandleFunc("POST /posts/{id}/tier", h.handleTierUpdate)
	mux.HandleFunc("POST /adrs", h.handleADRCreate)
}

// funcMap holds template helpers used across views.
func funcMap() template.FuncMap {
	return template.FuncMap{
		"derefStr": func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		},
		"pct":    func(f float64) string { return fmt.Sprintf("%.0f%%", f*100) },
		"phases": domain.AllPhases,
		"tiers":  domain.AllTiers,
		"shortDate": func(s string) string {
			if len(s) >= 10 {
				return s[:10]
			}
			return s
		},
		"timeShort":  func(t time.Time) string { return t.Local().Format("2006-01-02 15:04") },
		"add":        func(a, b int) int { return a + b },
		"title":      func(s string) string { return s },
		"phaseLabel": func(p domain.Phase) string { return p.Label() },
		"sparkline":  sparklineSVG,
		"tierBars":   tierBarsSVG,
		"tierDonut":  tierMixDonutSVG,
		"trace":      sprintTraceSVG,
		// Grafana-style additions.
		"barGauge":     barGaugeSVG,
		"areaChart":    areaChartSVG,
		"uptime":       uptimeSVG,
		"progressRing": progressRingSVG,
		"activityFeed": activityFeedHTML,
	}
}

// buildTemplates parses layout + partials + each page into its own set.
func buildTemplates() (map[string]*template.Template, error) {
	base := []string{"templates/layout.html"}
	partials, err := fs.Glob(templatesFS, "templates/partials/*.html")
	if err != nil {
		return nil, err
	}
	pages, err := fs.Glob(templatesFS, "templates/pages/*.html")
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("no page templates found")
	}
	out := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		name := baseName(page)
		files := append(append([]string{}, base...), partials...)
		files = append(files, page)
		t, err := template.New("layout.html").Funcs(funcMap()).ParseFS(templatesFS, files...)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", page, err)
		}
		out[name] = t
	}
	return out, nil
}

func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			p = p[i+1:]
			break
		}
	}
	if len(p) > 5 && p[len(p)-5:] == ".html" {
		p = p[:len(p)-5]
	}
	return p
}

// render executes the named page template's "base" definition with data.
func (h *Handlers) render(w http.ResponseWriter, page string, data any) {
	t, ok := h.templates[page]
	if !ok {
		h.serverError(w, fmt.Errorf("template %q not found", page))
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", data); err != nil {
		h.serverError(w, fmt.Errorf("execute %q: %w", page, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// renderPartial executes a named template (not the full layout) for htmx swaps.
func (h *Handlers) renderPartial(w http.ResponseWriter, page, define string, data any) {
	t, ok := h.templates[page]
	if !ok {
		h.serverError(w, fmt.Errorf("template %q not found", page))
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, define, data); err != nil {
		h.serverError(w, fmt.Errorf("execute partial %q/%q: %w", page, define, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (h *Handlers) serverError(w http.ResponseWriter, err error) {
	h.log.Error("web error", "err", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
