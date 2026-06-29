// Package domain holds the core entities and enums shared across every layer of
// local-scava. These types are a direct encoding of the career-growth skills
// (Monthly Skill Sprint + Three-Tier Content Cadence) and mirror the database
// schema in /schema/schema.sql.
//
// Layer rule: domain is a leaf package. It imports nothing from this module and
// is imported by store, services, and web. It contains data and pure helpers
// only — no SQL, no HTTP.
package domain

// Phase is one of the four Monthly Skill Sprint stages. Phase is set manually
// (sprints drift; it is never derived from the calendar date).
type Phase int

const (
	PhaseScopeDeclare   Phase = 1 // Week 1: scope the skill + microapp, declare publicly
	PhaseBuild          Phase = 2 // Week 2: make the core feature work locally
	PhasePolishDocument Phase = 3 // Week 3: README, tests, clean naming, UI pass
	PhaseDeployShowcase Phase = 4 // Week 4: live URL + recap + portfolio update
)

// Valid reports whether p is a known phase.
func (p Phase) Valid() bool { return p >= PhaseScopeDeclare && p <= PhaseDeployShowcase }

// Label returns the human-readable phase name used across the UI.
func (p Phase) Label() string {
	switch p {
	case PhaseScopeDeclare:
		return "Scope & Declare"
	case PhaseBuild:
		return "Build"
	case PhasePolishDocument:
		return "Polish & Document"
	case PhaseDeployShowcase:
		return "Deploy & Showcase"
	default:
		return "Unknown"
	}
}

// AllPhases returns the four phases in order. Useful for rendering the stepper.
func AllPhases() []Phase {
	return []Phase{PhaseScopeDeclare, PhaseBuild, PhasePolishDocument, PhaseDeployShowcase}
}

// SprintStatus is the lifecycle state of a sprint.
type SprintStatus string

const (
	SprintPlanned   SprintStatus = "planned"
	SprintActive    SprintStatus = "active"
	SprintShipped   SprintStatus = "shipped"
	SprintAbandoned SprintStatus = "abandoned"
)

// Valid reports whether s is a known sprint status.
func (s SprintStatus) Valid() bool {
	switch s {
	case SprintPlanned, SprintActive, SprintShipped, SprintAbandoned:
		return true
	default:
		return false
	}
}

// Tier is one of the three matched versions of a post.
type Tier string

const (
	TierBlog     Tier = "blog"     // deep, teaching, long-form (personal blog)
	TierLinkedIn Tier = "linkedin" // summary, credibility, link-back
	TierX        Tier = "x"        // punchy, brag, 1-4 lines
)

// Valid reports whether t is a known tier.
func (t Tier) Valid() bool {
	switch t {
	case TierBlog, TierLinkedIn, TierX, TierInstagram, TierTikTok:
		return true
	default:
		return false
	}
}

// AllTiers returns the three tiers in canonical display order.
func AllTiers() []Tier { return []Tier{TierBlog, TierLinkedIn, TierX} }

// Label returns a display label for a tier.
func (t Tier) Label() string {
	switch t {
	case TierBlog:
		return "Blog"
	case TierLinkedIn:
		return "LinkedIn"
	case TierX:
		return "X"
	default:
		return "Unknown"
	}
}

// IsCredibility reports whether the tier counts toward "did I post today" for
// cadence purposes. The X tier is optional; blog and linkedin are the
// credibility tiers.
func (t Tier) IsCredibility() bool { return t == TierBlog || t == TierLinkedIn }

// TierStatus is the publication state of a single post tier.
type TierStatus string

const (
	TierNone      TierStatus = "none"      // not started
	TierDrafted   TierStatus = "drafted"   // content written, not published
	TierPublished TierStatus = "published" // live, has a URL
)

// Valid reports whether ts is a known tier status.
func (ts TierStatus) Valid() bool {
	switch ts {
	case TierNone, TierDrafted, TierPublished:
		return true
	default:
		return false
	}
}

// VisualKind is the LinkedIn-tier visual attachment choice. The skill nudges
// toward an ADR or an animated data-flow diagram over a flat screenshot.
type VisualKind string

const (
	VisualNone       VisualKind = "none"
	VisualADR        VisualKind = "adr"
	VisualDiagram    VisualKind = "diagram"
	VisualScreenshot VisualKind = "screenshot"
)

// Valid reports whether v is a known visual kind.
func (v VisualKind) Valid() bool {
	switch v {
	case VisualNone, VisualADR, VisualDiagram, VisualScreenshot:
		return true
	default:
		return false
	}
}

// PostType distinguishes a daily update from a Sunday recap.
type PostType string

const (
	PostDaily PostType = "daily"
	PostRecap PostType = "recap"
)

// Valid reports whether pt is a known post type.
func (pt PostType) Valid() bool {
	switch pt {
	case PostDaily, PostRecap:
		return true
	default:
		return false
	}
}

// BlockerDecision is how a build blocker was resolved (sprint Phase-2 rule).
type BlockerDecision string

const (
	BlockerSolve      BlockerDecision = "solve"
	BlockerWorkaround BlockerDecision = "workaround"
	BlockerCut        BlockerDecision = "cut"
)

// Valid reports whether bd is a known blocker decision.
func (bd BlockerDecision) Valid() bool {
	switch bd {
	case BlockerSolve, BlockerWorkaround, BlockerCut:
		return true
	default:
		return false
	}
}

// ADRStatus is the lifecycle state of an Architecture Decision Record.
type ADRStatus string

const (
	ADRProposed   ADRStatus = "proposed"
	ADRDecided    ADRStatus = "decided"
	ADRSuperseded ADRStatus = "superseded"
)

// Valid reports whether as is a known ADR status.
func (as ADRStatus) Valid() bool {
	switch as {
	case ADRProposed, ADRDecided, ADRSuperseded:
		return true
	default:
		return false
	}
}

// EventSource records who triggered a career event: a web form, the chat
// bridge, the system itself, or the REST API.
type EventSource string

const (
	SourceForm   EventSource = "form"
	SourceChat   EventSource = "chat"
	SourceSystem EventSource = "system"
	SourceAPI    EventSource = "api"
)

// Valid reports whether es is a known event source.
func (es EventSource) Valid() bool {
	switch es {
	case SourceForm, SourceChat, SourceSystem, SourceAPI:
		return true
	default:
		return false
	}
}

// Tier extensions for phase 2 platforms.
const (
	TierInstagram Tier = "instagram"
	TierTikTok   Tier = "tiktok"
)

// OnboardingStatus tracks the wizard progress.
type OnboardingStatus string

const (
	OnboardingPending    OnboardingStatus = "pending"
	OnboardingRoleDone   OnboardingStatus = "role_done"
	OnboardingBlocksDone OnboardingStatus = "blocks_done"
	OnboardingComplete   OnboardingStatus = "complete"
)

// Valid reports whether os is a known onboarding status.
func (os OnboardingStatus) Valid() bool {
	switch os {
	case OnboardingPending, OnboardingRoleDone, OnboardingBlocksDone, OnboardingComplete:
		return true
	default:
		return false
	}
}

// UserRole is the role chosen at onboarding; used to suggest default blocks.
type UserRole string

const (
	RoleBackend    UserRole = "backend"
	RoleFrontend   UserRole = "frontend"
	RoleFullstack  UserRole = "fullstack"
	RoleDevOps     UserRole = "devops"
	RoleData       UserRole = "data"
	RoleIndie      UserRole = "indie"
	RoleManager    UserRole = "manager"
	RoleOther      UserRole = "other"
)

// Valid reports whether r is a known user role.
func (r UserRole) Valid() bool {
	switch r {
	case RoleBackend, RoleFrontend, RoleFullstack, RoleDevOps,
		RoleData, RoleIndie, RoleManager, RoleOther:
		return true
	default:
		return false
	}
}

// TodoStatus is the lifecycle state of a todo item.
type TodoStatus string

const (
	TodoOpen    TodoStatus = "open"
	TodoDone    TodoStatus = "done"
	TodoDropped TodoStatus = "dropped"
)

// Valid reports whether ts is a known todo status.
func (ts TodoStatus) Valid() bool {
	switch ts {
	case TodoOpen, TodoDone, TodoDropped:
		return true
	default:
		return false
	}
}

// TodoPriority is the urgency level of a todo.
type TodoPriority string

const (
	PriorityLow    TodoPriority = "low"
	PriorityNormal TodoPriority = "normal"
	PriorityHigh   TodoPriority = "high"
)

// Valid reports whether tp is a known todo priority.
func (tp TodoPriority) Valid() bool {
	switch tp {
	case PriorityLow, PriorityNormal, PriorityHigh:
		return true
	default:
		return false
	}
}

// Severity is the health severity of a sprint.
type Severity string

const (
	SeveritySuccess Severity = "success"
	SeverityWarning Severity = "warning"
	SeverityAlert   Severity = "alert"
	SeverityAlarm   Severity = "alarm"
	SeverityNudge   Severity = "nudge"
)
