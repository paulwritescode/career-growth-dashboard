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

	"github.com/paulkinyatti/local-scava/internal/auth"
	"github.com/paulkinyatti/local-scava/internal/block"
	"github.com/paulkinyatti/local-scava/internal/domain"
	"github.com/paulkinyatti/local-scava/internal/export"
	"github.com/paulkinyatti/local-scava/internal/onboarding"
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
	svc        *service.Service
	auth       *auth.Service
	block      *block.Service
	onboarding *onboarding.Service
	export     *export.Service
	log        *slog.Logger
	meta       Meta
	templates  map[string]*template.Template // page name -> parsed template set
	static     http.Handler
}

// New builds the web handlers, parsing all page templates against the shared
// layout and partials.
func New(svc *service.Service, authSvc *auth.Service, blockSvc *block.Service, onboardingSvc *onboarding.Service, log *slog.Logger, meta Meta) (*Handlers, error) {
	tmpls, err := buildTemplates()
	if err != nil {
		return nil, err
	}
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	return &Handlers{
		svc:        svc,
		auth:       authSvc,
		block:      blockSvc,
		onboarding: onboardingSvc,
		log:        log,
		meta:       meta,
		templates:  tmpls,
		static:     http.StripPrefix("/static/", http.FileServer(http.FS(sub))),
	}, nil
}

// Mount registers all dashboard routes on the given mux.
func (h *Handlers) Mount(mux *http.ServeMux) {
	mux.Handle("GET /static/", h.static)

	// Auth routes (no session required).
	mux.HandleFunc("GET /setup", h.handleSetupForm)
	mux.HandleFunc("POST /setup", h.handleSetupSubmit)
	mux.HandleFunc("GET /login", h.handleLoginForm)
	mux.HandleFunc("POST /login", h.handleLoginSubmit)
	mux.HandleFunc("POST /logout", h.handleLogout)

	// Onboarding routes (session required, but not onboarding-complete).
	mux.HandleFunc("GET /onboarding/role", h.handleOnboardingRole)
	mux.HandleFunc("POST /onboarding/role", h.handleOnboardingRoleSubmit)
	mux.HandleFunc("GET /onboarding/blocks", h.handleOnboardingBlocks)
	mux.HandleFunc("POST /onboarding/blocks", h.handleOnboardingBlocksSubmit)
	mux.HandleFunc("GET /onboarding/confirm", h.handleOnboardingConfirm)
	mux.HandleFunc("POST /onboarding/confirm", h.handleOnboardingConfirmSubmit)

	// Dashboard routes — all require an authenticated session.
	ra := h.requireAuth
	mux.HandleFunc("GET /{$}", ra(h.handleOverview))
	mux.HandleFunc("GET /sprints", ra(h.blockGate("sprint", h.handleSprintList)))
	mux.HandleFunc("GET /sprints/{id}", ra(h.handleSprintDetail))
	mux.HandleFunc("GET /cadence", ra(h.blockGate("posts", h.handleCadence)))
	mux.HandleFunc("GET /posts/{id}", ra(h.blockGate("posts", h.handlePostDetail)))
	mux.HandleFunc("GET /logs", ra(h.blockGate("logs", h.handleLogbook)))
	mux.HandleFunc("GET /adrs", ra(h.blockGate("adr", h.handleADRList)))
	mux.HandleFunc("GET /adrs/{id}", ra(h.blockGate("adr", h.handleADRDetail)))
	mux.HandleFunc("GET /metrics", ra(h.blockGate("metrics", h.handleMetrics)))
	mux.HandleFunc("GET /traces", ra(h.blockGate("traces", h.handleTraces)))
	mux.HandleFunc("GET /new", ra(h.handleNew))
	mux.HandleFunc("GET /settings", ra(h.handleSettings))

	// Block toggle.
	mux.HandleFunc("POST /settings/blocks/{key}", ra(h.handleBlockToggle))

	// Profile and password.
	mux.HandleFunc("POST /settings/profile", ra(h.handleProfileUpdate))
	mux.HandleFunc("POST /settings/password", ra(h.handlePasswordChange))

	// Mutating form routes.
	mux.HandleFunc("POST /sprints", ra(h.handleSprintCreate))
	mux.HandleFunc("POST /sprints/{id}/phase", ra(h.handleSprintPhase))
	mux.HandleFunc("POST /sprints/{id}/status", ra(h.handleSprintStatus))
	mux.HandleFunc("POST /sprints/{id}/retro", ra(h.handleSprintRetro))
	mux.HandleFunc("POST /sprints/{id}/delete", ra(h.handleSprintDelete))
	mux.HandleFunc("POST /checklist/{id}/toggle", ra(h.handleChecklistToggle))
	mux.HandleFunc("POST /logs", ra(h.handleLogCreate))
	mux.HandleFunc("POST /logs/{id}/delete", ra(h.handleLogDelete))
	mux.HandleFunc("POST /posts", ra(h.handlePostCreate))
	mux.HandleFunc("POST /posts/{id}/tier", ra(h.handleTierUpdate))
	mux.HandleFunc("POST /posts/{id}/delete", ra(h.handlePostDelete))
	mux.HandleFunc("POST /adrs", ra(h.handleADRCreate))
	mux.HandleFunc("POST /adrs/{id}/update", ra(h.handleADRUpdate))
	mux.HandleFunc("POST /adrs/{id}/delete", ra(h.handleADRDelete))

	// PDF export routes.
	mux.HandleFunc("GET /sprints/{id}/export.pdf", ra(h.handleSprintExport))
	mux.HandleFunc("GET /adrs/{id}/export.pdf", ra(h.handleADRExport))
	mux.HandleFunc("GET /logs/export.pdf", ra(h.handleLogExport))
	mux.HandleFunc("GET /metrics/export.pdf", ra(h.handleMetricsExport))

	// Block-gated routes (todo, habits, review).
	mux.HandleFunc("GET /todos", ra(h.blockGate("todo", h.handleTodos)))
	mux.HandleFunc("POST /todos", ra(h.handleTodoCreate))
	mux.HandleFunc("POST /todos/{id}/status", ra(h.handleTodoStatus))
	mux.HandleFunc("POST /todos/{id}/delete", ra(h.handleTodoDelete))
	mux.HandleFunc("GET /habits", ra(h.blockGate("habits", h.handleHabits)))
	mux.HandleFunc("POST /habits", ra(h.handleHabitCreate))
	mux.HandleFunc("POST /habits/{id}/toggle", ra(h.handleHabitToggle))
	mux.HandleFunc("POST /habits/{id}/archive", ra(h.handleHabitArchive))
	mux.HandleFunc("GET /review", ra(h.blockGate("review", h.handleReview)))
	mux.HandleFunc("POST /review", ra(h.handleReviewSave))

	// API docs.
	mux.HandleFunc("GET /api/docs", ra(h.handleAPIDocs))

	// Command palette search and user-info endpoint.
	mux.HandleFunc("GET /api/search", ra(h.handleSearch))
	mux.HandleFunc("GET /api/me", ra(h.handleAPIMe))

	// REST API v1.
	h.MountAPI(mux)
}

// roleItem is a helper for templates to render role options.
type roleItem struct {
	Value string
	Label string
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
		"roles": func() []roleItem {
			return []roleItem{
				{"backend", "Backend engineer"},
				{"frontend", "Frontend engineer"},
				{"fullstack", "Full-stack engineer"},
				{"devops", "DevOps / SRE / Platform"},
				{"data", "Data / ML engineer"},
				{"indie", "Indie hacker / solo founder"},
				{"manager", "Engineering manager"},
				{"other", "Other"},
			}
		},
		"shortDate": func(s string) string {
			if len(s) >= 10 {
				return s[:10]
			}
			return s
		},
		"timeShort":  func(t time.Time) string { return t.Local().Format("2006-01-02 15:04") },
		"add":        func(a, b int) int { return a + b },
		"phaseNext":  func(p domain.Phase) domain.Phase { return p + 1 },
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
		// Phase-2 template helpers.
		"seq": func(from, to int) []int {
			var out []int
			if from >= to {
				for i := from; i >= to; i-- {
					out = append(out, i)
				}
			} else {
				for i := from; i <= to; i++ {
					out = append(out, i)
				}
			}
			return out
		},
		"daysAgo": func(n int) string {
			return time.Now().AddDate(0, 0, -n).Format("2006-01-02")
		},
		"inSlice": func(slice []string, item string) bool {
			for _, s := range slice {
				if s == item {
					return true
				}
			}
			return false
		},
		"apiEndpoints": func() []apiEndpoint {
			return apiEndpointList()
		},
	}
}

// buildTemplates parses layout + partials + each page into its own set.
func buildTemplates() (map[string]*template.Template, error) {
	base := []string{"templates/layout.html"}
	chromeless := []string{"templates/chromeless.html"}
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

	// Pages that use the chromeless layout (no sidebar/topbar).
	chromelessPages := map[string]bool{
		"setup":              true,
		"login":              true,
		"onboarding_role":    true,
		"onboarding_blocks":  true,
		"onboarding_confirm": true,
	}

	out := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		name := baseName(page)
		var layout []string
		if chromelessPages[name] {
			layout = chromeless
		} else {
			layout = base
		}
		files := append(append([]string{}, layout...), partials...)
		files = append(files, page)

		defName := "layout.html"
		if chromelessPages[name] {
			defName = "chromeless.html"
		}

		t, err := template.New(defName).Funcs(funcMap()).ParseFS(templatesFS, files...)
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

// chromelessPages is the set of page template names that use the chromeless layout.
var chromelessPages = map[string]bool{
	"setup":              true,
	"login":              true,
	"onboarding_role":    true,
	"onboarding_blocks":  true,
	"onboarding_confirm": true,
}

// render executes the named page template's layout definition with data.
func (h *Handlers) render(w http.ResponseWriter, page string, data any) {
	t, ok := h.templates[page]
	if !ok {
		h.serverError(w, fmt.Errorf("template %q not found", page))
		return
	}
	var buf bytes.Buffer
	defName := "base"
	if chromelessPages[page] {
		defName = "chromeless"
	}
	if err := t.ExecuteTemplate(&buf, defName, data); err != nil {
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
