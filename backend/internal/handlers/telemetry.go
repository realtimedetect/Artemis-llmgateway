package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"llm-gatway/internal/models"
	"llm-gatway/internal/telemetry"
)

// TelemetryHandler handles telemetry endpoints
type TelemetryHandler struct {
	aggregator *telemetry.Aggregator
}

// NewTelemetryHandler creates a new telemetry handler
func NewTelemetryHandler(aggregator *telemetry.Aggregator) *TelemetryHandler {
	return &TelemetryHandler{
		aggregator: aggregator,
	}
}

// GetLiveMetrics is a method on Handler for live metrics
func (h *Handler) GetLiveMetrics(w http.ResponseWriter, r *http.Request) {
	if h.aggregator == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "aggregator not initialized"})
		return
	}

	timeWindow := r.URL.Query().Get("window")
	if timeWindow == "" {
		timeWindow = "1m"
	}

	metrics := h.aggregator.GetLiveMetrics(timeWindow)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// GetProviderMetrics returns metrics for all providers
func (h *Handler) GetProviderMetrics(w http.ResponseWriter, r *http.Request) {
	if h.aggregator == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "aggregator not initialized"})
		return
	}

	liveMetrics := h.aggregator.GetLiveMetrics("1m")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"timestamp":         liveMetrics.Timestamp,
		"providers":         liveMetrics.ProviderMetrics,
		"active_providers":  liveMetrics.ActiveProviders,
		"total_requests":    liveMetrics.TotalRequests,
		"total_cost_usd":    liveMetrics.TotalCostUSD,
	})
}

// GetProviderMetric returns metrics for a single provider
func (h *Handler) GetProviderMetric(w http.ResponseWriter, r *http.Request) {
	if h.aggregator == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "aggregator not initialized"})
		return
	}

	providerID := r.URL.Query().Get("provider_id")
	if providerID == "" {
		http.Error(w, "provider_id parameter required", http.StatusBadRequest)
		return
	}

	metrics, err := h.aggregator.GetProviderMetrics(providerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// GetLatencyPercentiles returns percentile latency data for a provider
func (h *Handler) GetLatencyPercentiles(w http.ResponseWriter, r *http.Request) {
	if h.aggregator == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "aggregator not initialized"})
		return
	}

	providerID := r.URL.Query().Get("provider_id")
	if providerID == "" {
		http.Error(w, "provider_id parameter required", http.StatusBadRequest)
		return
	}

	percentiles, err := h.aggregator.GetLatencyPercentiles(providerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"provider_id": providerID,
		"percentiles": percentiles,
		"timestamp":   "now",
	})
}

// GetRouteLog returns recent request routing
func (h *Handler) GetRouteLog(w http.ResponseWriter, r *http.Request) {
	if h.aggregator == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "aggregator not initialized"})
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			if l > 500 {
				l = 500 // Cap at 500
			}
			limit = l
		}
	}

	metrics := h.aggregator.GetRecentMetrics(limit)
	routeLogs := make([]models.RouteLog, len(metrics))

	for i, m := range metrics {
		routeLogs[i] = models.RouteLog{
			SourceRoute:    fmt.Sprintf("API → %s", m.ProviderName),
			TargetProvider: m.ProviderName,
			LatencyMs:      m.LatencyMs,
			Tokens:         m.TotalTokens,
			CostUSD:        m.CostUSD,
			CreatedAt:      m.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"timestamp": "now",
		"logs":      routeLogs,
		"count":     len(routeLogs),
	})
}

// GetDashboardSnapshot returns full dashboard data
func (h *Handler) GetDashboardSnapshot(w http.ResponseWriter, r *http.Request) {
	if h.aggregator == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "aggregator not initialized"})
		return
	}

	liveMetrics := h.aggregator.GetLiveMetrics("1m")
	recentRoutes := h.aggregator.GetRecentMetrics(20)

	routeLogs := make([]models.RouteLog, len(recentRoutes))
	for i, m := range recentRoutes {
		routeLogs[i] = models.RouteLog{
			SourceRoute:    fmt.Sprintf("API → %s", m.ProviderName),
			TargetProvider: m.ProviderName,
			LatencyMs:      m.LatencyMs,
			Tokens:         m.TotalTokens,
			CostUSD:        m.CostUSD,
			CreatedAt:      m.CreatedAt,
		}
	}

	dashboard := map[string]interface{}{
		"timestamp": liveMetrics.Timestamp,
		"metrics": map[string]interface{}{
			"total_requests":      liveMetrics.TotalRequests,
			"requests_per_second": liveMetrics.RequestsPerSecond,
			"average_latency_ms":  liveMetrics.AverageLatencyMs,
			"p50_latency_ms":      liveMetrics.P50LatencyMs,
			"p90_latency_ms":      liveMetrics.P90LatencyMs,
			"p99_latency_ms":      liveMetrics.P99LatencyMs,
			"max_latency_ms":      liveMetrics.MaxLatencyMs,
			"success_rate_pct":    liveMetrics.SuccessRate,
			"total_cost_usd":      liveMetrics.TotalCostUSD,
			"failed_requests":     liveMetrics.FailedRequests,
		},
		"providers": liveMetrics.ProviderMetrics,
		"active_providers": liveMetrics.ActiveProviders,
		"route_log": routeLogs,
		"time_window": liveMetrics.TimeWindow,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboard)
}

// GetMetricsStats returns metrics collection statistics
func (h *Handler) GetMetricsStats(w http.ResponseWriter, r *http.Request) {
	if h.aggregator == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "aggregator not initialized"})
		return
	}

	stats := map[string]interface{}{
		"metrics_collected": h.aggregator.GetMetricsCount(),
		"active_providers":  h.aggregator.GetProviderCount(),
		"timestamp":         "now",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// TelemetryHealthCheck returns telemetry system health status
func (h *Handler) TelemetryHealthCheck(w http.ResponseWriter, r *http.Request) {
	if h.aggregator == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "aggregator not initialized"})
		return
	}

	status := map[string]interface{}{
		"status": "ok",
		"timestamp": "now",
		"aggregator": map[string]interface{}{
			"metrics_stored": h.aggregator.GetMetricsCount(),
			"providers": h.aggregator.GetProviderCount(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
