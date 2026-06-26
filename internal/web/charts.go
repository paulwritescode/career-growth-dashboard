package web

import (
	"fmt"
	"html/template"
	"strings"

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
	fmt.Fprintf(&b, `<polyline fill="none" stroke="currentColor" stroke-width="1.5" points="%s"/>`, pts.String())
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
