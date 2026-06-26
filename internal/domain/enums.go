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
	case TierBlog, TierLinkedIn, TierX:
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
// bridge, or the system itself.
type EventSource string

const (
	SourceForm   EventSource = "form"
	SourceChat   EventSource = "chat"
	SourceSystem EventSource = "system"
)

// Valid reports whether es is a known event source.
func (es EventSource) Valid() bool {
	switch es {
	case SourceForm, SourceChat, SourceSystem:
		return true
	default:
		return false
	}
}
