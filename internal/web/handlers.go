package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/paulkinyatti/local-scava/internal/domain"
	"github.com/paulkinyatti/local-scava/internal/service"
)

// pageBase carries fields common to every page (for the layout/sidebar).
type pageBase struct {
	Title       string
	Nav         string // active sidebar key
	Today       string
	Flash       string
	HasActive   bool   // an active sprint exists (for the top status bar)
	ActiveID    int64  // active sprint's ID (for linking)
	ActiveSkill string // active sprint's skill name
	ActivePhase int    // active sprint's current phase number
}

func (h *Handlers) base(title, nav string) pageBase {
	pb := pageBase{Title: title, Nav: nav, Today: h.svc.Today()}
	if sp, err := h.svc.CurrentSprint(context.Background()); err == nil {
		pb.HasActive = true
		pb.ActiveID = sp.ID
		pb.ActiveSkill = sp.SkillName
		pb.ActivePhase = int(sp.CurrentPhase)
	}
	return pb
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
	h.render(w, "sprints", sprintListData{pageBase: h.base("Sprints", "sprints"), Sprints: sprints})
}

type sprintDetailData struct {
	pageBase
	Sprint    domain.Sprint
	Checklist map[domain.Phase][]domain.ChecklistItem
	Logs      []domain.DailyLog
	Health    service.PhaseHealth
	Unchecked int
	Trace     []service.PhaseSpan
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
	for _, it := range items {
		byPhase[it.Phase] = append(byPhase[it.Phase], it)
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
	h.render(w, "sprint_detail", sprintDetailData{
		pageBase: h.base("Sprint · "+sp.SkillName, "sprints"),
		Sprint:   sp, Checklist: byPhase, Logs: logs, Health: health, Unchecked: unchecked, Trace: trace,
	})
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
		pageBase: h.base("Cadence", "cadence"),
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
		pageBase: h.base("Post · "+post.PostDate, "cadence"), Post: post, ADRs: adrs,
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
	h.render(w, "logbook", logbookData{pageBase: h.base("Logbook", "logbook"), Events: events, Logs: logs})
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
	h.render(w, "adrs", adrListData{pageBase: h.base("ADRs", "adrs"), ADRs: adrs})
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

type settingsData struct {
	pageBase
	Meta Meta
}

func (h *Handlers) handleSettings(w http.ResponseWriter, r *http.Request) {
	h.render(w, "settings", settingsData{pageBase: h.base("Settings", "settings"), Meta: h.meta})
}
