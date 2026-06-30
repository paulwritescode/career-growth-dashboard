package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// --- Todos ---

// SprintChecklist groups a sprint's phase-gate items for display on /todos.
type SprintChecklist struct {
	Sprint    domain.Sprint
	ByPhase   map[domain.Phase][]domain.ChecklistItem
	PhaseKeys []domain.Phase
}

type todosData struct {
	pageBase
	Todos             []domain.Todo
	SprintChecklists  []SprintChecklist
}

func (h *Handlers) handleTodos(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	filter := r.URL.Query().Get("status")
	todos, err := h.svc.Store().ListTodos(ctx, userID, filter)
	if err != nil {
		h.serverError(w, err)
		return
	}

	// Load phase checklists per sprint (so users can tick them from /todos).
	sprints, _ := h.svc.ListSprints(ctx)
	var sprintChecklists []SprintChecklist
	for _, sp := range sprints {
		items, err := h.svc.Checklist(ctx, sp.ID)
		if err != nil || len(items) == 0 {
			continue
		}
		byPhase := map[domain.Phase][]domain.ChecklistItem{}
		for _, it := range items {
			byPhase[it.Phase] = append(byPhase[it.Phase], it)
		}
		phases := []domain.Phase{1, 2, 3, 4}
		sprintChecklists = append(sprintChecklists, SprintChecklist{
			Sprint:    sp,
			ByPhase:   byPhase,
			PhaseKeys: phases,
		})
	}

	h.render(w, "todos", todosData{
		pageBase:         h.base("Todos", "todo"),
		Todos:            todos,
		SprintChecklists: sprintChecklists,
	})
}

func (h *Handlers) handleTodoCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.serverError(w, err)
		return
	}
	text := r.FormValue("text")
	if text == "" {
		http.Redirect(w, r, "/todos", http.StatusSeeOther)
		return
	}
	priority := domain.TodoPriority(r.FormValue("priority"))
	if !priority.Valid() {
		priority = domain.PriorityNormal
	}
	var sprintID *int64
	if sp, err := h.svc.CurrentSprint(r.Context()); err == nil {
		sprintID = &sp.ID
	}

	_, err := h.svc.Store().CreateTodo(r.Context(), domain.Todo{
		UserID:   userID,
		SprintID: sprintID,
		Text:     text,
		Priority: priority,
		Status:   domain.TodoOpen,
	})
	if err != nil {
		h.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/todos", http.StatusSeeOther)
}

func (h *Handlers) handleTodoStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.serverError(w, err)
		return
	}
	status := domain.TodoStatus(r.FormValue("status"))
	if !status.Valid() {
		status = domain.TodoDone
	}
	if err := h.svc.Store().UpdateTodoStatus(r.Context(), id, status); err != nil {
		h.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/todos", http.StatusSeeOther)
}

func (h *Handlers) handleTodoDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.svc.Store().DeleteTodo(r.Context(), id)
	http.Redirect(w, r, "/todos", http.StatusSeeOther)
}

// --- Habits ---

type habitsData struct {
	pageBase
	Habits        []domain.Habit
	SprintHabits  []domain.Habit // sprint-linked habits (second strip)
	Today         string
	Entries       map[int64][]string // habitID -> list of dates with entries (last 30 days)
	Streaks       map[int64]int      // habitID -> current streak
	ActiveSprint  *domain.Sprint
}

func (h *Handlers) handleHabits(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	habits, err := h.svc.Store().ListHabits(ctx, userID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	today := h.svc.Today()
	from := time.Now().AddDate(0, 0, -30).Format("2006-01-02")

	entries := make(map[int64][]string)
	streaks := make(map[int64]int)
	for _, hab := range habits {
		e, _ := h.svc.Store().HabitEntries(ctx, hab.ID, from, today)
		entries[hab.ID] = e
		s, _ := h.svc.Store().HabitStreak(ctx, hab.ID, today)
		streaks[hab.ID] = s
	}

	// Split into sprint-linked and general habits.
	var sprintHabits []domain.Habit
	var generalHabits []domain.Habit
	for _, hab := range habits {
		if hab.SprintLinked {
			sprintHabits = append(sprintHabits, hab)
		} else {
			generalHabits = append(generalHabits, hab)
		}
	}

	// Get active sprint for context.
	var activeSprint *domain.Sprint
	if sp, err := h.svc.CurrentSprint(ctx); err == nil {
		activeSprint = &sp
	}

	h.render(w, "habits", habitsData{
		pageBase:      h.base("Habits", "habits"),
		Habits:        generalHabits,
		SprintHabits:  sprintHabits,
		Today:         today,
		Entries:       entries,
		Streaks:       streaks,
		ActiveSprint:  activeSprint,
	})
}

func (h *Handlers) handleHabitCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.serverError(w, err)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Redirect(w, r, "/habits", http.StatusSeeOther)
		return
	}
	_, err := h.svc.Store().CreateHabit(r.Context(), domain.Habit{
		UserID: userID,
		Name:   name,
	})
	if err != nil {
		h.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/habits", http.StatusSeeOther)
}

func (h *Handlers) handleHabitToggle(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	today := h.svc.Today()
	date := r.URL.Query().Get("date")
	if date == "" {
		date = today
	}
	_, err = h.svc.Store().ToggleHabitEntry(r.Context(), id, date)
	if err != nil {
		h.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/habits", http.StatusSeeOther)
}

func (h *Handlers) handleHabitArchive(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.svc.Store().ArchiveHabit(r.Context(), id)
	http.Redirect(w, r, "/habits", http.StatusSeeOther)
}

// --- Weekly Review ---

type reviewData struct {
	pageBase
	Review  domain.WeeklyReview
	ISOWeek string
}

func (h *Handlers) handleReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Current ISO week.
	year, week := time.Now().ISOWeek()
	isoWeek := fmt.Sprintf("%d-W%02d", year, week)

	review, err := h.svc.Store().GetWeeklyReview(r.Context(), userID, isoWeek)
	if err != nil {
		// Not found — create an empty one for the template.
		review = domain.WeeklyReview{UserID: userID, ISOWeek: isoWeek}
	}

	h.render(w, "review", reviewData{
		pageBase: h.base("Weekly Review", "review"),
		Review:   review,
		ISOWeek:  isoWeek,
	})
}

func (h *Handlers) handleReviewSave(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.serverError(w, err)
		return
	}

	year, week := time.Now().ISOWeek()
	isoWeek := fmt.Sprintf("%d-W%02d", year, week)

	shipped := r.FormValue("what_shipped")
	slipped := r.FormValue("what_slipped")
	carry := r.FormValue("carry_forward")
	learning := r.FormValue("one_learning")

	_, err := h.svc.Store().UpsertWeeklyReview(r.Context(), domain.WeeklyReview{
		UserID:       userID,
		ISOWeek:      isoWeek,
		WhatShipped:  &shipped,
		WhatSlipped:  &slipped,
		CarryForward: &carry,
		OneLearning:  &learning,
	})
	if err != nil {
		h.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/review?msg=saved", http.StatusSeeOther)
}

// --- API Docs ---

func (h *Handlers) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	h.render(w, "api_docs", h.base("API Documentation", "settings"))
}

// --- Command palette search ---

func (h *Handlers) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		apiJSON(w, http.StatusOK, map[string]any{"results": []any{}})
		return
	}

	userID, _ := h.currentUserID(r)
	var results []map[string]string

	// Search sprints.
	sprints, _ := h.svc.ListSprints(r.Context())
	for _, sp := range sprints {
		if containsCI(sp.SkillName, query) || containsCI(sp.MicroappOneLiner, query) {
			results = append(results, map[string]string{
				"block": "sprint",
				"title": sp.SkillName,
				"href":  fmt.Sprintf("/sprints/%d", sp.ID),
			})
		}
	}

	// Search ADRs.
	adrs, _ := h.svc.ListADRs(r.Context())
	for _, a := range adrs {
		if containsCI(a.Title, query) || containsCI(a.Decision, query) {
			results = append(results, map[string]string{
				"block": "adr",
				"title": fmt.Sprintf("ADR-%d: %s", a.Number, a.Title),
				"href":  fmt.Sprintf("/adrs/%d", a.ID),
			})
		}
	}

	// Search logs.
	logs, _ := h.svc.ListLogs(r.Context(), 50)
	for _, l := range logs {
		if containsCI(l.WorkedOn, query) || containsCI(l.Insight, query) {
			results = append(results, map[string]string{
				"block": "logs",
				"title": l.LogDate + ": " + l.WorkedOn,
				"href":  "/logs",
			})
		}
	}

	// Search todos.
	if userID > 0 {
		todos, _ := h.svc.Store().ListTodos(r.Context(), userID, "")
		for _, t := range todos {
			if containsCI(t.Text, query) {
				results = append(results, map[string]string{
					"block": "todo",
					"title": t.Text,
					"href":  "/todos",
				})
			}
		}
	}

	// Limit results.
	if len(results) > 20 {
		results = results[:20]
	}

	apiJSON(w, http.StatusOK, map[string]any{"results": results})
}

func containsCI(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) > 0 &&
		contains(toLower(haystack), toLower(needle))
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		} else {
			b[i] = c
		}
	}
	return string(b)
}

func contains(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
