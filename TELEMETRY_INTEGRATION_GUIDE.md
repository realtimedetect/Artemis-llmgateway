# Telemetry - Metrics Collection Integration Guide

## Overview

This guide explains how to integrate metrics collection into the existing ChatCompletions and Embeddings handlers to enable real-time telemetry tracking.

## Architecture

```
Request Arrives
    ↓
[Authentication & Policy Check]
    ↓
Handler starts timer (start := time.Now())
    ↓
[Process request to LLM provider]
    ↓
Calculate:
  - latency := time.Since(start).Milliseconds()
  - status code
  - tokens from response
  - cost from tokens + model pricing
    ↓
Create RequestMetric struct
    ↓
h.aggregator.RecordMetric(metric)
    ↓
Send response to client
```

## Integration Points

### 1. ChatCompletions Handler

**Location**: `internal/handlers/handler.go` → `ChatCompletions()` method

**Steps to add metrics collection**:

```go
func (h *Handler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...
    
    // 1. Start timer AT THE BEGINNING before decode
    startTime := time.Now()
    
    // 2. Parse request (existing code)
    var req openaiRequest // existing type
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
        return
    }
    
    // ... authentication, routing, provider selection (existing code) ...
    
    userID := r.Context().Value("user_id").(string)
    apiKeyID := r.Context().Value("api_key_id").(string) // May be nil if JWT auth
    
    // 3. Make request to provider (existing code)
    respBody, StatusCode, errMsg := h.proxyToProvider(...)
    
    // 4. Extract metrics from response
    latencyMs := int(time.Since(startTime).Milliseconds())
    totalTokens := extractTokensFromResponse(respBody) // Parse response
    cost := calculateTokenCost(providerID, model, totalTokens)
    
    // 5. Create and record metric
    if h.aggregator != nil {
        metric := models.RequestMetric{
            ProviderID:   providerID,
            ProviderName: providerName,
            ModelName:    model,
            Endpoint:     "/api/chat/completions",
            LatencyMs:    latencyMs,
            TotalTokens:  totalTokens,
            CostUSD:      cost,
            Status:       StatusCode,
            CreatedAt:    startTime,
        }
        if errMsg != "" {
            metric.ErrorMessage = errMsg
        }
        h.aggregator.RecordMetric(metric)
    }
    
    // 6. Write response (existing code)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(StatusCode)
    fmt.Fprint(w, respBody)
}
```

### 2. Embeddings Handler

**Location**: `internal/handlers/handler.go` → `Embeddings()` method

**Same pattern as ChatCompletions**:
- Start timer at beginning
- Extract tokens from embedding response
- Calculate cost
- Record metric before returning

### 3. AgentRun Handler

**Location**: `internal/handlers/handler.go` → `AgentRun()` method

**Same pattern**:
- Track total latency for entire agent execution
- Sum tokens from all internal calls
- Sum costs from all providers used
- Record as single aggregate metric

## Helper Functions to Implement

### 1. Extract Tokens from Response

```go
// extractTokensFromResponse parses OpenAI-compatible response
func extractTokensFromResponse(respBody []byte) int {
    var resp map[string]interface{}
    if err := json.Unmarshal(respBody, &resp); err != nil {
        return 0
    }
    
    usage, ok := resp["usage"].(map[string]interface{})
    if !ok {
        return 0
    }
    
    total, ok := usage["total_tokens"].(float64)
    if !ok {
        return 0
    }
    
    return int(total)
}
```

### 2. Calculate Token Cost

```go
// calculateTokenCost computes USD cost from tokens
func (h *Handler) calculateTokenCost(
    providerID string, 
    model string, 
    totalTokens int,
) float64 {
    // Query model_costs table for this user/provider/model
    var inputCost, outputCost float64
    
    err := h.db.QueryRow(`
        SELECT input_cost_per_1m, output_cost_per_1m 
        FROM model_costs 
        WHERE user_id = ? AND provider_id = ? AND model = ?
    `, userID, providerID, model).Scan(&inputCost, &outputCost)
    
    if err != nil {
        // Use default pricing or return 0
        return 0
    }
    
    // Approximate: assume 70% input, 30% output tokens
    inputTokens := float64(totalTokens) * 0.7
    outputTokens := float64(totalTokens) * 0.3
    
    cost := (inputTokens * inputCost / 1e6) + (outputTokens * outputCost / 1e6)
    return cost
}
```

### 3. Extract Provider Name

```go
// getProviderName maps provider ID to display name
func getProviderName(providerID string) string {
    names := map[string]string{
        "openai":     "OpenAI",
        "anthropic":  "Anthropic",
        "google":     "Google",
        "azure":      "Azure",
        "mistral":    "Mistral",
        "llama":      "Llama",
    }
    
    if name, ok := names[providerID]; ok {
        return name
    }
    return strings.ToTitle(providerID)
}
```

## Error Handling

When recording metrics, always check if aggregator is nil and handle gracefully:

```go
if h.aggregator != nil {
    h.aggregator.RecordMetric(metric)
} else {
    log.Println("Warning: aggregator not initialized, metrics not recorded")
}
```

## Async Database Write (Future)

For now, metrics are stored in memory only. To add database persistence:

```go
// Pseudo-code for async database write
go func() {
    err := h.db.Exec(`
        INSERT INTO request_metrics (
            user_id, api_key_id, provider_id, model_name, 
            provider_name, endpoint, latency_ms, total_tokens, 
            cost_usd, status, error_message, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `,
        userID, apiKeyID, metric.ProviderID, metric.ModelName,
        metric.ProviderName, metric.Endpoint, metric.LatencyMs,
        metric.TotalTokens, metric.CostUSD, metric.Status,
        metric.ErrorMessage, metric.CreatedAt,
    )
    if err != nil {
        log.Printf("Error writing metrics to DB: %v", err)
    }
}()
```

## Testing Integration

```go
// Test in your test file
func TestMetricsCollection(t *testing.T) {
    // Create test aggregator
    agg := telemetry.NewAggregator(time.Minute, 100)
    
    // Create handler with aggregator
    h := handlers.New(db)
    h.SetAggregator(agg)
    
    // Make test request
    req := httptest.NewRequest("POST", "/api/chat/completions", ...)
    w := httptest.NewRecorder()
    h.ChatCompletions(w, req)
    
    // Verify metrics collected
    require.Equal(t, 1, agg.GetMetricsCount())
    
    metrics := agg.GetLiveMetrics("1m")
    require.Equal(t, int64(1), metrics.TotalRequests)
    require.Greater(t, metrics.AverageLatencyMs, 0.0)
}
```

## Configuration

The aggregator is initialized in `cmd/server/main.go`:

```go
// Current configuration
aggregator := telemetry.NewAggregator(
    time.Minute,  // 60-second window
    10000,        // Keep up to 10,000 metrics
)
h.SetAggregator(aggregator)
```

To adjust:
- **Smaller window** (30 seconds): More recent data, faster cleanup
- **Larger window** (5 minutes): More historical context, more memory
- **Fewer metrics** (5,000): Lower memory, older data discarded faster
- **More metrics** (20,000): More data, higher memory usage

## Verification Checklist

After integration, verify:

- [ ] POST `/api/chat/completions` records metrics
- [ ] POST `/api/embeddings` records metrics
- [ ] GET `/api/telemetry/live` shows increasing request count
- [ ] Latency values are reasonable (< 10s)
- [ ] Token counts match model responses
- [ ] Cost calculations use correct pricing
- [ ] P90 latency is between avg and max
- [ ] Provider breakdown sums to total
- [ ] Route log shows newest requests first
- [ ] No nil pointer errors in aggregator

## Common Issues

### Issue: Metrics appearing empty
- **Cause**: No requests sent yet
- **Fix**: Send at least one request to a provider

### Issue: Zero latency or tokens
- **Cause**: Extraction logic incorrect
- **Fix**: Verify response parsing matches provider format

### Issue: Incorrect costs
- **Cause**: Model pricing not configured
- **Fix**: Verify model_costs table has entries

### Issue: High memory usage
- **Cause**: Window too large or max metrics too high
- **Fix**: Reduce aggregator parameters

## Next Steps

1. Identify all request handler methods that should record metrics
2. Implement metrics collection in each handler
3. Add token extraction based on provider response format
4. Implement cost calculation using existing pricing data
5. Test with 100+ requests
6. Monitor memory and performance
7. Consider adding database writes for historical analysis
