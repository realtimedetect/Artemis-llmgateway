package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"llm-gatway/internal/middleware"
)

// GetObservabilityMetrics returns aggregate and timeseries observability metrics.
func (h *Handler) GetObservabilityMetrics(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	period := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period")))
	if period == "" {
		period = "1h"
	}

	windowStart := time.Now().Add(-1 * time.Hour)
	switch period {
	case "15m":
		windowStart = time.Now().Add(-15 * time.Minute)
	case "1h":
		windowStart = time.Now().Add(-1 * time.Hour)
	case "24h":
		windowStart = time.Now().Add(-24 * time.Hour)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid period, use 15m|1h|24h"})
		return
	}

	var totalRequests int
	var errorRequests int
	var avgLatency sql.NullFloat64
	var avgTTFT sql.NullFloat64
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT
		   COUNT(*) AS total_requests,
		   COALESCE(SUM(CASE WHEN status >= 400 THEN 1 ELSE 0 END), 0) AS error_requests,
		   COALESCE(AVG(latency_ms), 0) AS avg_latency_ms,
		   COALESCE(AVG(CASE WHEN ttft_ms > 0 THEN ttft_ms END), 0) AS avg_ttft_ms
		 FROM requests
		 WHERE user_id = ? AND created_at >= ?`,
		userID, windowStart,
	).Scan(&totalRequests, &errorRequests, &avgLatency, &avgTTFT); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	p95Latency := h.percentileMetric(r, userID, windowStart, "latency_ms", 0.95)
	p95TTFT := h.percentileMetric(r, userID, windowStart, "ttft_ms", 0.95)

	windowSeconds := time.Since(windowStart).Seconds()
	if windowSeconds <= 0 {
		windowSeconds = 1
	}
	errorRate := 0.0
	if totalRequests > 0 {
		errorRate = float64(errorRequests) / float64(totalRequests)
	}

	timeseriesRows, err := h.db.QueryContext(r.Context(),
		`SELECT DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:00') AS minute_bucket,
		        COUNT(*) AS requests,
		        COALESCE(SUM(CASE WHEN status >= 400 THEN 1 ELSE 0 END), 0) AS errors,
		        COALESCE(AVG(latency_ms), 0) AS avg_latency_ms,
		        COALESCE(AVG(CASE WHEN ttft_ms > 0 THEN ttft_ms END), 0) AS avg_ttft_ms
		 FROM requests
		 WHERE user_id = ? AND created_at >= ?
		 GROUP BY minute_bucket
		 ORDER BY minute_bucket ASC`,
		userID, windowStart,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer timeseriesRows.Close()

	type point struct {
		Minute      string  `json:"minute"`
		Requests    int     `json:"requests"`
		Errors      int     `json:"errors"`
		ErrorRate   float64 `json:"error_rate"`
		AvgLatency  float64 `json:"avg_latency_ms"`
		AvgTTFT     float64 `json:"avg_ttft_ms"`
		ThroughputR float64 `json:"throughput_rps"`
	}
	series := make([]point, 0)
	for timeseriesRows.Next() {
		var minute string
		var reqCount, errCount int
		var avgLat, avgT sql.NullFloat64
		if scanErr := timeseriesRows.Scan(&minute, &reqCount, &errCount, &avgLat, &avgT); scanErr != nil {
			continue
		}
		bucketErrRate := 0.0
		if reqCount > 0 {
			bucketErrRate = float64(errCount) / float64(reqCount)
		}
		series = append(series, point{
			Minute:      minute,
			Requests:    reqCount,
			Errors:      errCount,
			ErrorRate:   bucketErrRate,
			AvgLatency:  nullableFloat(avgLat),
			AvgTTFT:     nullableFloat(avgT),
			ThroughputR: float64(reqCount) / 60.0,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"period": period,
		"summary": map[string]any{
			"total_requests": totalRequests,
			"error_requests": errorRequests,
			"error_rate":     errorRate,
			"throughput_rps": float64(totalRequests) / windowSeconds,
			"avg_latency_ms": nullableFloat(avgLatency),
			"p95_latency_ms": p95Latency,
			"avg_ttft_ms":    nullableFloat(avgTTFT),
			"p95_ttft_ms":    p95TTFT,
		},
		"timeseries": series,
	})
}

func nullableFloat(v sql.NullFloat64) float64 {
	if v.Valid {
		return v.Float64
	}
	return 0
}

func (h *Handler) percentileMetric(r *http.Request, userID string, from time.Time, column string, p float64) int {
	if p <= 0 || p > 1 {
		return 0
	}
	if column != "latency_ms" && column != "ttft_ms" {
		return 0
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM requests WHERE user_id = ? AND created_at >= ? AND ` + column + ` > 0`
	if err := h.db.QueryRowContext(r.Context(), countQuery, userID, from).Scan(&total); err != nil || total == 0 {
		return 0
	}
	offset := int(float64(total-1) * p)
	var value int
	valueQuery := `SELECT ` + column + ` FROM requests WHERE user_id = ? AND created_at >= ? AND ` + column + ` > 0 ORDER BY ` + column + ` ASC LIMIT 1 OFFSET ?`
	if err := h.db.QueryRowContext(r.Context(), valueQuery, userID, from, offset).Scan(&value); err != nil {
		return 0
	}
	return value
}
