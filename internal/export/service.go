// Package export provides server-side PDF generation for sprint reports,
// ADRs, log ranges, and metrics snapshots. Pure-Go, no headless browser.
package export

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/paulkinyatti/local-scava/internal/domain"
	"github.com/paulkinyatti/local-scava/internal/service"
	"github.com/paulkinyatti/local-scava/internal/store"
)

// Service generates PDF exports.
type Service struct {
	svc   *service.Service
	store *store.Store
}

// New creates an export service.
func New(svc *service.Service, st *store.Store) *Service {
	return &Service{svc: svc, store: st}
}

// SprintReport generates a full sprint report PDF.
func (s *Service) SprintReport(ctx context.Context, sprintID int64) ([]byte, string, error) {
	sp, err := s.svc.GetSprint(ctx, sprintID)
	if err != nil {
		return nil, "", fmt.Errorf("export: get sprint: %w", err)
	}

	pdf := newPDF()
	addTitle(pdf, "Sprint Report")

	// Header section.
	addSection(pdf, "Overview")
	title := sp.SkillName
	if sp.Title != nil && *sp.Title != "" {
		title = *sp.Title
	}
	addKV(pdf, "Title", title)
	addKV(pdf, "Skill", sp.SkillName)
	addKV(pdf, "Microapp", sp.MicroappOneLiner)
	addKV(pdf, "Status", string(sp.Status))
	if sp.LiveURL != nil && *sp.LiveURL != "" {
		addKV(pdf, "Live URL", *sp.LiveURL)
	}
	if sp.StartedOn != nil {
		addKV(pdf, "Started", *sp.StartedOn)
	}
	if sp.EndedOn != nil {
		addKV(pdf, "Ended", *sp.EndedOn)
	}

	// Deliverables.
	deliverables, _ := s.store.ListDeliverables(ctx, sprintID)
	if len(deliverables) > 0 {
		addSection(pdf, "Deliverables")
		for _, d := range deliverables {
			marker := "☐"
			if d.IsDone {
				marker = "✓"
			}
			addBody(pdf, fmt.Sprintf("  %s %s", marker, d.Text))
		}
		done := 0
		for _, d := range deliverables {
			if d.IsDone {
				done++
			}
		}
		addBody(pdf, fmt.Sprintf("\n  Completion: %d/%d (%.0f%%)", done, len(deliverables), float64(done)/float64(len(deliverables))*100))
	}

	// Build logs.
	logs, _ := s.svc.ListLogsBySprint(ctx, sprintID)
	if len(logs) > 0 {
		addSection(pdf, "Build Log")
		for _, l := range logs {
			addBody(pdf, fmt.Sprintf("%s — %s", l.LogDate, l.WorkedOn))
			if l.Insight != "" {
				addBody(pdf, fmt.Sprintf("  Insight: %s", l.Insight))
			}
		}
	}

	// Retro.
	if sp.RetroWorked != "" || sp.RetroLearned != "" {
		addSection(pdf, "Retrospective")
		if sp.RetroWorked != "" {
			addKV(pdf, "What worked", sp.RetroWorked)
		}
		if sp.RetroDifferently != "" {
			addKV(pdf, "Do differently", sp.RetroDifferently)
		}
		if sp.RetroLearned != "" {
			addKV(pdf, "Key learning", sp.RetroLearned)
		}
	}

	slug := slugify(title)
	filename := fmt.Sprintf("sprint-%s-%s.pdf", slug, time.Now().Format("2006-01-02"))
	return pdfBytes(pdf), filename, nil
}

// ADR generates an ADR decision record PDF.
func (s *Service) ADR(ctx context.Context, adrID int64) ([]byte, string, error) {
	adr, err := s.store.GetADR(ctx, adrID)
	if err != nil {
		return nil, "", fmt.Errorf("export: get adr: %w", err)
	}

	pdf := newPDF()
	addTitle(pdf, fmt.Sprintf("ADR-%03d: %s", adr.Number, adr.Title))

	addKV(pdf, "Status", string(adr.Status))
	if adr.DecidedOn != nil {
		addKV(pdf, "Decided", *adr.DecidedOn)
	}

	if adr.Problem != "" {
		addSection(pdf, "Problem")
		addBody(pdf, adr.Problem)
	}
	if adr.Options != "" {
		addSection(pdf, "Options Considered")
		addBody(pdf, adr.Options)
	}
	if adr.Decision != "" {
		addSection(pdf, "Decision")
		addBody(pdf, adr.Decision)
	}
	if adr.Why != "" {
		addSection(pdf, "Why")
		addBody(pdf, adr.Why)
	}
	if adr.Consequences != "" {
		addSection(pdf, "Consequences")
		addBody(pdf, adr.Consequences)
	}

	slug := slugify(adr.Title)
	filename := fmt.Sprintf("adr-%03d-%s.pdf", adr.Number, slug)
	return pdfBytes(pdf), filename, nil
}

// LogRange generates a PDF of logs within a date range.
func (s *Service) LogRange(ctx context.Context, from, to string) ([]byte, string, error) {
	if from == "" {
		from = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}

	logs, err := s.store.ListLogsByDateRange(ctx, nil, from, to)
	if err != nil {
		return nil, "", fmt.Errorf("export: list logs: %w", err)
	}

	pdf := newPDF()
	addTitle(pdf, fmt.Sprintf("Build Logs: %s to %s", from, to))

	if len(logs) == 0 {
		addBody(pdf, "No log entries in this range.")
	} else {
		for _, l := range logs {
			addSection(pdf, l.LogDate)
			addBody(pdf, l.WorkedOn)
			if l.WhatHappened != "" {
				addBody(pdf, "  What happened: "+l.WhatHappened)
			}
			if l.Insight != "" {
				addBody(pdf, "  Insight: "+l.Insight)
			}
			if l.NextUp != "" {
				addBody(pdf, "  Next up: "+l.NextUp)
			}
		}
	}

	filename := fmt.Sprintf("logs-%s_%s.pdf", from, to)
	return pdfBytes(pdf), filename, nil
}

// MetricsSnapshot generates a metrics PDF for a given window.
func (s *Service) MetricsSnapshot(ctx context.Context, window string) ([]byte, string, error) {
	pdf := newPDF()
	addTitle(pdf, "Metrics Snapshot — "+window)

	cadence30, _ := s.svc.CadenceRate(ctx, 30)
	cadence90, _ := s.svc.CadenceRate(ctx, 90)
	streaks, _ := s.svc.Streaks(ctx, 365)
	shipRate, _ := s.svc.ShipRate(ctx)

	addSection(pdf, "Cadence")
	addKV(pdf, "30-day rate", fmt.Sprintf("%.0f%%", cadence30*100))
	addKV(pdf, "90-day rate", fmt.Sprintf("%.0f%%", cadence90*100))

	addSection(pdf, "Streaks")
	addKV(pdf, "Current", fmt.Sprintf("%d days", streaks.Current))
	addKV(pdf, "Longest", fmt.Sprintf("%d days", streaks.Longest))

	addSection(pdf, "Ship Rate")
	addKV(pdf, "Shipped", fmt.Sprintf("%d of %d sprints", shipRate.Shipped, shipRate.Ended))

	filename := fmt.Sprintf("metrics-%s-%s.pdf", window, time.Now().Format("2006-01-02"))
	return pdfBytes(pdf), filename, nil
}

// --- Internal PDF helpers ---

func newPDF() *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 10)
	return pdf
}

func addTitle(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, title, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	pdf.CellFormat(0, 5, "Generated: "+time.Now().Format("2006-01-02 15:04 MST"), "", 1, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "", 10)
}

func addSection(pdf *fpdf.Fpdf, title string) {
	pdf.Ln(3)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 7, title, "B", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.Ln(2)
}

func addKV(pdf *fpdf.Fpdf, key, value string) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(35, 5, key+":", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.MultiCell(0, 5, value, "", "L", false)
}

func addBody(pdf *fpdf.Fpdf, text string) {
	pdf.SetFont("Helvetica", "", 9)
	pdf.MultiCell(0, 5, text, "", "L", false)
	pdf.Ln(1)
}

func pdfBytes(pdf *fpdf.Fpdf) []byte {
	var buf strings.Builder
	// fpdf doesn't have a direct bytes output, use the writer approach.
	// Actually it does have Output which writes to a writer.
	// We'll use a buffer.
	_ = buf // unused, use the proper approach below
	return pdfToBytes(pdf)
}

func pdfToBytes(pdf *fpdf.Fpdf) []byte {
	// fpdf.Output returns (io.ReadCloser, error) in v0.9
	// Let's use the buffer approach
	var b strings.Builder
	_ = b
	// Use OutputFileAndClose to a temp, or use the bytes buffer
	buf := &bytesBuffer{}
	if err := pdf.Output(buf); err != nil {
		return nil
	}
	return buf.Bytes()
}

// bytesBuffer implements io.Writer for fpdf.Output.
type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *bytesBuffer) Bytes() []byte {
	return b.data
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var out []byte
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			out = append(out, c)
		} else if c == ' ' || c == '_' || c == '/' {
			if len(out) > 0 && out[len(out)-1] != '-' {
				out = append(out, '-')
			}
		}
	}
	return strings.Trim(string(out), "-")
}

// Ensure domain import is used.
var _ domain.Sprint
