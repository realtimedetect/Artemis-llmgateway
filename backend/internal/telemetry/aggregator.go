package telemetry

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"llm-gatway/internal/models"
)

// Aggregator collects and calculates real-time metrics
type Aggregator struct {
	mu               sync.RWMutex
	metrics          []models.RequestMetric
	maxMetricsSize   int    // Keep last N metrics in memory
	windowDuration   time.Duration
	providerMetrics  map[string]*ProviderStats
	lastAggregation  time.Time
	aggregationMutex sync.Mutex
}

// ProviderStats tracks per-provider statistics
type ProviderStats struct {
	ProviderID       string
	ProviderName     string
	RequestCount     int64
	SuccessCount     int64
	FailureCount     int64
	TotalLatency     int64
	Latencies        []int
	TotalTokens      int64
	TotalCost        float64
	LastRequestTime  time.Time
}

// NewAggregator creates a new telemetry aggregator
func NewAggregator(windowDuration time.Duration, maxMetricsSize int) *Aggregator {
	return &Aggregator{
		metrics:         make([]models.RequestMetric, 0, maxMetricsSize),
		maxMetricsSize:  maxMetricsSize,
		windowDuration:  windowDuration,
		providerMetrics: make(map[string]*ProviderStats),
		lastAggregation: time.Now(),
	}
}

// RecordMetric adds a new request metric
func (a *Aggregator) RecordMetric(metric models.RequestMetric) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Keep metrics within the window
	now := time.Now()
	cutoff := now.Add(-a.windowDuration)

	// Remove old metrics
	filtered := make([]models.RequestMetric, 0)
	for _, m := range a.metrics {
		if m.CreatedAt.After(cutoff) {
			filtered = append(filtered, m)
		}
	}
	filtered = append(filtered, metric)

	// Cap metrics to max size (keep most recent)
	if len(filtered) > a.maxMetricsSize {
		filtered = filtered[len(filtered)-a.maxMetricsSize:]
	}

	a.metrics = filtered

	// Update provider stats
	a.updateProviderStats(metric)
}

// updateProviderStats updates per-provider statistics
func (a *Aggregator) updateProviderStats(metric models.RequestMetric) {
	key := metric.ProviderID

	if _, exists := a.providerMetrics[key]; !exists {
		a.providerMetrics[key] = &ProviderStats{
			ProviderID:   metric.ProviderID,
			ProviderName: metric.ProviderName,
			Latencies:    make([]int, 0),
		}
	}

	stats := a.providerMetrics[key]
	stats.RequestCount++
	stats.TotalLatency += int64(metric.LatencyMs)
	stats.Latencies = append(stats.Latencies, metric.LatencyMs)
	stats.TotalTokens += int64(metric.TotalTokens)
	stats.TotalCost += metric.CostUSD
	stats.LastRequestTime = time.Now()

	if metric.Status >= 200 && metric.Status < 300 {
		stats.SuccessCount++
	} else {
		stats.FailureCount++
	}

	// Keep latencies list bounded
	if len(stats.Latencies) > 10000 {
		stats.Latencies = stats.Latencies[len(stats.Latencies)-10000:]
	}
}

// GetLiveMetrics returns current live metrics snapshot
func (a *Aggregator) GetLiveMetrics(timeWindow string) models.LiveMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	metrics := models.LiveMetrics{
		Timestamp:       time.Now(),
		TimeWindow:      timeWindow,
		TotalRequests:   int64(len(a.metrics)),
		ProviderMetrics: []models.ProviderMetrics{},
	}

	if len(a.metrics) == 0 {
		return metrics
	}

	// Calculate aggregate metrics
	var totalLatency int64
	var totalCost float64
	var successCount int64
	var failureCount int64
	var totalTokens int64
	latencies := make([]int, 0)

	for _, m := range a.metrics {
		totalLatency += int64(m.LatencyMs)
		totalCost += m.CostUSD
		totalTokens += int64(m.TotalTokens)
		latencies = append(latencies, m.LatencyMs)

		if m.Status >= 200 && m.Status < 300 {
			successCount++
		} else {
			failureCount++
		}
	}

	metrics.TotalCostUSD = totalCost
	metrics.AverageLatencyMs = float64(totalLatency) / float64(len(a.metrics))
	metrics.FailedRequests = failureCount
	metrics.TotalTokens = totalTokens

	// Calculate percentiles
	if len(latencies) > 0 {
		sort.Ints(latencies)
		metrics.P50LatencyMs = float64(percentile(latencies, 50))
		metrics.P90LatencyMs = float64(percentile(latencies, 90))
		metrics.P99LatencyMs = float64(percentile(latencies, 99))
		metrics.MaxLatencyMs = float64(latencies[len(latencies)-1])
	}

	// Success rate
	if metrics.TotalRequests > 0 {
		metrics.SuccessRate = (float64(successCount) / float64(metrics.TotalRequests)) * 100
	}

	// Calculate requests per second
	timeSpan := time.Since(a.lastAggregation).Seconds()
	if timeSpan > 0 {
		metrics.RequestsPerSecond = float64(len(a.metrics)) / timeSpan
	}

	// Add provider metrics
	metrics.ProviderMetrics = a.getProviderMetrics()
	metrics.ActiveProviders = len(a.providerMetrics)

	return metrics
}

// getProviderMetrics calculates metrics for each provider
func (a *Aggregator) getProviderMetrics() []models.ProviderMetrics {
	result := make([]models.ProviderMetrics, 0)

	for _, stats := range a.providerMetrics {
		pm := models.ProviderMetrics{
			ProviderID:   stats.ProviderID,
			ProviderName: stats.ProviderName,
			RequestCount: stats.RequestCount,
			SuccessCount: stats.SuccessCount,
			FailureCount: stats.FailureCount,
			TotalTokens:  stats.TotalTokens,
			TotalCostUSD: stats.TotalCost,
		}

		if stats.RequestCount > 0 {
			pm.AverageLatencyMs = float64(stats.TotalLatency) / float64(stats.RequestCount)
		}

		if len(stats.Latencies) > 0 {
			latencies := make([]int, len(stats.Latencies))
			copy(latencies, stats.Latencies)
			sort.Ints(latencies)

			pm.P50LatencyMs = float64(percentile(latencies, 50))
			pm.P90LatencyMs = float64(percentile(latencies, 90))
			pm.P99LatencyMs = float64(percentile(latencies, 99))
		}

		if !stats.LastRequestTime.IsZero() {
			pm.LastRequestAt = stats.LastRequestTime
		}

		result = append(result, pm)
	}

	return result
}

// GetLatencyPercentiles calculates percentiles for a provider
func (a *Aggregator) GetLatencyPercentiles(providerID string) (models.LatencyPercentiles, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats, exists := a.providerMetrics[providerID]
	if !exists {
		return models.LatencyPercentiles{}, fmt.Errorf("provider not found: %s", providerID)
	}

	if len(stats.Latencies) == 0 {
		return models.LatencyPercentiles{}, nil
	}

	latencies := make([]int, len(stats.Latencies))
	copy(latencies, stats.Latencies)
	sort.Ints(latencies)

	var total int64
	for _, l := range latencies {
		total += int64(l)
	}

	return models.LatencyPercentiles{
		P50:  float64(percentile(latencies, 50)),
		P75:  float64(percentile(latencies, 75)),
		P90:  float64(percentile(latencies, 90)),
		P95:  float64(percentile(latencies, 95)),
		P99:  float64(percentile(latencies, 99)),
		P999: float64(percentile(latencies, 99.9)),
		Min:  float64(latencies[0]),
		Max:  float64(latencies[len(latencies)-1]),
		Avg:  float64(total) / float64(len(latencies)),
	}, nil
}

// GetProviderMetrics gets metrics for a single provider
func (a *Aggregator) GetProviderMetrics(providerID string) (models.ProviderMetrics, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats, exists := a.providerMetrics[providerID]
	if !exists {
		return models.ProviderMetrics{}, fmt.Errorf("provider not found: %s", providerID)
	}

	pm := models.ProviderMetrics{
		ProviderID:   stats.ProviderID,
		ProviderName: stats.ProviderName,
		RequestCount: stats.RequestCount,
		SuccessCount: stats.SuccessCount,
		FailureCount: stats.FailureCount,
		TotalTokens:  stats.TotalTokens,
		TotalCostUSD: stats.TotalCost,
	}

	if stats.RequestCount > 0 {
		pm.AverageLatencyMs = float64(stats.TotalLatency) / float64(stats.RequestCount)
	}

	if len(stats.Latencies) > 0 {
		latencies := make([]int, len(stats.Latencies))
		copy(latencies, stats.Latencies)
		sort.Ints(latencies)

		pm.P50LatencyMs = float64(percentile(latencies, 50))
		pm.P90LatencyMs = float64(percentile(latencies, 90))
		pm.P99LatencyMs = float64(percentile(latencies, 99))
	}

	if !stats.LastRequestTime.IsZero() {
		pm.LastRequestAt = stats.LastRequestTime
	}

	return pm, nil
}

// GetRecentMetrics returns recent metrics for dashboard
func (a *Aggregator) GetRecentMetrics(limit int) []models.RequestMetric {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit > len(a.metrics) {
		limit = len(a.metrics)
	}

	// Return most recent metrics
	result := make([]models.RequestMetric, limit)
	copy(result, a.metrics[len(a.metrics)-limit:])

	// Reverse to get newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// percentile calculates the Nth percentile of sorted data
func percentile(sorted []int, p float64) int {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	index := (p / 100.0) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	fraction := index - float64(lower)
	return int(float64(sorted[lower])*(1-fraction) + float64(sorted[upper])*fraction)
}

// Reset clears all metrics (optional, for testing)
func (a *Aggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.metrics = make([]models.RequestMetric, 0, a.maxMetricsSize)
	a.providerMetrics = make(map[string]*ProviderStats)
	a.lastAggregation = time.Now()
}

// GetMetricsCount returns total metrics stored
func (a *Aggregator) GetMetricsCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return len(a.metrics)
}

// GetProviderCount returns number of active providers
func (a *Aggregator) GetProviderCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return len(a.providerMetrics)
}
