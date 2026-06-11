package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"
	"gorm.io/gorm"
)

type ExportHandler struct{ db *gorm.DB }

func NewExportHandler(db *gorm.DB) *ExportHandler { return &ExportHandler{db: db} }

// exportRow is one review row joined with its PR and repository.
type exportRow struct {
	ID           uint
	Status       string
	Score        int
	Summary      string
	InputTokens  int
	OutputTokens int
	LatencyMS    int64
	CreatedAt    time.Time
	Owner        string
	RepoName     string
	PRNumber     int
}

// parseDateRange reads start/end query params (YYYY-MM-DD); end is exclusive (+1 day).
func parseDateRange(r *http.Request) (start, end time.Time, repoFilter string) {
	q := r.URL.Query()
	end = time.Now()
	if v := q.Get("start"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			start = t
		}
	}
	if v := q.Get("end"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			end = t.Add(24 * time.Hour)
		}
	}
	return start, end, q.Get("repo")
}

func (h *ExportHandler) queryRows(r *http.Request, start, end time.Time, repoFilter string) ([]exportRow, error) {
	tx := h.db.WithContext(r.Context()).
		Table("reviews").
		Select(`reviews.id, reviews.status, reviews.score, reviews.summary,
			reviews.input_tokens, reviews.output_tokens, reviews.latency_ms, reviews.created_at,
			repositories.owner, repositories.name as repo_name, pull_requests.number as pr_number`).
		Joins("JOIN pull_requests ON pull_requests.id = reviews.pr_id").
		Joins("JOIN repositories ON repositories.id = pull_requests.repo_id").
		Order("reviews.created_at desc")

	if !start.IsZero() {
		tx = tx.Where("reviews.created_at >= ?", start)
	}
	tx = tx.Where("reviews.created_at < ?", end)
	if repoFilter != "" {
		tx = tx.Where("repositories.owner || '/' || repositories.name = ?", repoFilter)
	}

	var rows []exportRow
	err := tx.Scan(&rows).Error
	return rows, err
}

func (h *ExportHandler) ReviewsCSV(w http.ResponseWriter, r *http.Request) {
	start, end, repoFilter := parseDateRange(r)
	rows, err := h.queryRows(r, start, end, repoFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="reviews_%s.csv"`, time.Now().Format("20060102")))

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "status", "score", "summary", "repo", "pr_number", "input_tokens", "output_tokens", "latency_ms", "created_at"})
	for _, r := range rows {
		_ = cw.Write([]string{
			fmt.Sprint(r.ID),
			r.Status,
			fmt.Sprint(r.Score),
			r.Summary,
			r.Owner + "/" + r.RepoName,
			fmt.Sprint(r.PRNumber),
			fmt.Sprint(r.InputTokens),
			fmt.Sprint(r.OutputTokens),
			fmt.Sprint(r.LatencyMS),
			r.CreatedAt.Format(time.RFC3339),
		})
	}
	cw.Flush()
}

// reviewSummary holds the aggregate statistics used in the PDF report.
type reviewSummary struct {
	Count        int
	TotalScore   int
	Approvals    int
	Changes      int
	Comments     int
	PerRepoCount map[string]int
	PerRepoScore map[string]int
}

// AvgScore returns the mean score across all reviews (0 when there are none).
func (s reviewSummary) AvgScore() float64 {
	if s.Count == 0 {
		return 0
	}
	return float64(s.TotalScore) / float64(s.Count)
}

// summarizeReviews aggregates rows into totals and per-repo breakdowns. Pure (no DB),
// so it is unit-testable.
func summarizeReviews(rows []exportRow) reviewSummary {
	s := reviewSummary{
		Count:        len(rows),
		PerRepoCount: map[string]int{},
		PerRepoScore: map[string]int{},
	}
	for _, row := range rows {
		s.TotalScore += row.Score
		switch row.Status {
		case "APPROVE":
			s.Approvals++
		case "REQUEST_CHANGES":
			s.Changes++
		case "COMMENT":
			s.Comments++
		}
		key := row.Owner + "/" + row.RepoName
		s.PerRepoCount[key]++
		s.PerRepoScore[key] += row.Score
	}
	return s
}

// renderReportPDF builds the report PDF bytes. Pure (no DB / no http), so it is
// unit-testable independently of the handler.
func renderReportPDF(rows []exportRow, start, end time.Time, repoFilter string, generatedAt time.Time) []byte {
	s := summarizeReviews(rows)

	rangeLabel := "all time"
	if !start.IsZero() {
		rangeLabel = start.Format("2006-01-02") + " to " + end.Add(-24*time.Hour).Format("2006-01-02")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("PR Reviewer Report", false)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 18)
	pdf.Cell(0, 12, "PR Reviewer — Review Report")
	pdf.Ln(14)

	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(90, 90, 90)
	meta := "Range: " + rangeLabel
	if repoFilter != "" {
		meta += "   |   Repo: " + repoFilter
	}
	meta += "   |   Generated: " + generatedAt.Format("2006-01-02 15:04")
	pdf.Cell(0, 6, meta)
	pdf.Ln(12)
	pdf.SetTextColor(0, 0, 0)

	// Summary metrics.
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(0, 8, "Summary")
	pdf.Ln(9)
	pdf.SetFont("Arial", "", 11)
	summaryLines := [][2]string{
		{"Total reviews", fmt.Sprint(s.Count)},
		{"Average score", fmt.Sprintf("%.1f / 100", s.AvgScore())},
		{"Approved", fmt.Sprint(s.Approvals)},
		{"Changes requested", fmt.Sprint(s.Changes)},
		{"Commented", fmt.Sprint(s.Comments)},
	}
	for _, l := range summaryLines {
		pdf.Cell(60, 7, l[0])
		pdf.SetFont("Arial", "B", 11)
		pdf.Cell(0, 7, l[1])
		pdf.SetFont("Arial", "", 11)
		pdf.Ln(7)
	}
	pdf.Ln(4)

	// Per-repo breakdown.
	if len(s.PerRepoCount) > 0 {
		pdf.SetFont("Arial", "B", 13)
		pdf.Cell(0, 8, "By repository")
		pdf.Ln(9)
		pdf.SetFont("Arial", "B", 10)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(100, 7, "Repository", "1", 0, "L", true, 0, "")
		pdf.CellFormat(35, 7, "Reviews", "1", 0, "R", true, 0, "")
		pdf.CellFormat(35, 7, "Avg score", "1", 0, "R", true, 0, "")
		pdf.Ln(7)
		pdf.SetFont("Arial", "", 10)
		repos := make([]string, 0, len(s.PerRepoCount))
		for k := range s.PerRepoCount {
			repos = append(repos, k)
		}
		sort.Slice(repos, func(i, j int) bool { return s.PerRepoCount[repos[i]] > s.PerRepoCount[repos[j]] })
		for _, k := range repos {
			avg := float64(s.PerRepoScore[k]) / float64(s.PerRepoCount[k])
			pdf.CellFormat(100, 7, truncatePDF(k, 55), "1", 0, "L", false, 0, "")
			pdf.CellFormat(35, 7, fmt.Sprint(s.PerRepoCount[k]), "1", 0, "R", false, 0, "")
			pdf.CellFormat(35, 7, fmt.Sprintf("%.1f", avg), "1", 0, "R", false, 0, "")
			pdf.Ln(7)
		}
		pdf.Ln(4)
	}

	// Recent reviews (cap at 25 for readability).
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(0, 8, "Recent reviews")
	pdf.Ln(9)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(70, 6, "Repository / PR", "1", 0, "L", true, 0, "")
	pdf.CellFormat(40, 6, "Status", "1", 0, "L", true, 0, "")
	pdf.CellFormat(20, 6, "Score", "1", 0, "R", true, 0, "")
	pdf.CellFormat(40, 6, "Date", "1", 0, "L", true, 0, "")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 9)
	limit := len(rows)
	if limit > 25 {
		limit = 25
	}
	for _, row := range rows[:limit] {
		label := fmt.Sprintf("%s/%s#%d", row.Owner, row.RepoName, row.PRNumber)
		pdf.CellFormat(70, 6, truncatePDF(label, 40), "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, row.Status, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 6, fmt.Sprint(row.Score), "1", 0, "R", false, 0, "")
		pdf.CellFormat(40, 6, row.CreatedAt.Format("2006-01-02"), "1", 0, "L", false, 0, "")
		pdf.Ln(6)
	}
	if len(rows) > limit {
		pdf.Ln(2)
		pdf.SetFont("Arial", "I", 9)
		pdf.SetTextColor(120, 120, 120)
		pdf.Cell(0, 6, fmt.Sprintf("… and %d more (see CSV export for the full list).", len(rows)-limit))
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil
	}
	return buf.Bytes()
}

// ReviewsPDF renders a summary report PDF for the selected date range / repo filter.
func (h *ExportHandler) ReviewsPDF(w http.ResponseWriter, r *http.Request) {
	start, end, repoFilter := parseDateRange(r)
	rows, err := h.queryRows(r, start, end, repoFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}

	out := renderReportPDF(rows, start, end, repoFilter, time.Now())
	if out == nil {
		writeError(w, http.StatusInternalServerError, "pdf generation failed")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pr-reviewer-report_%s.pdf"`, time.Now().Format("20060102")))
	_, _ = w.Write(out)
}

func truncatePDF(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
