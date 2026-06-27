package web

import (
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
	"github.com/paulkinyatti/local-scava/internal/service"
)

// sparklineSVG renders a monochrome SVG sparkline of daily published-tier counts
// from a cadence heatmap window. Server-generated; no JS charting dependency.
func sparklineSVG(cells []service.CadenceCell) template.HTML {
	const w, h, pad = 600, 60, 4
	if len(cells) == 0 {
		return template.HTML(`<svg viewBox="0 0 600 60" class="chart"></svg>`)
	}
	maxV := 1
	for _, c := range cells {
		if c.Published > maxV {
			maxV = c.Published
		}
	}
	n := len(cells)
	step := float64(w-2*pad) / float64(max(n-1, 1))
	var pts strings.Builder
	for i, c := range cells {
		x := float64(pad) + step*float64(i)
		y := float64(h-pad) - (float64(c.Published)/float64(maxV))*float64(h-2*pad)
		if i > 0 {
			pts.WriteByte(' ')
		}
		fmt.Fprintf(&pts, "%.1f,%.1f", x, y)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" class="chart" preserveAspectRatio="none">`, w, h)
	fmt.Fprintf(&b, `<polyline fill="none" stroke="var(--cyan)" stroke-width="1.5" points="%s"/>`, pts.String())
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// tierBarsSVG renders a monochrome horizontal bar chart of published tier counts.
func tierBarsSVG(mix map[domain.Tier]int) template.HTML {
	maxV := 1
	for _, v := range mix {
		if v > maxV {
			maxV = v
		}
	}
	const w, rowH, pad, labelW = 360, 26, 6, 80
	tiers := domain.AllTiers()
	h := rowH*len(tiers) + pad*2
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" class="chart">`, w, h)
	for i, t := range tiers {
		y := pad + i*rowH
		v := mix[t]
		barMax := w - labelW - pad
		bw := int(float64(barMax) * float64(v) / float64(maxV))
		fmt.Fprintf(&b, `<text x="%d" y="%d" class="chart-label">%s</text>`, pad, y+rowH/2+4, t.Label())
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" fill="currentColor" opacity="0.85"/>`,
			labelW, y+4, bw, rowH-10)
		fmt.Fprintf(&b, `<text x="%d" y="%d" class="chart-label">%d</text>`, labelW+bw+6, y+rowH/2+4, v)
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// tierMixDonutSVG renders a monochrome SVG donut of published-tier distribution
// (spec frontend-templates: "monochrome donut … flags neglect of the deep blog
// tier"). Each tier is a grayscale arc segment; the centre shows the total.
func tierMixDonutSVG(mix map[domain.Tier]int) template.HTML {
	tiers := domain.AllTiers()
	total := 0
	for _, t := range tiers {
		total += mix[t]
	}
	const size, stroke = 180, 26
	cx, cy := float64(size)/2, float64(size)/2
	r := cx - stroke
	circ := 2 * math.Pi * r

	shades := map[domain.Tier]string{
		domain.TierBlog:     "var(--green)",
		domain.TierLinkedIn: "var(--blue)",
		domain.TierX:        "var(--yellow)",
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" class="chart donut" role="img" aria-label="tier mix donut">`, size, size+44)
	// Track ring.
	fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="var(--line)" stroke-width="%d"/>`, cx, cy, r, stroke)

	if total > 0 {
		offset := 0.0
		for _, t := range tiers {
			v := mix[t]
			if v == 0 {
				continue
			}
			seg := float64(v) / float64(total) * circ
			fmt.Fprintf(&b,
				`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="%s" stroke-width="%d" `+
					`stroke-dasharray="%.2f %.2f" stroke-dashoffset="%.2f" transform="rotate(-90 %.1f %.1f)"/>`,
				cx, cy, r, shades[t], stroke, seg, circ-seg, -offset, cx, cy)
			offset += seg
		}
	}
	// Centre total.
	fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" class="donut-total">%d</text>`, cx, cy+2, total)
	fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" class="chart-label">published</text>`, cx, cy+18)

	// Legend row below the ring.
	lx, ly := 8, size+24
	for _, t := range tiers {
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="10" height="10" fill="%s"/>`, lx, ly-9, shades[t])
		fmt.Fprintf(&b, `<text x="%d" y="%d" class="chart-label">%s %d</text>`, lx+14, ly, t.Label(), mix[t])
		lx += 58
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// sprintTraceSVG renders a sprint as a left-to-right phase waterfall (spec 09
// "traces"): four phase spans, width proportional to time spent, fill keyed to
// checklist completion in that phase. The open (current) span is hatched.
func sprintTraceSVG(spans []service.PhaseSpan) template.HTML {
	const w, rowH, gap, labelW, pad = 720, 30, 6, 150, 6
	if len(spans) == 0 {
		return template.HTML(`<svg viewBox="0 0 720 40" class="chart"></svg>`)
	}
	h := pad*2 + len(spans)*(rowH+gap)
	track := float64(w - labelW - pad*2)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" class="chart trace" role="img" aria-label="sprint phase trace">`, w, h)
	b.WriteString(`<defs><pattern id="hatch" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">` +
		`<rect width="6" height="6" fill="var(--bg-2)"/><line x1="0" y1="0" x2="0" y2="6" stroke="var(--fg-1)" stroke-width="1.5"/></pattern></defs>`)
	for i, sp := range spans {
		y := pad + i*(rowH+gap)
		x := float64(labelW+pad) + track*sp.OffsetPct/100
		bw := track * sp.WidthPct / 100
		if bw < 2 {
			bw = 2
		}
		fillOpacity := 0.35
		if sp.TotalGates > 0 {
			fillOpacity = 0.35 + 0.6*float64(sp.DoneGates)/float64(sp.TotalGates)
		}
		fmt.Fprintf(&b, `<text x="%d" y="%d" class="chart-label">%d · %s</text>`, pad, y+rowH/2+4, int(sp.Phase), sp.Label)
		if sp.IsCurrent {
			fmt.Fprintf(&b, `<rect x="%.1f" y="%d" width="%.1f" height="%d" fill="url(#hatch)" stroke="var(--fg-0)" stroke-width="1"/>`,
				x, y, bw, rowH)
		} else {
			fmt.Fprintf(&b, `<rect x="%.1f" y="%d" width="%.1f" height="%d" fill="currentColor" fill-opacity="%.2f" stroke="var(--line)"/>`,
				x, y, bw, rowH, fillOpacity)
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="%d" class="chart-label trace-ann">%s · %d/%d</text>`,
			x+4, y+rowH/2+4, sp.DurationText, sp.DoneGates, sp.TotalGates)
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// --- Grafana-style chart additions ----------------------------------------

// barGaugeSVG renders a horizontal bar gauge with a value/max indicator — the
// Grafana "bar gauge" panel style. Color shifts from dim to bright as the fill
// increases.
func barGaugeSVG(label string, value, max int, color string) template.HTML {
	if max <= 0 {
		max = 1
	}
	pct := float64(value) / float64(max)
	if pct > 1 {
		pct = 1
	}
	const w, h = 280, 28
	barW := int(pct * float64(w-80))
	if barW < 0 {
		barW = 0
	}
	// Grafana-style: gradient from dark to vivid
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" class="chart bar-gauge">`, w, h)
	// Track background.
	fmt.Fprintf(&b, `<rect x="0" y="4" width="%d" height="20" fill="var(--bg-2)" rx="2"/>`, w-80)
	// Filled bar with slight gradient effect.
	if barW > 0 {
		fmt.Fprintf(&b, `<rect x="0" y="4" width="%d" height="20" fill="%s" opacity="0.9" rx="2"/>`, barW, color)
		// Glow highlight at the top of the bar.
		fmt.Fprintf(&b, `<rect x="0" y="4" width="%d" height="4" fill="%s" opacity="0.3" rx="2"/>`, barW, color)
	}
	// Value text.
	fmt.Fprintf(&b, `<text x="%d" y="18" class="chart-label" fill="var(--fg-0)" style="font-size:12px">%d/%d</text>`, w-74, value, max)
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// areaChartSVG renders a filled area chart for a time series of day counts —
// like Grafana's time series panels with gradient fill.
func areaChartSVG(data []service.DayCount, color string) template.HTML {
	const w, h, pad = 600, 80, 4
	if len(data) == 0 {
		return template.HTML(fmt.Sprintf(`<svg viewBox="0 0 %d %d" class="chart"></svg>`, w, h))
	}
	maxV := 1
	for _, d := range data {
		if d.Count > maxV {
			maxV = d.Count
		}
	}
	n := len(data)
	step := float64(w-2*pad) / float64(max(n-1, 1))

	var pts strings.Builder
	// Start at bottom-left for area fill.
	fmt.Fprintf(&pts, "%.1f,%.1f ", float64(pad), float64(h-pad))
	for i, d := range data {
		x := float64(pad) + step*float64(i)
		y := float64(h-pad) - (float64(d.Count)/float64(maxV))*float64(h-2*pad)
		fmt.Fprintf(&pts, "%.1f,%.1f ", x, y)
	}
	// Close at bottom-right.
	fmt.Fprintf(&pts, "%.1f,%.1f", float64(w-pad), float64(h-pad))

	// Line points (no bottom corners).
	var linePts strings.Builder
	for i, d := range data {
		x := float64(pad) + step*float64(i)
		y := float64(h-pad) - (float64(d.Count)/float64(maxV))*float64(h-2*pad)
		if i > 0 {
			linePts.WriteByte(' ')
		}
		fmt.Fprintf(&linePts, "%.1f,%.1f", x, y)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" class="chart area-chart" preserveAspectRatio="none">`, w, h)
	// Gradient fill area.
	fmt.Fprintf(&b, `<polygon fill="%s" fill-opacity="0.15" points="%s"/>`, color, pts.String())
	// Line on top.
	fmt.Fprintf(&b, `<polyline fill="none" stroke="%s" stroke-width="1.5" points="%s"/>`, color, linePts.String())
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// uptimeSVG renders a large uptime-style counter like Grafana's "6d 13:12:27"
// panels.
func uptimeSVG(days int, label string) template.HTML {
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="uptime-panel">`)
	fmt.Fprintf(&b, `<div class="uptime-val mono">%dd</div>`, days)
	fmt.Fprintf(&b, `<div class="uptime-label muted">%s</div>`, label)
	fmt.Fprintf(&b, `</div>`)
	return template.HTML(b.String())
}

// progressRingSVG renders a circular progress ring like Grafana's gauge panels.
func progressRingSVG(pct float64, label string, color string) template.HTML {
	const size, stroke = 100, 10
	cx, cy := float64(size)/2, float64(size)/2
	r := cx - float64(stroke)
	circ := 2 * math.Pi * r
	filled := circ * pct
	if filled < 0 {
		filled = 0
	}
	if filled > circ {
		filled = circ
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" class="chart ring-gauge">`, size, size+20)
	// Track.
	fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="var(--line)" stroke-width="%d"/>`, cx, cy, r, stroke)
	// Filled arc.
	if filled > 0 {
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="%s" stroke-width="%d" `+
			`stroke-dasharray="%.2f %.2f" stroke-dashoffset="%.2f" transform="rotate(-90 %.1f %.1f)"/>`,
			cx, cy, r, color, stroke, filled, circ-filled, 0.0, cx, cy)
	}
	// Center percentage.
	fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" fill="var(--fg-0)" class="ring-pct mono">%.0f%%</text>`, cx, cy+5, pct*100)
	// Label below.
	fmt.Fprintf(&b, `<text x="%.1f" y="%d" text-anchor="middle" class="chart-label">%s</text>`, cx, size+14, label)
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// activityFeedHTML renders the last N career events as a compact log stream
// similar to Grafana's log panel.
func activityFeedHTML(events []domain.CareerEvent) template.HTML {
	if len(events) == 0 {
		return template.HTML(`<div class="muted">No recent activity</div>`)
	}
	var b strings.Builder
	b.WriteString(`<div class="activity-feed">`)
	for _, e := range events {
		ts := e.OccurredAt.Local().Format("Jan 02 15:04")
		kindColor := "var(--fg-2)"
		switch {
		case strings.Contains(e.Kind, "shipped"):
			kindColor = "var(--green)"
		case strings.Contains(e.Kind, "published"):
			kindColor = "var(--cyan)"
		case strings.Contains(e.Kind, "created"):
			kindColor = "var(--blue)"
		case strings.Contains(e.Kind, "phase"):
			kindColor = "var(--orange)"
		case strings.Contains(e.Kind, "log"):
			kindColor = "var(--purple)"
		case strings.Contains(e.Kind, "retro"):
			kindColor = "var(--yellow)"
		}
		fmt.Fprintf(&b, `<div class="feed-line"><span class="feed-time mono">%s</span>`+
			`<span class="feed-dot" style="background:%s"></span>`+
			`<span class="feed-msg">%s</span></div>`, ts, kindColor, template.HTMLEscapeString(e.Summary))
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

// weekdayLabel returns a short 3-letter label.
func weekdayLabel(t time.Time) string {
	return t.Format("Mon")
}
