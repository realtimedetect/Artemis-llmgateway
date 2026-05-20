package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"llm-gatway/internal/middleware"
	"llm-gatway/internal/models"
)

func (h *Handler) GetRoutingConfig(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	cfg, err := h.getRoutingConfigByUser(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if cfg == nil {
		cfg = defaultRoutingConfig(userID)
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *Handler) UpsertRoutingConfig(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	var body struct {
		SmartEnabled        bool    `json:"smart_enabled"`
		CostWeight          float64 `json:"cost_weight"`
		PerformanceWeight   float64 `json:"performance_weight"`
		ComplexityThreshold int     `json:"complexity_threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.CostWeight < 0 || body.CostWeight > 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cost_weight must be between 0 and 1"})
		return
	}
	if body.PerformanceWeight < 0 || body.PerformanceWeight > 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "performance_weight must be between 0 and 1"})
		return
	}
	if body.CostWeight+body.PerformanceWeight <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cost_weight + performance_weight must be greater than zero"})
		return
	}
	if body.ComplexityThreshold < 200 || body.ComplexityThreshold > 20000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "complexity_threshold must be between 200 and 20000"})
		return
	}

	cfg := models.RoutingConfig{
		UserID:              userID,
		SmartEnabled:        body.SmartEnabled,
		CostWeight:          body.CostWeight,
		PerformanceWeight:   body.PerformanceWeight,
		ComplexityThreshold: body.ComplexityThreshold,
	}

	current, err := h.getRoutingConfigByUser(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	if current == nil {
		cfg.ID = uuid.NewString()
		_, err = h.db.ExecContext(r.Context(),
			`INSERT INTO routing_configs (id, user_id, smart_enabled, cost_weight, performance_weight, complexity_threshold)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			cfg.ID, cfg.UserID, cfg.SmartEnabled, cfg.CostWeight, cfg.PerformanceWeight, cfg.ComplexityThreshold,
		)
	} else {
		cfg.ID = current.ID
		_, err = h.db.ExecContext(r.Context(),
			`UPDATE routing_configs
			 SET smart_enabled=?, cost_weight=?, performance_weight=?, complexity_threshold=?
			 WHERE user_id=?`,
			cfg.SmartEnabled, cfg.CostWeight, cfg.PerformanceWeight, cfg.ComplexityThreshold, userID,
		)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusOK, cfg)
}

func defaultRoutingConfig(userID string) *models.RoutingConfig {
	return &models.RoutingConfig{
		UserID:              userID,
		SmartEnabled:        false,
		CostWeight:          0.7,
		PerformanceWeight:   0.3,
		ComplexityThreshold: 1200,
	}
}

func (h *Handler) getRoutingConfigByUser(ctx context.Context, userID string) (*models.RoutingConfig, error) {
	var cfg models.RoutingConfig
	err := h.db.QueryRowContext(ctx,
		`SELECT id, user_id, smart_enabled, cost_weight, performance_weight, complexity_threshold, created_at, updated_at
		 FROM routing_configs WHERE user_id = ?`, userID,
	).Scan(
		&cfg.ID, &cfg.UserID, &cfg.SmartEnabled, &cfg.CostWeight, &cfg.PerformanceWeight, &cfg.ComplexityThreshold, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if cfg.CostWeight < 0 {
		cfg.CostWeight = 0.7
	}
	if cfg.PerformanceWeight < 0 {
		cfg.PerformanceWeight = 0.3
	}
	if cfg.ComplexityThreshold <= 0 {
		cfg.ComplexityThreshold = 1200
	}
	return &cfg, nil
}

func (h *Handler) orderProvidersForSmartRouting(ctx context.Context, userID, model string, bodyBytes []byte, routeMatched bool, candidates []models.Provider) []models.Provider {
	if routeMatched || len(candidates) <= 1 {
		return candidates
	}

	cfg, err := h.getRoutingConfigByUser(ctx, userID)
	if err != nil || cfg == nil || !cfg.SmartEnabled {
		return candidates
	}

	complexity := estimateRequestComplexity(bodyBytes, cfg.ComplexityThreshold)
	type scoreItem struct {
		provider models.Provider
		score    float64
	}

	items := make([]scoreItem, 0, len(candidates))
	costVals := make([]float64, 0, len(candidates))
	latVals := make([]float64, 0, len(candidates))

	for _, provider := range candidates {
		costRate := h.providerCostRatePer1M(ctx, userID, provider.ID, model)
		latency := h.providerLatencyEstimateMs(ctx, userID, provider.ID, model)
		costVals = append(costVals, costRate)
		latVals = append(latVals, latency)
		items = append(items, scoreItem{provider: provider})
	}

	costNorm := normalizedValues(costVals)
	latNorm := normalizedValues(latVals)

	costWeight := cfg.CostWeight * (1.0 - complexity)
	perfWeight := cfg.PerformanceWeight * complexity
	weightSum := costWeight + perfWeight
	if weightSum == 0 {
		costWeight = 0.7
		perfWeight = 0.3
		weightSum = 1.0
	}

	for i := range items {
		items[i].score = (costNorm[i]*costWeight + latNorm[i]*perfWeight) / weightSum
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].score < items[j].score
	})

	ordered := make([]models.Provider, 0, len(items))
	for _, item := range items {
		ordered = append(ordered, item.provider)
	}
	return ordered
}

func estimateRequestComplexity(bodyBytes []byte, threshold int) float64 {
	if threshold <= 0 {
		threshold = 1200
	}

	var raw map[string]any
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return 0.5
	}

	text := extractSemanticPromptText(bodyBytes)
	approxPromptTokens := len([]rune(text)) / 4
	if approxPromptTokens < 1 {
		approxPromptTokens = 1
	}
	maxTokens := intFromAny(raw["max_tokens"])
	if maxTokens < 0 {
		maxTokens = 0
	}
	total := approxPromptTokens + maxTokens
	if total <= 0 {
		return 0
	}
	return math.Min(1.0, float64(total)/float64(threshold))
}

func normalizedValues(values []float64) []float64 {
	if len(values) == 0 {
		return values
	}
	minVal := values[0]
	maxVal := values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	result := make([]float64, len(values))
	if math.Abs(maxVal-minVal) < 1e-9 {
		return result
	}
	for i, v := range values {
		result[i] = (v - minVal) / (maxVal - minVal)
	}
	return result
}

func (h *Handler) providerCostRatePer1M(ctx context.Context, userID, providerID, model string) float64 {
	var inputRate, outputRate float64
	if err := h.db.QueryRowContext(ctx,
		`SELECT input_cost_per_1m, output_cost_per_1m
		 FROM model_costs WHERE user_id = ? AND provider_id = ? AND model = ?`,
		userID, providerID, model,
	).Scan(&inputRate, &outputRate); err == nil {
		return inputRate + outputRate
	}
	return 1000
}

func (h *Handler) providerLatencyEstimateMs(ctx context.Context, userID, providerID, model string) float64 {
	lookback := time.Now().Add(-24 * time.Hour)
	var avgLatency sql.NullFloat64
	_ = h.db.QueryRowContext(ctx,
		`SELECT AVG(latency_ms) FROM requests
		 WHERE user_id = ? AND provider_id = ? AND model = ? AND created_at >= ?`,
		userID, providerID, model, lookback,
	).Scan(&avgLatency)
	if avgLatency.Valid && avgLatency.Float64 > 0 {
		return avgLatency.Float64
	}

	_ = h.db.QueryRowContext(ctx,
		`SELECT AVG(latency_ms) FROM requests
		 WHERE user_id = ? AND provider_id = ? AND created_at >= ?`,
		userID, providerID, lookback,
	).Scan(&avgLatency)
	if avgLatency.Valid && avgLatency.Float64 > 0 {
		return avgLatency.Float64
	}
	return 1200
}

func extractSemanticPromptText(bodyBytes []byte) string {
	var raw struct {
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return ""
	}

	for i := len(raw.Messages) - 1; i >= 0; i-- {
		msg := raw.Messages[i]
		if strings.EqualFold(strings.TrimSpace(msg.Role), "user") {
			return stringifyUnifiedMessageContent(msg.Content)
		}
	}
	if len(raw.Messages) > 0 {
		return stringifyUnifiedMessageContent(raw.Messages[len(raw.Messages)-1].Content)
	}
	return ""
}

func cacheSemanticIndexKey(cfg models.CacheConfig, userID, model string) string {
	prefix := strings.TrimSpace(cfg.KeyPrefix)
	if prefix == "" {
		prefix = "llm-gw"
	}
	return prefix + ":" + userID + ":" + model + ":semantic:index"
}

func (h *Handler) getSemanticCachedChatCompletion(
	ctx context.Context,
	userID string,
	apiKeyID *string,
	requestID string,
	routeSlug string,
	model string,
	queryText string,
	providers []models.Provider,
) (*cachePayload, bool, []float64, error) {
	cfg, err := h.getCacheConfigByUser(ctx, userID)
	if err != nil || cfg == nil || !cfg.Enabled || !cfg.SemanticEnabled {
		return nil, false, nil, err
	}
	if strings.TrimSpace(queryText) == "" {
		return nil, false, nil, nil
	}

	embedding, err := h.getSemanticEmbedding(ctx, userID, apiKeyID, requestID, routeSlug, model, cfg.SemanticEmbeddingModel, queryText, providers)
	if err != nil || len(embedding) == 0 {
		return nil, false, nil, err
	}

	client := h.newRedisClient(*cfg)
	defer client.Close()
	indexKey := cacheSemanticIndexKey(*cfg, userID, model)
	keys, err := client.LRange(ctx, indexKey, 0, int64(cfg.SemanticMaxCandidates-1)).Result()
	if err != nil || len(keys) == 0 {
		return nil, false, embedding, err
	}

	rawItems, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, false, embedding, err
	}

	bestScore := -1.0
	var bestPayload *cachePayload
	for _, item := range rawItems {
		if item == nil {
			continue
		}
		var payloadBytes []byte
		switch typed := item.(type) {
		case string:
			payloadBytes = []byte(typed)
		case []byte:
			payloadBytes = typed
		default:
			continue
		}
		var payload cachePayload
		if json.Unmarshal(payloadBytes, &payload) != nil {
			continue
		}
		if len(payload.PromptEmbedding) == 0 {
			continue
		}
		score := cosineSimilarity(embedding, payload.PromptEmbedding)
		if score > bestScore {
			bestScore = score
			copyPayload := payload
			bestPayload = &copyPayload
		}
	}

	if bestPayload != nil && bestScore >= cfg.SemanticThreshold {
		return bestPayload, true, embedding, nil
	}
	return nil, false, embedding, nil
}

func (h *Handler) getSemanticEmbedding(
	ctx context.Context,
	userID string,
	apiKeyID *string,
	requestID string,
	routeSlug string,
	chatModel string,
	embeddingModel string,
	queryText string,
	providers []models.Provider,
) ([]float64, error) {
	body := map[string]any{
		"model": embeddingModel,
		"input": queryText,
	}
	bodyBytes, _ := json.Marshal(body)
	providerPtr, resp, _, err := h.executeProviderRequest(ctx, userID, apiKeyID, requestID, routeSlug, chatModel, providers, "/embeddings", bodyBytes, 45*time.Second, false)
	if err != nil || providerPtr == nil || resp == nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil
	}
	respBytes, _ := io.ReadAll(resp.Body)
	return parseEmbeddingVector(respBytes), nil
}

func parseEmbeddingVector(respBytes []byte) []float64 {
	var payload struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if json.Unmarshal(respBytes, &payload) != nil {
		return nil
	}
	if len(payload.Data) == 0 {
		return nil
	}
	return payload.Data[0].Embedding
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return -1
	}
	maxLen := len(a)
	if len(b) < maxLen {
		maxLen = len(b)
	}
	dot := 0.0
	normA := 0.0
	normB := 0.0
	for i := 0; i < maxLen; i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return -1
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
