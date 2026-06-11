package handlers

import (
	"net/http"
	"time"

	"gorm.io/gorm"
)

type SystemMetricsHandler struct{ db *gorm.DB }

func NewSystemMetricsHandler(db *gorm.DB) *SystemMetricsHandler {
	return &SystemMetricsHandler{db: db}
}

func (h *SystemMetricsHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	var queueDepth int64
	h.db.WithContext(ctx).
		Raw("SELECT count(*) FROM river_job WHERE kind = 'review' AND state = 'available'").
		Scan(&queueDepth)

	var reviewsToday, reviewsWeek, reviewsMonth int64
	h.db.WithContext(ctx).Raw(
		"SELECT count(*) FROM reviews WHERE created_at >= date_trunc('day', now() AT TIME ZONE 'UTC')",
	).Scan(&reviewsToday)
	h.db.WithContext(ctx).Raw(
		"SELECT count(*) FROM reviews WHERE created_at >= ?", now.AddDate(0, 0, -7),
	).Scan(&reviewsWeek)
	h.db.WithContext(ctx).Raw(
		"SELECT count(*) FROM reviews WHERE created_at >= ?", now.AddDate(0, -1, 0),
	).Scan(&reviewsMonth)

	var latencies struct {
		P50 float64
		P95 float64
		P99 float64
	}
	h.db.WithContext(ctx).Raw(`
		SELECT
			coalesce(percentile_cont(0.50) WITHIN GROUP (ORDER BY latency_ms), 0) AS p50,
			coalesce(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms), 0) AS p95,
			coalesce(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms), 0) AS p99
		FROM reviews WHERE latency_ms > 0
	`).Scan(&latencies)

	var webhookErrors, webhookTotal int64
	cutoff24h := now.Add(-24 * time.Hour)
	h.db.WithContext(ctx).Raw(
		"SELECT count(*) FROM webhook_deliveries WHERE processed_at >= ? AND status = 'failed'", cutoff24h,
	).Scan(&webhookErrors)
	h.db.WithContext(ctx).Raw(
		"SELECT count(*) FROM webhook_deliveries WHERE processed_at >= ?", cutoff24h,
	).Scan(&webhookTotal)

	writeJSON(w, http.StatusOK, map[string]any{
		"queue_depth":        queueDepth,
		"reviews_today":      reviewsToday,
		"reviews_week":       reviewsWeek,
		"reviews_month":      reviewsMonth,
		"latency_p50_ms":     latencies.P50,
		"latency_p95_ms":     latencies.P95,
		"latency_p99_ms":     latencies.P99,
		"webhook_errors_24h": webhookErrors,
		"webhook_total_24h":  webhookTotal,
	})
}
