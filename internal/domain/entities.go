package domain

import "time"

// Sprint is one month-long "learn a skill, ship a microapp" cycle.
// Mirrors the sprints table and sprint-plan-template.md.
type Sprint struct {
	ID                int64        `json:"id"`
	MonthLabel        string       `json:"month_label"`         // "2026-07" or "Sprint 7"
	SkillName         string       `json:"skill_name"`          // skill of the month
	SkillRationale    string       `json:"skill_rationale"`     // why this skill / what it unlocks
	MicroappOneLiner  string       `json:"microapp_one_liner"`  // the project, in one sentence
	CoreFeature       string       `json:"core_feature"`        // the feature that proves the skill
	OutOfScope        string       `json:"out_of_scope"`        // explicitly out of scope
	DeployPlatform    string       `json:"deploy_platform"`     // Vercel/Netlify/Railway/Fly.io/Render/Other
	CurrentPhase      Phase        `json:"current_phase"`       // 1-4
	Status            SprintStatus `json:"status"`              // planned/active/shipped/abandoned
	LiveURL           *string      `json:"live_url,omitempty"`  // required before status=shipped
	DeclarationPostID *int64       `json:"declaration_post_id,omitempty"`

	// Retro (filled after shipping).
	RetroWorked      string `json:"retro_worked"`
	RetroDifferently string `json:"retro_differently"`
	RetroLearned     string `json:"retro_learned"`
	RetroLiveLink    string `json:"retro_live_link"`

	StartedOn *string   `json:"started_on,omitempty"` // YYYY-MM-DD
	EndedOn   *string   `json:"ended_on,omitempty"`   // YYYY-MM-DD
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
// powering the SRE-style "logs" view. Source records form vs chat vs system.
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
