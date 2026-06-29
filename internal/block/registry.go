// Package block defines the block registry — the static list of toggleable
// features — and the service that manages per-user enable/disable state.
package block

// SidebarGroup is the section a block appears in within the sidebar.
type SidebarGroup string

const (
	GroupWatch   SidebarGroup = "Watch"
	GroupSprints SidebarGroup = "Sprints"
	GroupCadence SidebarGroup = "Cadence"
	GroupAct     SidebarGroup = "Act"
)

// Def describes a single block in the registry.
type Def struct {
	Key         string       // stable identifier stored in user_blocks
	Name        string       // display name
	Description string       // one-line description for onboarding/settings
	Icon        string       // Lucide icon name
	Group       SidebarGroup // sidebar section
	Routes      []string     // owned route patterns
	APIPaths    []string     // /api/v1 paths this block enables
	DefaultOn   bool         // whether it's recommended-on by default
}

// registry is the canonical ordered list of all blocks.
var registry = []Def{
	{
		Key:         "sprint",
		Name:        "Sprint",
		Description: "Sprint planner — phases, deliverables, checklists, health tracking",
		Icon:        "rocket",
		Group:       GroupSprints,
		Routes:      []string{"/sprints", "/sprints/"},
		APIPaths:    []string{"/api/v1/sprints"},
		DefaultOn:   true,
	},
	{
		Key:         "adr",
		Name:        "ADR",
		Description: "Architecture Decision Records — capture significant design choices, context, alternatives, and consequences",
		Icon:        "file-text",
		Group:       GroupSprints,
		Routes:      []string{"/adrs", "/adrs/"},
		APIPaths:    nil,
		DefaultOn:   false,
	},
	{
		Key:         "logs",
		Name:        "Logs",
		Description: "Daily build log + career-event audit trail",
		Icon:        "terminal",
		Group:       GroupCadence,
		Routes:      []string{"/logs"},
		APIPaths:    []string{"/api/v1/logs", "/api/v1/events"},
		DefaultOn:   true,
	},
	{
		Key:         "posts",
		Name:        "Post management",
		Description: "Content cadence across platforms",
		Icon:        "pen-tool",
		Group:       GroupCadence,
		Routes:      []string{"/cadence", "/posts/"},
		APIPaths:    nil,
		DefaultOn:   false,
	},
	{
		Key:         "todo",
		Name:        "Todo list",
		Description: "Task management with priority, due dates, and sprint linkage",
		Icon:        "check-square",
		Group:       GroupAct,
		Routes:      []string{"/todos"},
		APIPaths:    nil,
		DefaultOn:   false,
	},
	{
		Key:         "traces",
		Name:        "Traces",
		Description: "Sprint rendered as a phase waterfall (width ∝ time per phase)",
		Icon:        "activity",
		Group:       GroupWatch,
		Routes:      []string{"/traces"},
		APIPaths:    []string{"/api/v1/traces"},
		DefaultOn:   false,
	},
	{
		Key:         "metrics",
		Name:        "Metrics",
		Description: "Posting cadence, streaks, ship rate, tier-mix — the Grafana panel for your career",
		Icon:        "bar-chart-2",
		Group:       GroupWatch,
		Routes:      []string{"/metrics"},
		APIPaths:    []string{"/api/v1/metrics"},
		DefaultOn:   true,
	},
	{
		Key:         "habits",
		Name:        "Habit tracker",
		Description: "Daily binary habits linked to sprint health",
		Icon:        "flame",
		Group:       GroupWatch,
		Routes:      []string{"/habits"},
		APIPaths:    nil,
		DefaultOn:   false,
	},
	{
		Key:         "review",
		Name:        "Weekly review",
		Description: "Prompted end-of-week reflection, auto-populated from logs and sprint data",
		Icon:        "clipboard",
		Group:       GroupAct,
		Routes:      []string{"/review"},
		APIPaths:    nil,
		DefaultOn:   false,
	},
}

// Registry returns the full ordered list of block definitions.
func Registry() []Def {
	return registry
}

// ByKey returns the block definition for a given key, or false if unknown.
func ByKey(key string) (Def, bool) {
	for _, d := range registry {
		if d.Key == key {
			return d, true
		}
	}
	return Def{}, false
}

// AllKeys returns just the block keys in registry order.
func AllKeys() []string {
	keys := make([]string, len(registry))
	for i, d := range registry {
		keys[i] = d.Key
	}
	return keys
}

// DefaultBlocksForRole returns the recommended block keys for a given role.
func DefaultBlocksForRole(role string) []string {
	switch role {
	case "backend", "fullstack":
		return []string{"sprint", "adr", "logs", "todo", "metrics"}
	case "frontend":
		return []string{"sprint", "logs", "posts", "todo", "metrics"}
	case "devops":
		return []string{"sprint", "adr", "logs", "traces", "metrics"}
	case "data":
		return []string{"sprint", "adr", "logs", "metrics"}
	case "indie":
		return []string{"sprint", "posts", "logs", "todo", "metrics"}
	case "manager":
		return []string{"sprint", "adr", "logs", "metrics"}
	default: // "other"
		return []string{"sprint", "logs", "metrics"}
	}
}
