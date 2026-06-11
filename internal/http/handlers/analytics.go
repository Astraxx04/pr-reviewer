package handlers

import (
	"net/http"
	"time"

	"gorm.io/gorm"

	"pr-reviewer/internal/db/models"
)

// costPerInputToken and costPerOutputToken are rough average estimates in USD per token.
const costPerInputToken = 0.000003  // ~$3 / 1M tokens
const costPerOutputToken = 0.000015 // ~$15 / 1M tokens

type AnalyticsHandler struct{ db *gorm.DB }

func NewAnalyticsHandler(db *gorm.DB) *AnalyticsHandler { return &AnalyticsHandler{db: db} }

type dailyStat struct {
	Date  string  `json:"date"`
	Count int64   `json:"count"`
	Avg   float64 `json:"avg_score"`
}

func (h *AnalyticsHandler) Analytics(w http.ResponseWriter, r *http.Request) {
	days := min(queryInt(r, "days", 30), 90)
	since := time.Now().AddDate(0, 0, -days)

	var rows []struct {
		Day   time.Time
		Count int64
		Avg   float64
	}
	h.db.WithContext(r.Context()).
		Model(&models.Review{}).
		Select("date_trunc('day', created_at) as day, count(*) as count, coalesce(avg(score),0) as avg").
		Where("created_at >= ?", since).
		Group("day").
		Order("day").
		Scan(&rows)

	stats := make([]dailyStat, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, dailyStat{
			Date:  row.Day.Format("2006-01-02"),
			Count: row.Count,
			Avg:   row.Avg,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"series": stats, "days": days})
}

type repoCostStat struct {
	Repo         string  `json:"repo"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	EstCostUSD   float64 `json:"est_cost_usd"`
}

func (h *AnalyticsHandler) Cost(w http.ResponseWriter, r *http.Request) {
	days := min(queryInt(r, "days", 30), 90)
	since := time.Now().AddDate(0, 0, -days)

	var totals struct {
		InputTokens  int64
		OutputTokens int64
	}
	h.db.WithContext(r.Context()).
		Model(&models.Review{}).
		Select("coalesce(sum(input_tokens),0) as input_tokens, coalesce(sum(output_tokens),0) as output_tokens").
		Where("created_at >= ?", since).
		Scan(&totals)

	var byRepo []struct {
		Owner        string
		Name         string
		InputTokens  int64
		OutputTokens int64
	}
	h.db.WithContext(r.Context()).
		Table("reviews").
		Select("repositories.owner, repositories.name, coalesce(sum(reviews.input_tokens),0) as input_tokens, coalesce(sum(reviews.output_tokens),0) as output_tokens").
		Joins("JOIN pull_requests ON pull_requests.id = reviews.pr_id").
		Joins("JOIN repositories ON repositories.id = pull_requests.repo_id").
		Where("reviews.created_at >= ?", since).
		Group("repositories.id, repositories.owner, repositories.name").
		Order("input_tokens + output_tokens desc").
		Scan(&byRepo)

	repoStats := make([]repoCostStat, 0, len(byRepo))
	for _, r := range byRepo {
		est := float64(r.InputTokens)*costPerInputToken + float64(r.OutputTokens)*costPerOutputToken
		repoStats = append(repoStats, repoCostStat{
			Repo:         r.Owner + "/" + r.Name,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			EstCostUSD:   roundCents(est),
		})
	}

	totalEst := float64(totals.InputTokens)*costPerInputToken + float64(totals.OutputTokens)*costPerOutputToken
	writeJSON(w, http.StatusOK, map[string]any{
		"days":           days,
		"input_tokens":   totals.InputTokens,
		"output_tokens":  totals.OutputTokens,
		"est_cost_usd":   roundCents(totalEst),
		"by_repo":        repoStats,
	})
}

func roundCents(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
