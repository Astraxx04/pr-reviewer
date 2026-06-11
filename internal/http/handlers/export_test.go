package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSummarizeReviews(t *testing.T) {
	rows := []exportRow{
		{Owner: "acme", RepoName: "web", Status: "APPROVE", Score: 90},
		{Owner: "acme", RepoName: "web", Status: "REQUEST_CHANGES", Score: 40},
		{Owner: "acme", RepoName: "api", Status: "COMMENT", Score: 70},
	}
	s := summarizeReviews(rows)

	if s.Count != 3 {
		t.Errorf("Count = %d, want 3", s.Count)
	}
	if s.Approvals != 1 || s.Changes != 1 || s.Comments != 1 {
		t.Errorf("verdict counts = %d/%d/%d, want 1/1/1", s.Approvals, s.Changes, s.Comments)
	}
	if got := s.AvgScore(); got != (90+40+70)/3.0 {
		t.Errorf("AvgScore = %.2f, want %.2f", got, (90+40+70)/3.0)
	}
	if s.PerRepoCount["acme/web"] != 2 || s.PerRepoCount["acme/api"] != 1 {
		t.Errorf("PerRepoCount = %v, want acme/web:2 acme/api:1", s.PerRepoCount)
	}
	if s.PerRepoScore["acme/web"] != 130 {
		t.Errorf("PerRepoScore[acme/web] = %d, want 130", s.PerRepoScore["acme/web"])
	}
}

func TestSummarizeReviewsEmpty(t *testing.T) {
	s := summarizeReviews(nil)
	if s.Count != 0 || s.AvgScore() != 0 {
		t.Errorf("empty summary: Count=%d Avg=%.2f, want 0/0", s.Count, s.AvgScore())
	}
}

func TestRenderReportPDF(t *testing.T) {
	rows := []exportRow{
		{Owner: "acme", RepoName: "web", PRNumber: 1, Status: "APPROVE", Score: 88, CreatedAt: time.Now()},
	}
	out := renderReportPDF(rows, time.Time{}, time.Now(), "", time.Unix(1_700_000_000, 0))
	if len(out) == 0 {
		t.Fatal("renderReportPDF returned empty output")
	}
	// Valid PDFs begin with the "%PDF" magic header.
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Errorf("output is not a PDF (prefix = %q)", string(out[:min(8, len(out))]))
	}
}

func TestParseDateRange(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/reviews/export.pdf?start=2026-01-01&end=2026-01-31&repo=acme/web", nil)
	start, end, repo := parseDateRange(req)
	if start.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("start = %v, want 2026-01-01", start)
	}
	// end is exclusive (+1 day).
	if end.Format("2006-01-02") != "2026-02-01" {
		t.Errorf("end = %v, want 2026-02-01 (exclusive)", end)
	}
	if repo != "acme/web" {
		t.Errorf("repo = %q, want acme/web", repo)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
