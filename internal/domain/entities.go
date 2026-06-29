package domain

import "time"

// Sprint is one "learn a skill, ship a microapp" cycle. Phase 2 adds bounded
// durations and a title/goal, extending the month-long model.
type Sprint struct {
	ID                int64        `json:"id"`
	MonthLabel        string       `json:"month_label"`        // "2026-07" or "Sprint 7"
	SkillName         string       `json:"skill_name"`         // skill of the month
	SkillRationale    string       `json:"skill_rationale"`    // why this skill / what it unlocks
	MicroappOneLiner  string       `json:"microapp_one_liner"` // the project, in one sentence
	CoreFeature       string       `json:"core_feature"`       // the feature that proves the skill
	OutOfScope        string       `json:"out_of_scope"`       // explicitly out of scope
	DeployPlatform    string       `json:"deploy_platform"`    // Vercel/Netlify/Railway/Fly.io/Render/Other
	CurrentPhase      Phase        `json:"current_phase"`      // 1-4
	Status            SprintStatus `json:"status"`             // planned/active/shipped/abandoned
	LiveURL           *string      `json:"live_url,omitempty"` // required before status=shipped
	DeclarationPostID *int64       `json:"declaration_post_id,omitempty"`

	// Phase 2 additions.
	Title        *string `json:"title,omitempty"`         // short sprint title
	Goal         *string `json:"goal,omitempty"`          // 1-2 sentence objective
	DurationDays *int    `json:"duration_days,omitempty"` // 3/5/7/14
	StartsOn     *string `json:"starts_on,omitempty"`     // YYYY-MM-DD local
	EndsOn       *string `json:"ends_on,omitempty"`       // YYYY-MM-DD, derived starts_on + duration

	// Retro (filled after shipping).
	RetroWorked      string `json:"retro_worked"`
	RetroDifferently string `json:"retro_differently"`
	RetroLearned     string `json:"retro_learned"`
	RetroLiveLink    string `json:"retro_live_link"`

	StartedOn *string   `json:"started_on,omitempty"` // YYYY-MM-DD (phase-1 compat)
	EndedOn   *string   `json:"ended_on,omitempty"`   // YYYY-MM-DD (phase-1 compat)
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsShipped reports whether the sprint has been shipped.
func (s Sprint) IsShipped() bool { return s.Status == SprintShipped }

// HasLiveURL reports whether a non-empty live URL is set.
func (s Sprint) HasLiveURL() bool { return s.LiveURL != nil && *s.LiveURL != "" }

// ChecklistItem is a single phase gate seeded per sprint from weekly-checklist.md.
type ChecklistItem struct {
	ID        int64      `json:"id"`
	SprintID  int64      `json:"sprint_id"`
	Phase     Phase      `json:"phase"`
	Label     string     `json:"label"`
	IsDone    bool       `json:"is_done"`
	SortOrder int        `json:"sort_order"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// DailyLog is a per-day build note. It is the raw material recaps are built from.
type DailyLog struct {
	ID              int64            `json:"id"`
	SprintID        *int64           `json:"sprint_id,omitempty"`
	LogDate         string           `json:"log_date"` // YYYY-MM-DD (local day)
	WorkedOn        string           `json:"worked_on"`
	WhatHappened    string           `json:"what_happened"`
	Insight         string           `json:"insight"`
	NextUp          string           `json:"next_up"`
	Blocker         string           `json:"blocker"`
	BlockerDecision *BlockerDecision `json:"blocker_decision,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// Post is one day's content update (daily or Sunday recap). A post fans out to
// up to three tiers.
type Post struct {
	ID            int64     `json:"id"`
	SprintID      *int64    `json:"sprint_id,omitempty"`
	SourceLogID   *int64    `json:"source_log_id,omitempty"`
	PostDate      string    `json:"post_date"` // YYYY-MM-DD (local day)
	PostType      PostType  `json:"post_type"`
	Title         string    `json:"title"`
	IsDeclaration bool      `json:"is_declaration"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Tiers is optionally populated by the store/service when a post is loaded
	// with its tiers. Order follows AllTiers().
	Tiers []PostTier `json:"tiers,omitempty"`
}

// PostTier is one of the three matched versions of a post, tracked independently.
type PostTier struct {
	ID          int64      `json:"id"`
	PostID      int64      `json:"post_id"`
	Tier        Tier       `json:"tier"`
	Status      TierStatus `json:"status"`
	Content     string     `json:"content"`
	URL         *string    `json:"url,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	VisualKind  VisualKind `json:"visual_kind"`
	ADRID       *int64     `json:"adr_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsPublished reports whether the tier is live.
func (pt PostTier) IsPublished() bool { return pt.Status == TierPublished }

// ADR is an Architecture Decision Record (adr-lite-template.md), LinkedIn-attachable.
type ADR struct {
	ID           int64     `json:"id"`
	SprintID     *int64    `json:"sprint_id,omitempty"`
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	Status       ADRStatus `json:"status"`
	DecidedOn    *string   `json:"decided_on,omitempty"` // YYYY-MM-DD
	Problem      string    `json:"problem"`
	Options      string    `json:"options"`
	Decision     string    `json:"decision"`
	Why          string    `json:"why"`
	Consequences string    `json:"consequences"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CareerEvent is an append-only audit/log entry written on every mutation,
// powering the SRE-style "logs" view. Source records form vs chat vs system vs api.
type CareerEvent struct {
	ID         int64       `json:"id"`
	OccurredAt time.Time   `json:"occurred_at"`
	Kind       string      `json:"kind"` // e.g. "sprint.created", "post.published"
	Source     EventSource `json:"source"`
	SprintID   *int64      `json:"sprint_id,omitempty"`
	PostID     *int64      `json:"post_id,omitempty"`
	Summary    string      `json:"summary"`
	Detail     string      `json:"detail"`
}

// --- Phase 2 entities ---

// User is the single operator account.
type User struct {
	ID              int64     `json:"id"`
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	AvatarInitials  string    `json:"avatar_initials"`
	PasswordHash    string    `json:"-"` // never serialize
	Role            UserRole  `json:"role"`
	RoleOther       *string   `json:"role_other,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Session is a cookie-based login session.
type Session struct {
	ID         string    `json:"id"` // opaque cookie value
	UserID     int64     `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Remember   bool      `json:"remember"`
}

// APIKey is a REST API authentication key.
type APIKey struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Label      string     `json:"label"`
	KeyHash    string     `json:"-"` // never serialize
	KeyPrefix  string     `json:"key_prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// OnboardingState tracks the first-run wizard progress.
type OnboardingState struct {
	UserID    int64            `json:"user_id"`
	Status    OnboardingStatus `json:"status"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// UserBlock records whether a block is enabled for a user.
type UserBlock struct {
	UserID    int64     `json:"user_id"`
	BlockKey  string    `json:"block_key"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Deliverable is a user-defined "definition of done" item for a sprint.
type Deliverable struct {
	ID        int64      `json:"id"`
	SprintID  int64      `json:"sprint_id"`
	Text      string     `json:"text"`
	IsDone    bool       `json:"is_done"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
	SortOrder int        `json:"sort_order"`
	CreatedAt time.Time  `json:"created_at"`
}

// SprintHealth is the computed health state of a sprint.
type SprintHealth struct {
	Severity        Severity `json:"severity"`
	ElapsedPct      float64  `json:"elapsed_pct"`
	DeliverablesPct float64  `json:"deliverables_pct"`
	DaysRemaining   int      `json:"days_remaining"`
	IncompleteCount int      `json:"incomplete_count"`
	DoneCount       int      `json:"done_count"`
	TotalCount      int      `json:"total_count"`
	Message         string   `json:"message"`
}

// SprintTemplate is a reusable sprint shape.
type SprintTemplate struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Name         string    `json:"name"`
	DurationDays *int      `json:"duration_days,omitempty"`
	PhaseNotes   *string   `json:"phase_notes,omitempty"`   // JSON
	Deliverables *string   `json:"deliverables,omitempty"`  // JSON array of stub strings
	CreatedAt    time.Time `json:"created_at"`
}

// Habit is a user-defined binary daily habit.
type Habit struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	Name          string    `json:"name"`
	Icon          *string   `json:"icon,omitempty"`
	SprintLinked  bool      `json:"sprint_linked"`
	Archived      bool      `json:"archived"`
	CreatedAt     time.Time `json:"created_at"`
}

// HabitEntry records a habit completed on a given date.
type HabitEntry struct {
	HabitID   int64  `json:"habit_id"`
	EntryDate string `json:"entry_date"` // YYYY-MM-DD local
}

// WeeklyReview is a prompted end-of-week reflection.
type WeeklyReview struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	ISOWeek      string    `json:"iso_week"` // "2026-W26"
	WhatShipped  *string   `json:"what_shipped,omitempty"`
	WhatSlipped  *string   `json:"what_slipped,omitempty"`
	CarryForward *string   `json:"carry_forward,omitempty"`
	OneLearning  *string   `json:"one_learning,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Todo is a task with priority, optional due date, and optional sprint linkage.
type Todo struct {
	ID        int64        `json:"id"`
	UserID    int64        `json:"user_id"`
	SprintID  *int64       `json:"sprint_id,omitempty"`
	Text      string       `json:"text"`
	Priority  TodoPriority `json:"priority"`
	Status    TodoStatus   `json:"status"`
	DueOn     *string      `json:"due_on,omitempty"` // YYYY-MM-DD local
	CreatedAt time.Time    `json:"created_at"`
	DoneAt    *time.Time   `json:"done_at,omitempty"`
}

// MetricPoint is an API-pushed metric data point.
type MetricPoint struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Value      float64   `json:"value"`
	Tags       *string   `json:"tags,omitempty"` // JSON object
	OccurredAt time.Time `json:"occurred_at"`
}

// TraceSpan is an API-pushed sprint span for the waterfall.
type TraceSpan struct {
	ID         int64     `json:"id"`
	SprintID   *int64    `json:"sprint_id,omitempty"`
	Phase      *int      `json:"phase,omitempty"`
	Name       string    `json:"name"`
	DurationMS int64     `json:"duration_ms"`
	StartedAt  time.Time `json:"started_at"`
}
