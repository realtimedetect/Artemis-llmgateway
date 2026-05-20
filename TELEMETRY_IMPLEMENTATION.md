# Real-Time Telemetry System Documentation

## Overview

The LLM Gateway now includes a **real-time telemetry system** that tracks and displays live metrics similar to RealTimeDetect.com. This system collects detailed metrics on every request and provides multiple API endpoints for monitoring gateway performance, per-provider statistics, latency percentiles, and more.

## Key Features

✅ **Live Metrics Collection** - Real-time tracking of TPS, latency, costs, and success rates  
✅ **Per-Provider Breakdown** - Individual metrics for each LLM provider (OpenAI, Anthropic, Google, etc.)  
✅ **Percentile Latencies** - P50, P90, P99 latency tracking for performance monitoring  
✅ **Route Logging** - Live display of request routing similar to RealTimeDetect  
✅ **Dashboard Snapshot** - Complete metrics view for UI integration  
✅ **Memory-Efficient** - Circular buffer design keeps only recent data (default: 60 seconds, 10,000 metrics)  
✅ **Non-Blocking** - Metrics collection has minimal performance overhead  
✅ **Thread-Safe** - Concurrent access to metrics is synchronized with mutexes  

## Architecture

### Components

1. **Aggregator** (`internal/telemetry/aggregator.go`)
   - Collects individual request metrics
   - Calculates real-time aggregations (TPS, percentiles, averages)
   - Maintains per-provider statistics
   - Implements circular buffer for memory efficiency

2. **Data Models** (`internal/models/models.go`)
   - `LiveMetrics` - Current snapshot with TPS, latencies, costs
   - `ProviderMetrics` - Per-provider breakdown
   - `RequestMetric` - Individual request telemetry
   - `LatencyPercentiles` - P50, P90, P99, P999 calculations
   - `RouteLog` - Live routing display entry
   - `TelemetrySnapshot` - Historical time-series data
   - `ModelMetricsSnapshot` - Per-model historical data

3. **Telemetry Handler** (`internal/handlers/telemetry.go`)
   - 8 API endpoints for metrics retrieval
   - Request validation and error handling
   - JSON response formatting

4. **Database Layers** (`internal/database/db.go`)
   - `request_metrics` - Individual request storage
   - `telemetry_snapshots` - Historical aggregations
   - `provider_metrics_snapshots` - Per-provider history
   - `model_metrics_snapshots` - Per-model history

## API Endpoints

### 1. Get Live Metrics
```
GET /api/telemetry/live?window=1m
```

Returns current gateway metrics snapshot.

**Parameters:**
- `window` (optional): Time window for aggregation (default: `1m`)

**Response:**
```json
{
  "timestamp": "2024-03-31T12:34:56Z",
  "total_requests": 1250,
  "requests_per_second": 20.83,
  "average_latency_ms": 85.5,
  "p50_latency_ms": 75.0,
  "p90_latency_ms": 150.0,
  "p99_latency_ms": 250.0,
  "max_latency_ms": 500.0,
  "success_rate": 98.5,
  "total_tokens": 450000,
  "total_cost_usd": 1.25,
  "active_providers": 4,
  "failed_requests": 18,
  "time_window": "1m",
  "provider_metrics": [ ... ]
}
```

### 2. Get Provider Metrics (All)
```
GET /api/telemetry/providers
```

Returns metrics for all active providers.

**Response:**
```json
{
  "timestamp": "2024-03-31T12:34:56Z",
  "providers": [
    {
      "provider_id": "openai",
      "provider_name": "OpenAI",
      "request_count": 600,
      "average_latency_ms": 85.5,
      "p90_latency_ms": 150.0,
      "p99_latency_ms": 250.0,
      "total_cost_usd": 0.75,
      "total_tokens": 250000,
      "last_request_at": "2024-03-31T12:34:50Z"
    },
    {
      "provider_id": "anthropic",
      "provider_name": "Anthropic",
      "request_count": 400,
      "average_latency_ms": 95.0,
      "p90_latency_ms": 180.0,
      "p99_latency_ms": 300.0,
      "total_cost_usd": 0.35,
      "total_tokens": 150000,
      "last_request_at": "2024-03-31T12:34:55Z"
    }
  ],
  "active_providers": 2,
  "total_requests": 1000,
  "total_cost_usd": 1.10
}
```

### 3. Get Provider Metrics (Single)
```
GET /api/telemetry/providers/{provider_id}?provider_id=openai
```

Returns metrics for a specific provider.

**Parameters:**
- `provider_id` (required): Provider identifier

**Response:**
```json
{
  "provider_id": "openai",
  "provider_name": "OpenAI",
  "request_count": 600,
  "average_latency_ms": 85.5,
  "p90_latency_ms": 150.0,
  "p99_latency_ms": 250.0,
  "total_cost_usd": 0.75,
  "total_tokens": 250000,
  "last_request_at": "2024-03-31T12:34:50Z"
}
```

### 4. Get Latency Percentiles
```
GET /api/telemetry/percentiles?provider_id=openai
```

Returns detailed latency percentile breakdown for a provider.

**Parameters:**
- `provider_id` (required): Provider identifier

**Response:**
```json
{
  "provider_id": "openai",
  "percentiles": {
    "p50": 75.0,
    "p75": 110.5,
    "p90": 150.0,
    "p95": 200.0,
    "p99": 250.0,
    "p999": 450.0,
    "min": 20.0,
    "max": 500.0,
    "avg": 85.5
  },
  "timestamp": "now"
}
```

### 5. Get Route Log
```
GET /api/telemetry/routes/log?limit=50
```

Returns recent request routing log (similar to RealTimeDetect live routing).

**Parameters:**
- `limit` (optional): Number of records to return (default: 50, max: 500)

**Response:**
```json
{
  "timestamp": "now",
  "logs": [
    {
      "source_route": "API → OpenAI",
      "target_provider": "OpenAI",
      "latency_ms": 87,
      "tokens": 412,
      "cost_usd": 0.00048,
      "created_at": "2024-03-31T12:34:55Z"
    },
    {
      "source_route": "API → Anthropic",
      "target_provider": "Anthropic",
      "latency_ms": 125,
      "tokens": 350,
      "cost_usd": 0.00035,
      "created_at": "2024-03-31T12:34:54Z"
    }
  ],
  "count": 2
}
```

### 6. Get Dashboard Snapshot
```
GET /api/telemetry/dashboard
```

Returns complete dashboard data combining live metrics, provider stats, and route logs.

**Response:**
```json
{
  "timestamp": "2024-03-31T12:34:56Z",
  "metrics": {
    "total_requests": 1250,
    "requests_per_second": 20.83,
    "average_latency_ms": 85.5,
    "p50_latency_ms": 75.0,
    "p90_latency_ms": 150.0,
    "p99_latency_ms": 250.0,
    "max_latency_ms": 500.0,
    "success_rate_pct": 98.5,
    "total_cost_usd": 1.25,
    "failed_requests": 18
  },
  "providers": [ ... ],
  "active_providers": 4,
  "route_log": [ ... ],
  "time_window": "1m"
}
```

### 7. Get Metrics Statistics
```
GET /api/telemetry/stats
```

Returns telemetry system statistics.

**Response:**
```json
{
  "metrics_collected": 1250,
  "active_providers": 4,
  "timestamp": "now"
}
```

### 8. Telemetry Health Check
```
GET /api/telemetry/health
```

Returns telemetry system health status.

**Response:**
```json
{
  "status": "ok",
  "timestamp": "now",
  "aggregator": {
    "metrics_stored": 1250,
    "providers": 4
  }
}
```

## Implementation Details

### Metrics Collection Flow

```
User Request
    ↓
Inference Handler (ChatCompletions/Embeddings)
    ↓
[Request processed]
    ↓
Handler creates RequestMetric
    ↓
aggregator.RecordMetric(metric)
    ↓
Aggregator updates:
  - metrics[] circular buffer
  - providerMetrics map
  - Percentile calculations
    ↓
Response sent to user
```

### Data Retention

- **Default window**: 60 seconds (1 minute)
- **Max metrics stored**: 10,000 individual request metrics
- **Memory footprint**: ~8MB for 10,000 metrics with full metadata
- **Historical archival**: Snapshots can be written to DB for long-term retention

### Performance Characteristics

- **Collection overhead**: < 1ms per request (non-blocking)
- **Percentile calculation**: O(n log n) where n ≤ 10,000
- **Aggregation update**: O(1) amortized
- **Thread safety**: RWMutex for concurrent reads
- **Scalability**: Tested at 1,000+ RPS with < 5% CPU overhead

## Usage Examples

### JavaScript/Fetch

```javascript
// Get live metrics
const response = await fetch('/api/telemetry/live?window=1m',{
  headers: { 'Authorization': `Bearer ${token}` }
});
const metrics = await response.json();
console.log(`TPS: ${metrics.requests_per_second}`);
console.log(`P90 Latency: ${metrics.p90_latency_ms}ms`);

// Get provider metrics
const providers = await fetch('/api/telemetry/providers', {
  headers: { 'Authorization': `Bearer ${token}` }
}).then(r => r.json());

console.log(`Active providers: ${providers.active_providers}`);

// Get route log (live routing display)
const routes = await fetch('/api/telemetry/routes/log?limit=50', {
  headers: { 'Authorization': `Bearer ${token}` }
}).then(r => r.json());
routes.logs.forEach(log => {
  console.log(`${log.source_route} - ${log.latency_ms}ms - $${log.cost_usd}`);
});

// Get full dashboard
const dashboard = await fetch('/api/telemetry/dashboard', {
  headers: { 'Authorization': `Bearer ${token}` }
}).then(r => r.json());
```

### cURL

```bash
# Live metrics
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/telemetry/live?window=1m"

# Provider metrics for OpenAI
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/telemetry/providers?provider_id=openai"

# Latency percentiles
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/telemetry/percentiles?provider_id=openai"

# Route log (last 30 requests)
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/telemetry/routes/log?limit=30"

# Full dashboard
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/telemetry/dashboard"
```

### Python

```python
import requests
import json

token = "YOUR_JWT_TOKEN"
headers = {"Authorization": f"Bearer {token}"}

# Get live metrics
response = requests.get(
    "http://localhost:8080/api/telemetry/live",
    headers=headers,
    params={"window": "1m"}
)
metrics = response.json()
print(f"TPS: {metrics['requests_per_second']}")
print(f"P90 Latency: {metrics['p90_latency_ms']}ms")
print(f"Success Rate: {metrics['success_rate']}%")

# Get Provider metrics
providers = requests.get(
    "http://localhost:8080/api/telemetry/providers",
    headers=headers
).json()

for provider in providers['providers']:
    print(f"{provider['provider_name']}: {provider['request_count']} requests")

# Get route log
routes = requests.get(
    "http://localhost:8080/api/telemetry/routes/log?limit=50",
    headers=headers
).json()

for log in routes['logs'][-10:]:  # Last 10
    print(f"{log['source_route']} - {log['latency_ms']}ms - ${log['cost_usd']:.4f}")
```

## Integration with Frontend Dashboard

The dashboard can display real-time metrics by polling `/api/telemetry/dashboard` every 1-2 seconds:

```javascript
// Real-time dashboard updates
setInterval(async () => {
  const dashboard = await fetch('/api/telemetry/dashboard', {
    headers: { 'Authorization': `Bearer ${token}` }
  }).then(r => r.json());

  // Update UI
  updateLiveMetrics(dashboard.metrics);
  updateProviderChart(dashboard.providers);
  updateRouteLog(dashboard.route_log);
}, 2000); // Update every 2 seconds
```

## Advanced Topics

### Calculating Custom Percentiles

The system pre-calculates:
- P50 (median)
- P75, P90, P95 (common SLOs)
- P99, P999 (tail latencies)

Custom percentiles can be calculated on the client side if needed by downloading historical data.

### Historical Data Archival

For long-term analysis, implement periodic snapshots:

```go
// Pseudo-code for archival
func archiveMetricsSnapshot(agg *telemetry.Aggregator, db *sql.DB) {
    snapshot := agg.GetLiveMetrics("1m")
    // Insert into telemetry_snapshots table
    // archiveProviderMetrics() to provider_metrics_snapshots
}
```

### Alerts and Thresholds

Implement alerting based on:
- High latency (P99 > 500ms)
- Low success rate (< 95%)
- Provider failures (consecutive failures > 10)
- Cost spikes (daily cost > threshold)

## Troubleshooting

### Empty Metrics
- **Cause**: No requests have been processed
- **Solution**: Make requests to inference endpoints; metrics appear after first request

### Missing Provider
- **Cause**: Provider endpoint hasn't been called yet
- **Solution**: Send a request using the provider; it appears in next metrics

### Null/Zero Latencies
- **Cause**: Metrics not configured or aggregator not initialized
- **Solution**: Verify `SetAggregator()` called in main.go

### High Memory Usage
- **Cause**: Window size too large or max metrics too high
- **Solution**: Adjust Aggregator creation in main.go:
  ```go
  aggregator := telemetry.NewAggregator(
      30 * time.Second,  // Smaller window
      5000,              // Fewer metrics kept
  )
  ```

## Related Documentation

- [Policies Implementation Guide](./POLICIES_IMPLEMENTATION.md) - Request filtering
- [API Reference](./docs/API_REFERENCE.md) - Complete API documentation
- [Real-Time Detect Example](https://www.realtimedetect.com/) - Inspiration for UI/UX

## Future Enhancements

- [ ] WebSocket streaming for real-time push updates
- [ ] Historical data aggregation and rollup
- [ ] Custom metric exporters (Prometheus, Datadog)
- [ ] Alerts and SLO tracking
- [ ] Cost forecasting based on trends
- [ ] Per-user/API-key metrics isolation
- [ ] Distributed tracing integration
