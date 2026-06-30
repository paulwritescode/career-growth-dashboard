package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/paulkinyatti/local-scava/internal/block"
	"github.com/paulkinyatti/local-scava/internal/domain"
	"github.com/paulkinyatti/local-scava/internal/service"
)

// SidebarItem represents one entry in the dynamic sidebar.
type SidebarItem struct {
	Key   string // block key or fixed key like "overview"
	Name  string
	Href  string
	Icon  string // SVG path content
	Group string // sidebar group
	Count int    // badge count (0 = no badge)
}

// pageBase carries fields common to every page (for the layout/sidebar).
type pageBase struct {
	Title         string
	Nav           string // active sidebar key
	Today         string
	Flash         string
	HasActive     bool   // an active sprint exists (for the top status bar)
	ActiveID      int64  // active sprint's ID (for linking)
	ActiveSkill   string // active sprint's skill name
	ActivePhase   int    // active sprint's current phase number
	Sidebar       []SidebarItem // dynamic sidebar entries based on enabled blocks
	EnabledBlocks []string      // list of enabled block keys for this user
}

func (h *Handlers) base(title, nav string) pageBase {
	pb := pageBase{Title: title, Nav: nav, Today: h.svc.Today()}
	if sp, err := h.svc.CurrentSprint(context.Background()); err == nil {
		pb.HasActive = true
		pb.ActiveID = sp.ID
		pb.ActiveSkill = sp.SkillName
		pb.ActivePhase = int(sp.CurrentPhase)
	}
	pb.Sidebar, pb.EnabledBlocks = h.buildSidebar()
	return pb
}

// buildSidebar constructs the dynamic sidebar entries from enabled blocks.
// Returns the item list and a flat slice of enabled block keys.
func (h *Handlers) buildSidebar() ([]SidebarItem, []string) {
	userID := int64(1)
	enabled, _ := h.block.Enabled(context.Background(), userID)
	enabledKeys := make(map[string]bool, len(enabled))
	var enabledSlice []string
	for _, d := range enabled {
		enabledKeys[d.Key] = true
		enabledSlice = append(enabledSlice, d.Key)
	}

	// Collect per-block counts for badges.
	counts := h.sidebarCounts(context.Background(), userID)

	var items []SidebarItem
	items = append(items, SidebarItem{Key: "overview", Name: "Overview", Href: "/", Icon: "grid", Group: "Watch"})

	type blockNav struct {
		key   string
		name  string
		href  string
		icon  string
		group string
	}
	allBlocks := []blockNav{
		{"metrics", "Metrics", "/metrics", "bar-chart", "Watch"},
		{"traces", "Traces", "/traces", "activity", "Watch"},
		{"habits", "Habits", "/habits", "flame", "Watch"},
		{"sprint", "Sprints", "/sprints", "zap", "Sprints"},
		{"adr", "ADRs", "/adrs", "file-text", "Sprints"},
		{"posts", "Cadence", "/cadence", "calendar", "Cadence"},
		{"logs", "Logbook", "/logs", "book", "Cadence"},
		{"todo", "Todos", "/todos", "check-square", "Act"},
		{"review", "Weekly Review", "/review", "clipboard", "Act"},
	}

	for _, b := range allBlocks {
		if enabledKeys[b.key] {
			items = append(items, SidebarItem{
				Key:   b.key,
				Name:  b.name,
				Href:  b.href,
				Icon:  b.icon,
				Group: b.group,
				Count: counts[b.key],
			})
		}
	}

	items = append(items,
		SidebarItem{Key: "new", Name: "New entry", Href: "/new", Icon: "plus-circle", Group: "Act"},
		SidebarItem{Key: "settings", Name: "Settings", Href: "/settings", Icon: "settings", Group: "Act"},
	)

	return items, enabledSlice
}

// sidebarCounts returns per-block record counts for sidebar badges.
func (h *Handlers) sidebarCounts(ctx context.Context, userID int64) map[string]int {
	db := h.svc.Store().DB()
	counts := make(map[string]int)

	var n int
	if db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sprints`).Scan(&n) == nil && n > 0 {
		counts["sprint"] = n
	}
	n = 0
	if db.QueryRowContext(ctx, `SELECT COUNT(*) FROM adrs`).Scan(&n) == nil && n > 0 {
		counts["adr"] = n
	}
	n = 0
	if db.QueryRowContext(ctx, `SELECT COUNT(*) FROM todos WHERE user_id = ? AND status = 'open'`, userID).Scan(&n) == nil && n > 0 {
		counts["todo"] = n
	}
	n = 0
	if db.QueryRowContext(ctx, `SELECT COUNT(*) FROM habits WHERE user_id = ? AND archived = 0`, userID).Scan(&n) == nil && n > 0 {
		counts["habits"] = n
	}
	return counts
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// --- Overview -------------------------------------------------------------

type overviewData struct {
	pageBase
	Snapshot service.TodaySnapshot
	Cadence  float64
	Streaks  service.Streaks
	ShipRate service.ShipRateResult
}

func (h *Handlers) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	snap, err := h.svc.TodaySnapshot(ctx)
	if err != nil {
		h.serverError(w, err)
		return
	}
	cadence, err := h.svc.CadenceRate(ctx, 30)
	if err != nil {
		h.serverError(w, err)
		return
	}
	streaks, err := h.svc.Streaks(ctx, 365)
	if err != nil {
		h.serverError(w, err)
		return
	}
	shipRate, err := h.svc.ShipRate(ctx)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, "overview", overviewData{
		pageBase: h.base("Overview", "overview"),
		Snapshot: snap, Cadence: cadence, Streaks: streaks, ShipRate: shipRate,
	})
}

// --- Sprints --------------------------------------------------------------

type sprintListData struct {
	pageBase
	Sprints []domain.Sprint
}

func (h *Handlers) handleSprintList(w http.ResponseWriter, r *http.Request) {
	sprints, err := h.svc.ListSprints(r.Context())
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, "sprints", sprintListData{pageBase: h.base("Sprints", "sprint"), Sprints: sprints})
}

type sprintDetailData struct {
	pageBase
	Sprint       domain.Sprint
	Checklist    map[domain.Phase][]domain.ChecklistItem
	Logs         []domain.DailyLog
	Health       service.PhaseHealth
	Unchecked    int
	Trace        []service.PhaseSpan
	ADRs         []domain.ADR   // block-gated: only if "adr" enabled
	SprintADRs   []domain.ADR   // ADRs linked to this sprint
	HealthBanner sprintHealthBanner
}

// sprintHealthBanner carries the computed health status for the alert bar.
type sprintHealthBanner struct {
	Severity string
	Message  string
	Show     bool
}

func (h *Handlers) handleSprintDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sp, err := h.svc.GetSprint(ctx, id)
	if errors.Is(err, service.ErrNotFound) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.serverError(w, err)
		return
	}
	items, err := h.svc.Checklist(ctx, id)
	if err != nil {
		h.serverError(w, err)
		return
	}
	byPhase := map[domain.Phase][]domain.ChecklistItem{}
	unchecked := 0
	totalItems := 0
	doneItems := 0
	for _, it := range items {
		byPhase[it.Phase] = append(byPhase[it.Phase], it)
		totalItems++
		if it.IsDone {
			doneItems++
		}
		if it.Phase == sp.CurrentPhase && !it.IsDone {
			unchecked++
		}
	}
	logs, err := h.svc.ListLogsBySprint(ctx, id)
	if err != nil {
		h.serverError(w, err)
		return
	}
	health := service.HealthGreen
	switch {
	case unchecked > 2:
		health = service.HealthRed
	case unchecked == 2:
		health = service.HealthAmber
	}
	trace, err := h.svc.SprintTrace(ctx, id)
	if err != nil {
		h.serverError(w, err)
		return
	}

	// Compute health banner using checklist progress as proxy for deliverables.
	banner := computeSprintHealthBanner(sp, totalItems, doneItems)

	// Load sprint-linked ADRs if the adr block is enabled.
	pb := h.base("Sprint · "+sp.SkillName, "sprint")
	var sprintADRs []domain.ADR
	for _, key := range pb.EnabledBlocks {
		if key == "adr" {
			allADRs, _ := h.svc.ListADRs(ctx)
			for _, a := range allADRs {
				if a.SprintID != nil && *a.SprintID == id {
					sprintADRs = append(sprintADRs, a)
				}
			}
			break
		}
	}

	h.render(w, "sprint_detail", sprintDetailData{
		pageBase:     pb,
		Sprint:       sp,
		Checklist:    byPhase,
		Logs:         logs,
		Health:       health,
		Unchecked:    unchecked,
		Trace:        trace,
		SprintADRs:   sprintADRs,
		HealthBanner: banner,
	})
}

// computeSprintHealthBanner returns a health banner based on sprint age and
// checklist completion (used as a proxy for deliverables until the deliverables
// system is fully wired to the creation form).
func computeSprintHealthBanner(sp domain.Sprint, total, done int) sprintHealthBanner {
	if sp.Status != domain.SprintActive {
		return sprintHealthBanner{}
	}

	// If no dates set, fall back to a simple checklist-only message.
	startStr := ""
	if sp.StartsOn != nil {
		startStr = *sp.StartsOn
	} else if sp.StartedOn != nil {
		startStr = *sp.StartedOn
	}

	var elapsed float64
	if startStr != "" {
		durationDays := 7 // default weekly sprint
		if sp.DurationDays != nil {
			durationDays = *sp.DurationDays
		}
		daysSinceStart := daysSince(startStr)
		if durationDays > 0 {
			elapsed = float64(daysSinceStart) / float64(durationDays)
		}
	}

	var progress float64
	if total > 0 {
		progress = float64(done) / float64(total)
	}

	switch {
	case elapsed > 1.0 && progress < 1.0:
		return sprintHealthBanner{
			Severity: "critical",
			Message:  "Sprint overdue — mark complete or extend before starting a new one.",
			Show:     true,
		}
	case elapsed >= 0.8 && progress < 0.5:
		return sprintHealthBanner{
			Severity: "critical",
			Message:  "Sprint ending soon · " + strconv.Itoa(total-done) + " checklist items still incomplete.",
			Show:     true,
		}
	case elapsed >= 0.5 && progress < 0.25:
		return sprintHealthBanner{
			Severity: "warning",
			Message:  "Sprint halfway through — only " + strconv.Itoa(int(progress*100)) + "% of checklist complete. Pick up the pace.",
			Show:     true,
		}
	default:
		if total > 0 {
			return sprintHealthBanner{
				Severity: "success",
				Message:  "On track · " + strconv.Itoa(done) + "/" + strconv.Itoa(total) + " checklist items complete.",
				Show:     true,
			}
		}
		return sprintHealthBanner{}
	}
}

// daysSince returns days elapsed since a YYYY-MM-DD date string.
func daysSince(dateStr string) int {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0
	}
	return int(time.Since(t).Hours() / 24)
}

// --- Cadence --------------------------------------------------------------

type cadenceData struct {
	pageBase
	Heatmap []service.CadenceCell
	Posts   []domain.Post
	Rate    float64
}

func (h *Handlers) handleCadence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cells, err := h.svc.CadenceHeatmap(ctx, 90)
	if err != nil {
		h.serverError(w, err)
		return
	}
	posts, err := h.svc.ListPosts(ctx, 60)
	if err != nil {
		h.serverError(w, err)
		return
	}
	rate, err := h.svc.CadenceRate(ctx, 30)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, "cadence", cadenceData{
		pageBase: h.base("Cadence", "posts"),
		Heatmap:  cells, Posts: posts, Rate: rate,
	})
}

type postDetailData struct {
	pageBase
	Post         domain.Post
	ADRs         []domain.ADR
	WeekMaterial []domain.DailyLog // for recap posts: the week's daily logs
}

func (h *Handlers) handlePostDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	post, err := h.svc.GetPost(ctx, id)
	if errors.Is(err, service.ErrNotFound) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.serverError(w, err)
		return
	}
	adrs, err := h.svc.ListADRs(ctx)
	if err != nil {
		h.serverError(w, err)
		return
	}
	data := postDetailData{
		pageBase: h.base("Post · "+post.PostDate, "posts"), Post: post, ADRs: adrs,
	}
	// Sunday recaps draw from the week's daily logs (spec 05): surface them as
	// raw material for drafting.
	if post.PostType == domain.PostRecap {
		mat, err := h.svc.WeekMaterial(ctx, post.SprintID, post.PostDate)
		if err != nil {
			h.serverError(w, err)
			return
		}
		data.WeekMaterial = mat
	}
	h.render(w, "post_detail", data)
}

// --- Logbook --------------------------------------------------------------

type logbookData struct {
	pageBase
	Events []domain.CareerEvent
	Logs   []domain.DailyLog
}

func (h *Handlers) handleLogbook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	events, err := h.svc.ListEvents(ctx, 200)
	if err != nil {
		h.serverError(w, err)
		return
	}
	logs, err := h.svc.ListLogs(ctx, 200)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, "logbook", logbookData{pageBase: h.base("Logbook", "logs"), Events: events, Logs: logs})
}

// --- ADRs -----------------------------------------------------------------

type adrListData struct {
	pageBase
	ADRs []domain.ADR
}

func (h *Handlers) handleADRList(w http.ResponseWriter, r *http.Request) {
	adrs, err := h.svc.ListADRs(r.Context())
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, "adrs", adrListData{pageBase: h.base("Architecture Decision Records", "adr"), ADRs: adrs})
}

// adrDetailData carries the single-ADR view data.
type adrDetailData struct {
	pageBase
	ADR domain.ADR
}

func (h *Handlers) handleADRDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	adr, err := h.svc.Store().GetADR(ctx, id)
	if errors.Is(err, service.ErrNotFound) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.serverError(w, err)
		return
	}
	title := "ADR-" + strconv.Itoa(adr.Number) + ": " + adr.Title
	h.render(w, "adr_detail", adrDetailData{pageBase: h.base(title, "adr"), ADR: adr})
}

// --- Metrics --------------------------------------------------------------

type metricsData struct {
	pageBase
	Heatmap        []service.CadenceCell
	Cadence7       float64
	Cadence30      float64
	Cadence90      float64
	Streaks        service.Streaks
	ShipRate       service.ShipRateResult
	TierMix        map[domain.Tier]int
	TierGoals      []service.TierGoalProgress
	PostCounts     []service.DayCount
	LogCounts      []service.DayCount
	Uptime         service.SprintUptime
	Stats          service.ContentStats
	RecentActivity []domain.CareerEvent
}

func (h *Handlers) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c7, _ := h.svc.CadenceRate(ctx, 7)
	c30, _ := h.svc.CadenceRate(ctx, 30)
	c90, _ := h.svc.CadenceRate(ctx, 90)
	streaks, err := h.svc.Streaks(ctx, 365)
	if err != nil {
		h.serverError(w, err)
		return
	}
	shipRate, err := h.svc.ShipRate(ctx)
	if err != nil {
		h.serverError(w, err)
		return
	}
	mix, err := h.svc.TierMix(ctx, 90)
	if err != nil {
		h.serverError(w, err)
		return
	}
	cells, err := h.svc.CadenceHeatmap(ctx, 90)
	if err != nil {
		h.serverError(w, err)
		return
	}
	tierGoals, _ := h.svc.WeeklyTierGoals(ctx)
	postCounts, _ := h.svc.DailyPostCounts(ctx, 30)
	logCounts, _ := h.svc.DailyLogCounts(ctx, 30)
	uptime, _ := h.svc.CurrentSprintUptime(ctx)
	stats, _ := h.svc.ContentStats(ctx)
	activity, _ := h.svc.RecentActivity(ctx, 8)

	h.render(w, "metrics", metricsData{
		pageBase:       h.base("Metrics", "metrics"),
		Heatmap:        cells,
		Cadence7:       c7,
		Cadence30:      c30,
		Cadence90:      c90,
		Streaks:        streaks,
		ShipRate:       shipRate,
		TierMix:        mix,
		TierGoals:      tierGoals,
		PostCounts:     postCounts,
		LogCounts:      logCounts,
		Uptime:         uptime,
		Stats:          stats,
		RecentActivity: activity,
	})
}

// --- New entry hub --------------------------------------------------------

type newData struct {
	pageBase
	Sprints []domain.Sprint
	ADRs    []domain.ADR
}

func (h *Handlers) handleNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sprints, err := h.svc.ListSprints(ctx)
	if err != nil {
		h.serverError(w, err)
		return
	}
	adrs, err := h.svc.ListADRs(ctx)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, "new", newData{pageBase: h.base("New entry", "new"), Sprints: sprints, ADRs: adrs})
}

// --- Settings (read-only config view) -------------------------------------

// BlockCard represents a block with toggle state for the settings page.
type BlockCard struct {
	Key         string
	Name        string
	Description string
	Enabled     bool
	Metric      string // e.g. "14 ADRs"
}

type settingsData struct {
	pageBase
	Meta       Meta
	User       *domain.User
	Blocks     []BlockCard
	Msg        string
	ErrMsg     string
}

func (h *Handlers) handleSettings(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.currentUserID(r)

	data := settingsData{
		pageBase: h.base("Settings", "settings"),
		Meta:     h.meta,
		Msg:      r.URL.Query().Get("msg"),
		ErrMsg:   r.URL.Query().Get("err"),
	}

	// Load user profile.
	if userID > 0 {
		user, err := h.auth.GetUser(r.Context(), userID)
		if err == nil {
			data.User = &user
		}
	}

	// Load block cards with toggle state and metrics.
	if userID > 0 {
		metrics, _ := h.block.Metrics(r.Context(), userID)
		for _, def := range block.Registry() {
			enabled, _ := h.block.IsEnabled(r.Context(), userID, def.Key)
			card := BlockCard{
				Key:         def.Key,
				Name:        def.Name,
				Description: def.Description,
				Enabled:     enabled,
				Metric:      metrics[def.Key],
			}
			data.Blocks = append(data.Blocks, card)
		}
	}

	h.render(w, "settings", data)
}

// --- Traces ---------------------------------------------------------------

type tracesData struct {
	pageBase
	Sprints []domain.Sprint
	Traces  map[int64][]service.PhaseSpan // sprintID -> spans
}

func (h *Handlers) handleTraces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sprints, err := h.svc.ListSprints(ctx)
	if err != nil {
		h.serverError(w, err)
		return
	}
	traces := make(map[int64][]service.PhaseSpan)
	for _, sp := range sprints {
		spans, err := h.svc.SprintTrace(ctx, sp.ID)
		if err == nil {
			traces[sp.ID] = spans
		}
	}
	h.render(w, "traces", tracesData{
		pageBase: h.base("Traces", "traces"),
		Sprints:  sprints,
		Traces:   traces,
	})
}

// --- API: /api/me (localStorage hydration) --------------------------------

func (h *Handlers) handleAPIMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		apiJSON(w, http.StatusUnauthorized, map[string]any{"error": "not authenticated"})
		return
	}
	user, err := h.auth.GetUser(r.Context(), userID)
	if err != nil {
		apiJSON(w, http.StatusInternalServerError, map[string]any{"error": "user not found"})
		return
	}
	enabledDefs, _ := h.block.Enabled(r.Context(), userID)
	keys := make([]string, 0, len(enabledDefs))
	for _, d := range enabledDefs {
		keys = append(keys, d.Key)
	}
	apiJSON(w, http.StatusOK, map[string]any{
		"username":       user.Username,
		"displayName":    user.DisplayName,
		"avatarInitials": user.AvatarInitials,
		"role":           string(user.Role),
		"enabledBlocks":  keys,
	})
}

// --- Block-disabled page --------------------------------------------------

func (h *Handlers) handleBlockDisabled(w http.ResponseWriter, blockKey string) {
	def, ok := block.ByKey(blockKey)
	name := blockKey
	if ok {
		name = def.Name
	}
	w.WriteHeader(http.StatusNotFound)
	h.render(w, "block_disabled", struct {
		pageBase
		BlockName string
		BlockKey  string
	}{
		pageBase:  h.base("Block disabled", ""),
		BlockName: name,
		BlockKey:  blockKey,
	})
}

// blockGate returns a handler that checks whether the named block is enabled
// before invoking next. If disabled it renders the block-disabled page.
func (h *Handlers) blockGate(blockKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := h.currentUserID(r)
		enabled, err := h.block.IsEnabled(r.Context(), userID, blockKey)
		if err != nil || !enabled {
			h.handleBlockDisabled(w, blockKey)
			return
		}
		next(w, r)
	}
}

// ensure time is used (daysSince already references it)
var _ = time.Now
