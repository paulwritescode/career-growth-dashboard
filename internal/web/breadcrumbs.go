package web

// Crumb is a single breadcrumb entry.
type Crumb struct {
	Label string
	Href  string // empty for the last (current) crumb → rendered as text
}

// Breadcrumbs builds an ordered breadcrumb trail for a given page context.
// The first crumb is always "Dashboard" → "/". The last is the current page
// (no link). Entity-backed crumbs use the page's view-model title.
func Breadcrumbs(nav string, title string) []Crumb {
	crumbs := []Crumb{{Label: "Dashboard", Href: "/"}}

	switch nav {
	case "overview":
		// Dashboard is both first and current.
		crumbs[0].Href = ""
		return crumbs
	case "sprints":
		crumbs = append(crumbs, Crumb{Label: "Sprints", Href: ""})
	case "sprint_detail":
		crumbs = append(crumbs, Crumb{Label: "Sprints", Href: "/sprints"})
		crumbs = append(crumbs, Crumb{Label: title, Href: ""})
	case "adrs":
		crumbs = append(crumbs, Crumb{Label: "ADRs", Href: ""})
	case "adr_detail":
		crumbs = append(crumbs, Crumb{Label: "ADRs", Href: "/adrs"})
		crumbs = append(crumbs, Crumb{Label: title, Href: ""})
	case "cadence":
		crumbs = append(crumbs, Crumb{Label: "Cadence", Href: ""})
	case "post_detail":
		crumbs = append(crumbs, Crumb{Label: "Cadence", Href: "/cadence"})
		crumbs = append(crumbs, Crumb{Label: title, Href: ""})
	case "logbook":
		crumbs = append(crumbs, Crumb{Label: "Logbook", Href: ""})
	case "metrics":
		crumbs = append(crumbs, Crumb{Label: "Metrics", Href: ""})
	case "settings":
		crumbs = append(crumbs, Crumb{Label: "Settings", Href: ""})
	case "new":
		crumbs = append(crumbs, Crumb{Label: "New entry", Href: ""})
	case "habits":
		crumbs = append(crumbs, Crumb{Label: "Habits", Href: ""})
	case "review":
		crumbs = append(crumbs, Crumb{Label: "Weekly Review", Href: ""})
	case "todos":
		crumbs = append(crumbs, Crumb{Label: "Todos", Href: ""})
	default:
		if title != "" {
			crumbs = append(crumbs, Crumb{Label: title, Href: ""})
		}
	}
	return crumbs
}
