package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/crypto/bcrypt"

	"llm-gatway/internal/middleware"
	"llm-gatway/internal/models"
	"llm-gatway/internal/telemetry"
)

// Handler holds application-wide dependencies.
type Handler struct {
	db         *sql.DB
	aggregator *telemetry.Aggregator
}

type providerCircuitState struct {
	consecutiveFailures int
	openUntil           time.Time
}

var (
	providerCircuitMu sync.Mutex
	providerCircuits  = map[string]providerCircuitState{}
)

// providerResponseHeaderDenylist lists headers from LLM provider responses that
// must NOT be forwarded to API clients. Forwarding these could expose internal
// auth challenges, override gateway CORS policy, or set malicious cookies.
var providerResponseHeaderDenylist = map[string]bool{
	"Set-Cookie":                        true,
	"Www-Authenticate":                  true,
	"Proxy-Authenticate":                true,
	"Access-Control-Allow-Origin":       true,
	"Access-Control-Allow-Headers":      true,
	"Access-Control-Allow-Methods":      true,
	"Access-Control-Allow-Credentials":  true,
	"Access-Control-Expose-Headers":     true,
	"Access-Control-Max-Age":            true,
}

// privateIPNets holds IP ranges that must never be targets of outbound provider
// requests (SSRF protection). Populated once at startup.
var privateIPNets []*net.IPNet

func init() {
	for _, cidr := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16", // link-local / AWS metadata
		"0.0.0.0/8",
		"100.64.0.0/10", // carrier-grade NAT
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	} {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			privateIPNets = append(privateIPNets, network)
		}
	}
}

type cachePayload struct {
	StatusCode      int             `json:"status_code"`
	ProviderID      string          `json:"provider_id"`
	Body            json.RawMessage `json:"body"`
	PromptText      string          `json:"prompt_text,omitempty"`
	PromptEmbedding []float64       `json:"prompt_embedding,omitempty"`
}

// New creates a Handler with the given database.
func New(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// SetAggregator sets the telemetry aggregator for metrics collection
func (h *Handler) SetAggregator(agg *telemetry.Aggregator) {
	h.aggregator = agg
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// HealthCheck returns service status.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}

// Register creates a new user.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not hash password"})
		return
	}

	user := models.User{
		ID:    uuid.New().String(),
		Email: req.Email,
		Role:  "user",
		PlanID: "basic",
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO users (id, email, password, role) VALUES (?, ?, ?, ?)`,
		user.ID, user.Email, string(hash), user.Role,
	)
	if err != nil {
		// Return a generic message to prevent email enumeration.
		writeJSON(w, http.StatusConflict, map[string]string{"error": "registration failed"})
		return
	}

	token, err := generateJWT(user.ID, user.Role)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not generate token"})
		return
	}

	writeJSON(w, http.StatusCreated, models.AuthResponse{Token: token, User: user})
}

// Login authenticates a user and returns a JWT.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	var user models.User
	var hash string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, email, password, role FROM users WHERE email = ?`, req.Email,
	).Scan(&user.ID, &user.Email, &hash, &user.Role)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, err := generateJWT(user.ID, user.Role)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not generate token"})
		return
	}

	writeJSON(w, http.StatusOK, models.AuthResponse{Token: token, User: user})
}

// AdminCreateUser allows an admin to create a new user account with view-only role.
func (h *Handler) AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	adminRole, _ := r.Context().Value(middleware.UserRoleKey).(string)
	if !strings.EqualFold(adminRole, "admin") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not hash password"})
		return
	}

	user := models.User{
		ID:    uuid.New().String(),
		Email: req.Email,
		Role:  "user",
		PlanID: "basic",
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO users (id, email, password, role) VALUES (?, ?, ?, ?)`,
		user.ID, user.Email, string(hash), user.Role,
	)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "user creation failed"})
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// ChatCompletions proxies a chat completion request to the appropriate provider.
// If req.Model matches a route slug owned by the user, that route's provider + model are used.
func (h *Handler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	if usage, blocked, err := h.enforceMonthlyTokenQuota(r.Context(), userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	} else if blocked {
		response := map[string]any{
			"error":               "monthly token quota exceeded",
			"plan_id":             usage.PlanID,
			"monthly_used_tokens": usage.UsedTokens,
		}
		if usage.MonthlyTokenLimit.Valid {
			response["monthly_token_limit"] = usage.MonthlyTokenLimit.Int64
		}
		writeJSON(w, http.StatusTooManyRequests, response)
		return
	}
	requestID := chiMiddleware.GetReqID(r.Context())
	var apiKeyID *string
	if rawKeyID, ok := r.Context().Value(middleware.APIKeyIDKey).(string); ok && rawKeyID != "" {
		apiKeyID = &rawKeyID
	}
	apiKeyAllowedProviders, _ := r.Context().Value(middleware.APIKeyAllowedProvidersKey).(string)
	apiKeyAllowedModels, _ := r.Context().Value(middleware.APIKeyAllowedModelsKey).(string)

	var req models.ChatCompletionRequest
	bodyBytes, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	var provider models.Provider
	actualModel := req.Model
	routeSlug := req.Model
	var systemPrompt string
	routeMatched := false
	streamPassthrough := false
	routeFailoverProviders := ""
	routeAllowedModels := ""
	routePromptVersionID := ""
	routeEnforceJSONSchema := false
	routeOutputJSONSchema := ""

	// Try to resolve req.Model as a route slug first.
	var route models.LLMRoute
	routeErr := h.db.QueryRowContext(r.Context(),
		`SELECT r.id, r.model, r.system_prompt, r.temperature, r.max_tokens, r.stream_passthrough,
		        COALESCE(r.prompt_version_id,''), r.enforce_json_schema, COALESCE(r.output_json_schema,''),
		        COALESCE(r.failover_provider_ids,''), COALESCE(r.allowed_models,''),
		        p.id, p.base_url, p.adapter, p.api_version, p.api_key
		 FROM llm_routes r
		 JOIN providers p ON p.id = r.provider_id
		 WHERE r.user_id = ? AND r.slug = ? AND r.enabled = 1 AND p.enabled = 1`,
		userID, req.Model,
	).Scan(&route.ID, &route.Model, &route.SystemPrompt, &route.Temperature, &route.MaxTokens, &route.StreamPassthrough,
		&routePromptVersionID, &routeEnforceJSONSchema, &routeOutputJSONSchema,
		&routeFailoverProviders, &routeAllowedModels,
		&provider.ID, &provider.BaseURL, &provider.Adapter, &provider.APIVersion, &provider.APIKey)

	if routeErr == nil {
		if decryptErr := decryptProviderInPlace(&provider); decryptErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not decrypt provider key"})
			return
		}
		// Route matched – override model and inject system prompt.
		routeMatched = true
		actualModel = route.Model
		systemPrompt = route.SystemPrompt
		streamPassthrough = route.StreamPassthrough
		route.PromptVersionID = routePromptVersionID
		route.EnforceJSONSchema = routeEnforceJSONSchema
		route.OutputJSONSchema = routeOutputJSONSchema

		if strings.TrimSpace(route.PromptVersionID) != "" {
			if content, ok := h.getPromptVersionContent(r.Context(), userID, route.PromptVersionID); ok {
				systemPrompt = content
			}
		}
		if systemPrompt != "" {
			req.Messages = append([]models.Message{{Role: "system", Content: systemPrompt}}, req.Messages...)
		}
	} else if routeErr == sql.ErrNoRows {
		// No route match – fall back to first enabled provider.
		// Provider candidates are resolved below.
	} else {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	var raw map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &raw)
	raw["model"] = actualModel
	raw["messages"] = req.Messages

	if routeMatched && req.Stream && !streamPassthrough {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "streaming disabled for this route"})
		return
	}

	candidates, err := h.listCandidateProviders(r.Context(), routeMatched, provider.ID, routeFailoverProviders, apiKeyAllowedProviders)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	candidates = h.orderProvidersForSmartRouting(r.Context(), userID, actualModel, bodyBytes, routeMatched, candidates)
	if len(candidates) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no providers configured"})
		return
	}

	enableStreamPassThrough := req.Stream
	if enableStreamPassThrough {
		candidates = filterStreamingProviders(candidates)
		if len(candidates) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "streaming unavailable for the selected providers"})
			return
		}
	}

	raw["stream"] = enableStreamPassThrough
	if enableStreamPassThrough {
		streamOptions, _ := raw["stream_options"].(map[string]interface{})
		if streamOptions == nil {
			streamOptions = map[string]interface{}{}
		}
		streamOptions["include_usage"] = true
		raw["stream_options"] = streamOptions
	}
	if routeMatched {
		if route.MaxTokens > 0 {
			raw["max_tokens"] = route.MaxTokens
		}
		if route.Temperature != 1.0 {
			raw["temperature"] = route.Temperature
		}
		if route.EnforceJSONSchema {
			raw["response_format"] = map[string]any{"type": "json_object"}
		}
	}
	patched, _ := json.Marshal(raw)
	bodyBytes = patched

	if !isModelAllowed(actualModel, routeAllowedModels) || !isModelAllowed(actualModel, apiKeyAllowedModels) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "model not allowed by route or api key policy"})
		return
	}

	semanticPrompt := ""
	semanticEmbedding := []float64(nil)

	if !enableStreamPassThrough {
		cached, cacheHit, cacheErr := h.getCachedChatCompletion(r.Context(), userID, actualModel, bodyBytes)
		if cacheErr == nil && cacheHit && cached != nil {
			promptTokens, completionTokens, totalTokens := parseUsageTokens(cached.Body)
			if totalTokens == 0 {
				totalTokens = promptTokens + completionTokens
			}
			var providerID *string
			if strings.TrimSpace(cached.ProviderID) != "" {
				providerID = &cached.ProviderID
			}
			costUSD := 0.0
			if providerID != nil {
				costUSD = h.calculateRequestCost(r.Context(), userID, *providerID, actualModel, promptTokens, completionTokens)
			}
			_ = h.logRequest(userID, apiKeyID, providerID, actualModel, promptTokens, completionTokens, 0, 0, cached.StatusCode, costUSD)
			h.logAudit(r.Context(), userID, apiKeyID, providerID, requestID, "/chat/completions", "cache_hit", routeSlug, actualModel, cached.StatusCode, 0, true, "", cached.Body)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(cached.StatusCode)
			_, _ = w.Write(cached.Body)
			return
		}

		semanticPrompt = extractSemanticPromptText(bodyBytes)
		semanticCached, semanticHit, queryEmbedding, semanticErr := h.getSemanticCachedChatCompletion(r.Context(), userID, apiKeyID, requestID, routeSlug, actualModel, semanticPrompt, candidates)
		if semanticErr == nil {
			if len(queryEmbedding) > 0 {
				semanticEmbedding = queryEmbedding
			}
			if semanticHit && semanticCached != nil {
				promptTokens, completionTokens, totalTokens := parseUsageTokens(semanticCached.Body)
				if totalTokens == 0 {
					totalTokens = promptTokens + completionTokens
				}
				var providerID *string
				if strings.TrimSpace(semanticCached.ProviderID) != "" {
					providerID = &semanticCached.ProviderID
				}
				costUSD := 0.0
				if providerID != nil {
					costUSD = h.calculateRequestCost(r.Context(), userID, *providerID, actualModel, promptTokens, completionTokens)
				}
				_ = h.logRequest(userID, apiKeyID, providerID, actualModel, promptTokens, completionTokens, 0, 0, semanticCached.StatusCode, costUSD)
				h.logAudit(r.Context(), userID, apiKeyID, providerID, requestID, "/chat/completions", "semantic_cache_hit", routeSlug, actualModel, semanticCached.StatusCode, 0, true, "", semanticCached.Body)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "SEMANTIC-HIT")
				w.WriteHeader(semanticCached.StatusCode)
				_, _ = w.Write(semanticCached.Body)
				return
			}
		}
	}

	providerPtr, resp, latency, err := h.executeProviderRequest(r.Context(), userID, apiKeyID, requestID, routeSlug, actualModel, candidates, "/chat/completions", bodyBytes, 120*time.Second, enableStreamPassThrough)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider unreachable"})
		if providerPtr != nil {
			_ = h.logRequest(userID, apiKeyID, &providerPtr.ID, actualModel, 0, 0, latency, latency, http.StatusBadGateway, 0)
		} else {
			_ = h.logRequest(userID, apiKeyID, nil, actualModel, 0, 0, latency, latency, http.StatusBadGateway, 0)
		}
		return
	}
	defer resp.Body.Close()

	if enableStreamPassThrough {
		promptTokens, completionTokens, totalTokens, streamLatency, ttftMs := streamProviderResponse(w, resp, *providerPtr)
		if totalTokens == 0 {
			totalTokens = promptTokens + completionTokens
		}
		costUSD := h.calculateRequestCost(r.Context(), userID, providerPtr.ID, actualModel, promptTokens, completionTokens)
		_ = h.logRequest(userID, apiKeyID, &providerPtr.ID, actualModel, promptTokens, completionTokens, ttftMs, streamLatency, resp.StatusCode, costUSD)
		return
	}

	respBytes, _ := io.ReadAll(resp.Body)
	promptTokens, completionTokens, _ := parseUsageTokens(respBytes)

	if routeMatched && route.EnforceJSONSchema && strings.TrimSpace(route.OutputJSONSchema) != "" {
		if validationErr := validateStructuredOutput(respBytes, route.OutputJSONSchema); validationErr != nil {
			h.logAudit(r.Context(), userID, apiKeyID, &providerPtr.ID, requestID, "/chat/completions", "schema_validation_failed", routeSlug, actualModel, http.StatusBadGateway, latency, false, validationErr.Error(), respBytes)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "response did not match required JSON schema"})
			_ = h.logRequest(userID, apiKeyID, &providerPtr.ID, actualModel, promptTokens, completionTokens, latency, latency, http.StatusBadGateway, 0)
			return
		}
	}

	// Calculate cost using configured pricing rule for this provider + model.
	costUSD := h.calculateRequestCost(r.Context(), userID, providerPtr.ID, actualModel, promptTokens, completionTokens)
	_ = h.logRequest(userID, apiKeyID, &providerPtr.ID, actualModel,
		promptTokens, completionTokens, latency, latency, resp.StatusCode, costUSD)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_ = h.setCachedChatCompletion(r.Context(), userID, actualModel, bodyBytes, providerPtr.ID, resp.StatusCode, respBytes, semanticPrompt, semanticEmbedding)
	}

	copyProviderHeaders(w, resp)
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBytes)
}

// Embeddings proxies an embeddings request.
func (h *Handler) Embeddings(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	if usage, blocked, err := h.enforceMonthlyTokenQuota(r.Context(), userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	} else if blocked {
		response := map[string]any{
			"error":               "monthly token quota exceeded",
			"plan_id":             usage.PlanID,
			"monthly_used_tokens": usage.UsedTokens,
		}
		if usage.MonthlyTokenLimit.Valid {
			response["monthly_token_limit"] = usage.MonthlyTokenLimit.Int64
		}
		writeJSON(w, http.StatusTooManyRequests, response)
		return
	}
	requestID := chiMiddleware.GetReqID(r.Context())
	var apiKeyID *string
	if rawKeyID, ok := r.Context().Value(middleware.APIKeyIDKey).(string); ok && rawKeyID != "" {
		apiKeyID = &rawKeyID
	}
	apiKeyAllowedProviders, _ := r.Context().Value(middleware.APIKeyAllowedProvidersKey).(string)

	bodyBytes, _ := io.ReadAll(r.Body)
	candidates, err := h.listCandidateProviders(r.Context(), false, "", "", apiKeyAllowedProviders)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no providers configured"})
		return
	}
	if len(candidates) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no providers configured"})
		return
	}

	providerPtr, resp, latency, err := h.executeProviderRequest(r.Context(), userID, apiKeyID, requestID, "", "embedding", candidates, "/embeddings", bodyBytes, 60*time.Second, false)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider unreachable"})
		if providerPtr != nil {
			_ = h.logRequest(userID, apiKeyID, &providerPtr.ID, "embedding", 0, 0, latency, latency, http.StatusBadGateway, 0)
		} else {
			_ = h.logRequest(userID, apiKeyID, nil, "embedding", 0, 0, latency, latency, http.StatusBadGateway, 0)
		}
		return
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	promptTokens, completionTokens, _ := parseUsageTokens(respBytes)
	_ = h.logRequest(userID, apiKeyID, &providerPtr.ID, "embedding", promptTokens, completionTokens, latency, latency, resp.StatusCode, 0)

	copyProviderHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBytes)
}

// ListUnifiedModels returns an OpenAI-compatible model catalog for route aliases
// and directly configured provider models.
func (h *Handler) ListUnifiedModels(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	apiKeyAllowedModels, _ := r.Context().Value(middleware.APIKeyAllowedModelsKey).(string)
	cards := make([]map[string]any, 0)
	seen := map[string]bool{}

	routeRows, err := h.db.QueryContext(r.Context(),
		`SELECT rt.slug, rt.model, p.id, p.name, rt.created_at
		 FROM llm_routes rt
		 JOIN providers p ON p.id = rt.provider_id
		 WHERE rt.user_id = ? AND rt.enabled = 1 AND p.enabled = 1
		 ORDER BY rt.created_at ASC`, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer routeRows.Close()

	for routeRows.Next() {
		var slug, actualModel, providerID, providerName string
		var createdAt time.Time
		if scanErr := routeRows.Scan(&slug, &actualModel, &providerID, &providerName, &createdAt); scanErr != nil {
			continue
		}
		if slug == "" || seen[slug] || !isModelAllowed(actualModel, apiKeyAllowedModels) {
			continue
		}
		seen[slug] = true
		cards = append(cards, map[string]any{
			"id":         slug,
			"object":     "model",
			"created":    createdAt.Unix(),
			"owned_by":   providerName,
			"root":       actualModel,
			"permission": []any{},
			"metadata": map[string]any{
				"kind":          "route",
				"provider_id":   providerID,
				"provider_name": providerName,
				"actual_model":  actualModel,
			},
		})
	}

	directRows, err := h.db.QueryContext(r.Context(),
		`SELECT DISTINCT mc.model, p.id, p.name, mc.updated_at
		 FROM model_costs mc
		 JOIN providers p ON p.id = mc.provider_id
		 WHERE mc.user_id = ? AND p.enabled = 1
		 ORDER BY mc.updated_at DESC`, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer directRows.Close()

	for directRows.Next() {
		var modelID, providerID, providerName string
		var updatedAt time.Time
		if scanErr := directRows.Scan(&modelID, &providerID, &providerName, &updatedAt); scanErr != nil {
			continue
		}
		if modelID == "" || seen[modelID] || !isModelAllowed(modelID, apiKeyAllowedModels) {
			continue
		}
		seen[modelID] = true
		cards = append(cards, map[string]any{
			"id":         modelID,
			"object":     "model",
			"created":    updatedAt.Unix(),
			"owned_by":   providerName,
			"root":       modelID,
			"permission": []any{},
			"metadata": map[string]any{
				"kind":          "provider_model",
				"provider_id":   providerID,
				"provider_name": providerName,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   cards,
	})
}

// ListAPIKeys returns all API keys for the authenticated user.
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT k.id, k.user_id, k.name, k.key_prefix, k.group_id, COALESCE(g.name,''), k.expires_at, k.created_at,
		        COALESCE(k.allowed_provider_ids,''), COALESCE(k.allowed_models,''),
		        COUNT(r.id) AS request_count,
		        COALESCE(SUM(r.total_tokens),0) AS total_tokens,
		        COALESCE(SUM(r.cost_usd),0) AS total_cost_usd,
		        MAX(r.created_at) AS last_used_at
		 FROM api_keys k
		 LEFT JOIN cost_groups g ON g.id = k.group_id AND g.user_id = k.user_id
		 LEFT JOIN requests r ON r.api_key_id = k.id
		 WHERE k.user_id = ?
		 GROUP BY k.id, k.user_id, k.name, k.key_prefix, k.group_id, g.name, k.expires_at, k.created_at, k.allowed_provider_ids, k.allowed_models
		 ORDER BY k.created_at DESC`, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	keys := []models.APIKey{}
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.GroupID, &k.GroupName, &k.ExpiresAt, &k.CreatedAt,
			&k.AllowedProviderIDs, &k.AllowedModels, &k.RequestCount, &k.TotalTokens, &k.TotalCostUSD, &k.LastUsedAt); err != nil {
			continue
		}
		keys = append(keys, k)
	}
	writeJSON(w, http.StatusOK, keys)
}

// CreateAPIKey generates a new gateway API key for the user.
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	var body struct {
		Name               string `json:"name"`
		AllowedProviderIDs string `json:"allowed_provider_ids"`
		AllowedModels      string `json:"allowed_models"`
		GroupID            string `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}

	var groupID any
	if strings.TrimSpace(body.GroupID) != "" {
		if !h.groupBelongsToUser(r.Context(), userID, body.GroupID) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid group_id"})
			return
		}
		groupID = body.GroupID
	}

	rawKey := "gw-" + uuid.New().String()
	prefix := rawKey[:10]
	hash, _ := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)

	k := models.APIKey{
		ID:                 uuid.New().String(),
		UserID:             userID,
		Name:               body.Name,
		KeyPrefix:          prefix,
		AllowedProviderIDs: normalizeCSV(body.AllowedProviderIDs),
		AllowedModels:      normalizeCSV(body.AllowedModels),
		PlainKey:           rawKey,
	}
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO api_keys (id, user_id, group_id, name, key_hash, key_prefix, allowed_provider_ids, allowed_models)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		k.ID, k.UserID, groupID, k.Name, string(hash), k.KeyPrefix, k.AllowedProviderIDs, k.AllowedModels,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create key"})
		return
	}
	writeJSON(w, http.StatusCreated, k)
}

// DeleteAPIKey removes an API key owned by the user.
func (h *Handler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")
	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM api_keys WHERE id = ? AND user_id = ?`, id, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListProviders returns all configured LLM providers.
func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name, base_url, adapter, api_version, api_keys_json, enabled, created_at, updated_at FROM providers`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	providers := []models.Provider{}
	for rows.Next() {
		var p models.Provider
		if err := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.Adapter, &p.APIVersion, &p.APIKeysJSON, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		p.KeyCount = providerKeyCount(p)
		providers = append(providers, p)
	}
	writeJSON(w, http.StatusOK, providers)
}

// CreateProvider adds a new LLM provider.
func (h *Handler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	const singleLLMLicenseMsg = "Only one LLM can be configured in this plan. To configure more than one LLM, get the license or contact pv@realtimedetect.com"

	var body struct {
		Name       string `json:"name"`
		BaseURL    string `json:"base_url"`
		Adapter    string `json:"adapter"`
		APIVersion string `json:"api_version"`
		APIKey     string `json:"api_key"`
		APIKeys    []string `json:"api_keys"`
		Enabled    *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.BaseURL = strings.TrimSpace(body.BaseURL)
	body.Adapter = normalizeProviderAdapter(body.Adapter)
	if body.Name == "" || body.BaseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and base_url are required"})
		return
	}
	if body.Adapter == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "adapter must be one of: openai, anthropic"})
		return
	}
	if !isValidProviderURL(body.BaseURL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or disallowed base_url: must be http/https and not an internal address"})
		return
	}

	var providerCount int
	if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM providers`).Scan(&providerCount); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if providerCount >= 1 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": singleLLMLicenseMsg})
		return
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	p := models.Provider{
		ID:         uuid.New().String(),
		Name:       body.Name,
		BaseURL:    body.BaseURL,
		Adapter:    body.Adapter,
		APIVersion: strings.TrimSpace(body.APIVersion),
		Enabled:    enabled,
	}
	encryptedPrimary, encErr := encryptSecret(body.APIKey)
	if encErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "key encryption failed"})
		return
	}
	encryptedExtras, encErr := encryptSecrets(normalizeProviderAPIKeys(body.APIKey, body.APIKeys))
	if encErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "key encryption failed"})
		return
	}
	p.APIKeysJSON = encodeAPIKeyPoolJSON(encryptedExtras)
	p.KeyCount = providerKeyCount(p)
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO providers (id, name, base_url, adapter, api_version, api_key, api_keys_json, enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.BaseURL, p.Adapter, p.APIVersion, encryptedPrimary, p.APIKeysJSON, p.Enabled,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create provider"})
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// UpdateProvider modifies an existing provider.
func (h *Handler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Name       string `json:"name"`
		BaseURL    string `json:"base_url"`
		Adapter    string `json:"adapter"`
		APIVersion string `json:"api_version"`
		APIKey     string `json:"api_key"` // empty string = preserve existing key
		APIKeys    []string `json:"api_keys"`
		Enabled    bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.BaseURL = strings.TrimSpace(body.BaseURL)
	body.Adapter = normalizeProviderAdapter(body.Adapter)
	if body.Adapter == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "adapter must be one of: openai, anthropic"})
		return
	}
	if body.BaseURL != "" && !isValidProviderURL(body.BaseURL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or disallowed base_url: must be http/https and not an internal address"})
		return
	}
	var err error
	if body.APIKey != "" {
		encryptedPrimary, encErr := encryptSecret(body.APIKey)
		if encErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "key encryption failed"})
			return
		}
		encryptedExtras, encErr := encryptSecrets(normalizeProviderAPIKeys(body.APIKey, body.APIKeys))
		if encErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "key encryption failed"})
			return
		}
		// New API key provided — update all fields including the key.
		_, err = h.db.ExecContext(r.Context(),
			`UPDATE providers SET name=?, base_url=?, adapter=?, api_version=?, api_key=?, api_keys_json=?, enabled=? WHERE id=?`,
			body.Name, body.BaseURL, body.Adapter, strings.TrimSpace(body.APIVersion), encryptedPrimary, encodeAPIKeyPoolJSON(encryptedExtras), body.Enabled, id,
		)
	} else {
		// No new primary API key — preserve it but update key-pool JSON.
		var existingPrimary string
		if scanErr := h.db.QueryRowContext(r.Context(), `SELECT api_key FROM providers WHERE id = ?`, id).Scan(&existingPrimary); scanErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
			return
		}
		decryptedPrimary, decErr := decryptSecret(existingPrimary)
		if decErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not decrypt existing key"})
			return
		}
		encryptedExtras, encErr := encryptSecrets(normalizeProviderAPIKeys(decryptedPrimary, body.APIKeys))
		if encErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "key encryption failed"})
			return
		}
		_, err = h.db.ExecContext(r.Context(),
			`UPDATE providers SET name=?, base_url=?, adapter=?, api_version=?, api_keys_json=?, enabled=? WHERE id=?`,
			body.Name, body.BaseURL, body.Adapter, strings.TrimSpace(body.APIVersion), encodeAPIKeyPoolJSON(encryptedExtras), body.Enabled, id,
		)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	p := models.Provider{ID: id, Name: body.Name, BaseURL: body.BaseURL, Adapter: body.Adapter, APIVersion: strings.TrimSpace(body.APIVersion), Enabled: body.Enabled}
	writeJSON(w, http.StatusOK, p)
}

// DeleteProvider removes a provider.
func (h *Handler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM providers WHERE id = ?`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetUsage returns aggregated usage stats for the authenticated user.
func (h *Handler) GetUsage(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	var summary models.UsageSummary
	err := h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_usd),0), COALESCE(AVG(latency_ms),0)
		 FROM requests WHERE user_id = ?`, userID,
	).Scan(&summary.TotalRequests, &summary.TotalTokens, &summary.TotalCost, &summary.AvgLatencyMs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// GetAPIKeyAnalytics returns chart-ready API key analytics datasets.
func (h *Handler) GetAPIKeyAnalytics(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	timeseriesRows, err := h.db.QueryContext(r.Context(),
		`SELECT DATE_FORMAT(req.created_at, '%Y-%m-%d') AS day,
		        COALESCE(k.id,'deleted-key') AS key_id,
		        COALESCE(k.name,'Deleted Key') AS key_name,
		        COALESCE(k.key_prefix,'deleted') AS key_prefix,
		        COUNT(*) AS request_count,
		        COALESCE(SUM(req.total_tokens),0) AS total_tokens,
		        COALESCE(SUM(req.cost_usd),0) AS total_cost_usd
		 FROM requests req
		 LEFT JOIN api_keys k ON k.id = req.api_key_id
		 WHERE req.user_id = ? AND req.api_key_id IS NOT NULL
		 GROUP BY day, key_id, key_name, key_prefix
		 ORDER BY day ASC`, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer timeseriesRows.Close()

	type tsRow struct {
		Day         string  `json:"day"`
		KeyID       string  `json:"key_id"`
		KeyName     string  `json:"key_name"`
		KeyPrefix   string  `json:"key_prefix"`
		Requests    int     `json:"requests"`
		TotalTokens int     `json:"total_tokens"`
		TotalCost   float64 `json:"total_cost_usd"`
	}
	timeseries := []tsRow{}
	for timeseriesRows.Next() {
		var row tsRow
		if scanErr := timeseriesRows.Scan(&row.Day, &row.KeyID, &row.KeyName, &row.KeyPrefix, &row.Requests, &row.TotalTokens, &row.TotalCost); scanErr != nil {
			continue
		}
		timeseries = append(timeseries, row)
	}

	costRows, err := h.db.QueryContext(r.Context(),
		`SELECT COALESCE(k.id,'deleted-key') AS key_id,
		        COALESCE(k.name,'Deleted Key') AS key_name,
		        COALESCE(k.key_prefix,'deleted') AS key_prefix,
		        COALESCE(SUM(req.cost_usd),0) AS total_cost_usd
		 FROM requests req
		 LEFT JOIN api_keys k ON k.id = req.api_key_id
		 WHERE req.user_id = ? AND req.api_key_id IS NOT NULL
		 GROUP BY key_id, key_name, key_prefix
		 ORDER BY total_cost_usd DESC`, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer costRows.Close()

	type costRow struct {
		KeyID     string  `json:"key_id"`
		KeyName   string  `json:"key_name"`
		KeyPrefix string  `json:"key_prefix"`
		TotalCost float64 `json:"total_cost_usd"`
	}
	costByKey := []costRow{}
	for costRows.Next() {
		var row costRow
		if scanErr := costRows.Scan(&row.KeyID, &row.KeyName, &row.KeyPrefix, &row.TotalCost); scanErr != nil {
			continue
		}
		costByKey = append(costByKey, row)
	}

	topRows, err := h.db.QueryContext(r.Context(),
		`SELECT COALESCE(k.id,'deleted-key') AS key_id,
		        COALESCE(k.name,'Deleted Key') AS key_name,
		        COALESCE(k.key_prefix,'deleted') AS key_prefix,
		        COALESCE(SUM(req.total_tokens),0) AS total_tokens,
		        COUNT(*) AS request_count
		 FROM requests req
		 LEFT JOIN api_keys k ON k.id = req.api_key_id
		 WHERE req.user_id = ? AND req.api_key_id IS NOT NULL
		 GROUP BY key_id, key_name, key_prefix
		 ORDER BY total_tokens DESC
		 LIMIT 10`, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer topRows.Close()

	type topRow struct {
		KeyID       string `json:"key_id"`
		KeyName     string `json:"key_name"`
		KeyPrefix   string `json:"key_prefix"`
		TotalTokens int    `json:"total_tokens"`
		Requests    int    `json:"requests"`
	}
	topByTokens := []topRow{}
	for topRows.Next() {
		var row topRow
		if scanErr := topRows.Scan(&row.KeyID, &row.KeyName, &row.KeyPrefix, &row.TotalTokens, &row.Requests); scanErr != nil {
			continue
		}
		topByTokens = append(topByTokens, row)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"timeseries":         timeseries,
		"cost_by_key":        costByKey,
		"top_keys_by_tokens": topByTokens,
	})
}

// ListRequests returns recent requests for the authenticated user.
func (h *Handler) ListRequests(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, user_id, api_key_id, provider_id, model, prompt_tokens, completion_tokens, total_tokens, ttft_ms, latency_ms, status, cost_usd, created_at
		 FROM requests WHERE user_id = ? ORDER BY created_at DESC LIMIT 100`, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	reqs := []models.Request{}
	for rows.Next() {
		var req models.Request
		if err := rows.Scan(&req.ID, &req.UserID, &req.APIKeyID, &req.ProviderID, &req.Model,
			&req.PromptTokens, &req.CompletionTokens, &req.TotalTokens,
			&req.TTFTMs, &req.LatencyMs, &req.Status, &req.CostUSD, &req.CreatedAt); err != nil {
			continue
		}
		reqs = append(reqs, req)
	}
	writeJSON(w, http.StatusOK, reqs)
}

// ListAudits returns recent LLM audit events for the authenticated user.
func (h *Handler) ListAudits(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
	endpoint := strings.TrimSpace(r.URL.Query().Get("endpoint"))
	direction := strings.TrimSpace(r.URL.Query().Get("direction"))
	statusRaw := strings.TrimSpace(r.URL.Query().Get("status"))
	fromDate := strings.TrimSpace(r.URL.Query().Get("from"))
	toDate := strings.TrimSpace(r.URL.Query().Get("to"))

	query := `SELECT id, user_id, api_key_id, provider_id, request_id, endpoint, direction,
	        route_slug, model, http_status, latency_ms, success, COALESCE(error,''), COALESCE(payload,''), created_at
	 FROM audit_logs
	 WHERE user_id = ?`
	args := []any{userID}

	if requestID != "" {
		query += ` AND request_id LIKE ?`
		args = append(args, "%"+requestID+"%")
	}
	if endpoint != "" {
		query += ` AND endpoint = ?`
		args = append(args, endpoint)
	}
	if direction != "" {
		query += ` AND direction = ?`
		args = append(args, direction)
	}
	if statusRaw != "" {
		if status, convErr := strconv.Atoi(statusRaw); convErr == nil {
			query += ` AND http_status = ?`
			args = append(args, status)
		}
	}
	if fromDate != "" {
		query += ` AND DATE(created_at) >= ?`
		args = append(args, fromDate)
	}
	if toDate != "" {
		query += ` AND DATE(created_at) <= ?`
		args = append(args, toDate)
	}

	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	audits := []models.AuditLog{}
	for rows.Next() {
		var item models.AuditLog
		if scanErr := rows.Scan(
			&item.ID, &item.UserID, &item.APIKeyID, &item.ProviderID,
			&item.RequestID, &item.Endpoint, &item.Direction, &item.RouteSlug,
			&item.Model, &item.HTTPStatus, &item.LatencyMs, &item.Success,
			&item.Error, &item.Payload, &item.CreatedAt,
		); scanErr != nil {
			continue
		}
		audits = append(audits, item)
	}

	writeJSON(w, http.StatusOK, audits)
}

// logRequest inserts a request record into the database.
func (h *Handler) logRequest(userID string, apiKeyID *string, providerID *string, model string,
	promptTokens, completionTokens, ttftMs, latencyMs, status int, costUSD float64) error {
	var groupID *string
	if apiKeyID != nil && *apiKeyID != "" {
		groupID = h.resolveGroupIDForAPIKey(context.Background(), userID, *apiKeyID)
	}
	_, err := h.db.Exec(
		`INSERT INTO requests (id, user_id, api_key_id, group_id, provider_id, model, prompt_tokens, completion_tokens, total_tokens, ttft_ms, latency_ms, status, cost_usd)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), userID, apiKeyID, groupID, providerID, model,
		promptTokens, completionTokens, promptTokens+completionTokens, ttftMs, latencyMs, status, costUSD,
	)
	return err
}

func (h *Handler) resolveGroupIDForAPIKey(ctx context.Context, userID, apiKeyID string) *string {
	if apiKeyID == "" {
		return nil
	}
	var groupID sql.NullString
	if err := h.db.QueryRowContext(ctx,
		`SELECT group_id FROM api_keys WHERE id = ? AND user_id = ?`,
		apiKeyID, userID,
	).Scan(&groupID); err != nil {
		return nil
	}
	if !groupID.Valid || strings.TrimSpace(groupID.String) == "" {
		return nil
	}
	v := groupID.String
	return &v
}

// ListCostGroups returns spend groups owned by the authenticated user.
func (h *Handler) ListCostGroups(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, user_id, name, description, created_at, updated_at
		 FROM cost_groups WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	groups := []models.CostGroup{}
	for rows.Next() {
		var g models.CostGroup
		if scanErr := rows.Scan(&g.ID, &g.UserID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt); scanErr != nil {
			continue
		}
		groups = append(groups, g)
	}
	writeJSON(w, http.StatusOK, groups)
}

// CreateCostGroup creates a spend group for the authenticated user.
func (h *Handler) CreateCostGroup(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	group := models.CostGroup{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        body.Name,
		Description: strings.TrimSpace(body.Description),
	}
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO cost_groups (id, user_id, name, description) VALUES (?, ?, ?, ?)`,
		group.ID, group.UserID, group.Name, group.Description,
	)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "group name already exists"})
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

// UpdateCostGroup modifies a spend group owned by the user.
func (h *Handler) UpdateCostGroup(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	res, err := h.db.ExecContext(r.Context(),
		`UPDATE cost_groups SET name=?, description=? WHERE id=? AND user_id=?`,
		body.Name, strings.TrimSpace(body.Description), id, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "group name already exists"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "group not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "name": body.Name, "description": strings.TrimSpace(body.Description)})
}

// DeleteCostGroup removes a spend group and unassigns API keys from it.
func (h *Handler) DeleteCostGroup(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")
	_, _ = h.db.ExecContext(r.Context(),
		`UPDATE api_keys SET group_id = NULL WHERE user_id = ? AND group_id = ?`,
		userID, id,
	)
	res, err := h.db.ExecContext(r.Context(), `DELETE FROM cost_groups WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "group not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AssignAPIKeyGroup sets or clears an API key's spend group.
func (h *Handler) AssignAPIKeyGroup(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	apiKeyID := chi.URLParam(r, "id")
	var body struct {
		GroupID string `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.GroupID = strings.TrimSpace(body.GroupID)

	var groupValue any
	if body.GroupID != "" {
		if !h.groupBelongsToUser(r.Context(), userID, body.GroupID) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid group_id"})
			return
		}
		groupValue = body.GroupID
	}

	res, err := h.db.ExecContext(r.Context(),
		`UPDATE api_keys SET group_id = ? WHERE id = ? AND user_id = ?`,
		groupValue, apiKeyID, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "api key not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": apiKeyID, "group_id": body.GroupID})
}

// GetCostBreakdown returns user-level totals and group-level spend buckets.
func (h *Handler) GetCostBreakdown(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	period := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period")))
	if period == "" {
		period = "30d"
	}

	var timeFilter string
	var filterArg any
	switch period {
	case "today":
		timeFilter = " AND DATE(created_at) = CURDATE()"
	case "7d":
		timeFilter = " AND created_at >= ?"
		filterArg = time.Now().AddDate(0, 0, -7)
	case "30d":
		timeFilter = " AND created_at >= ?"
		filterArg = time.Now().AddDate(0, 0, -30)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid period, use today|7d|30d"})
		return
	}

	var userSummary models.UsageSummary
	userSummaryQuery := `SELECT COUNT(*), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_usd),0), COALESCE(AVG(latency_ms),0)
		 FROM requests WHERE user_id = ?` + timeFilter
	userSummaryArgs := []any{userID}
	if filterArg != nil {
		userSummaryArgs = append(userSummaryArgs, filterArg)
	}
	if err := h.db.QueryRowContext(r.Context(), userSummaryQuery, userSummaryArgs...).
		Scan(&userSummary.TotalRequests, &userSummary.TotalTokens, &userSummary.TotalCost, &userSummary.AvgLatencyMs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	groupQuery := `SELECT COALESCE(cg.id, 'ungrouped') AS group_id,
		        COALESCE(cg.name, 'Ungrouped') AS group_name,
		        COUNT(*) AS requests,
		        COALESCE(SUM(r.total_tokens),0) AS total_tokens,
		        COALESCE(SUM(r.cost_usd),0) AS total_cost_usd
		 FROM requests r
		 LEFT JOIN cost_groups cg ON cg.id = r.group_id AND cg.user_id = r.user_id
		 WHERE r.user_id = ?` + strings.ReplaceAll(timeFilter, "created_at", "r.created_at") + `
		 GROUP BY group_id, group_name
		 ORDER BY total_cost_usd DESC`
	groupArgs := []any{userID}
	if filterArg != nil {
		groupArgs = append(groupArgs, filterArg)
	}
	rows, err := h.db.QueryContext(r.Context(), groupQuery, groupArgs...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	groups := []models.CostGroupSpend{}
	for rows.Next() {
		var row models.CostGroupSpend
		if scanErr := rows.Scan(&row.GroupID, &row.GroupName, &row.Requests, &row.TotalTokens, &row.TotalCostUSD); scanErr != nil {
			continue
		}
		groups = append(groups, row)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"period":     period,
		"user_total": userSummary,
		"groups":     groups,
	})
}

func (h *Handler) groupBelongsToUser(ctx context.Context, userID, groupID string) bool {
	if strings.TrimSpace(groupID) == "" {
		return false
	}
	var exists int
	err := h.db.QueryRowContext(ctx,
		`SELECT 1 FROM cost_groups WHERE id = ? AND user_id = ? LIMIT 1`,
		groupID, userID,
	).Scan(&exists)
	return err == nil && exists == 1
}

func (h *Handler) logAudit(
	ctx context.Context,
	userID string,
	apiKeyID *string,
	providerID *string,
	requestID, endpoint, direction, routeSlug, model string,
	httpStatus, latencyMs int,
	success bool,
	errMessage string,
	payload []byte,
) {
	payloadText := ""
	if len(payload) > 0 {
		payloadText = truncateAuditPayload(payload, handlerEnvInt("AUDIT_MAX_BODY_BYTES", 65536))
	}
	_, _ = h.db.ExecContext(ctx,
		`INSERT INTO audit_logs
		  (id, user_id, api_key_id, provider_id, request_id, endpoint, direction, route_slug, model, http_status, latency_ms, success, error, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), userID, apiKeyID, providerID, requestID, endpoint, direction, routeSlug, model,
		httpStatus, latencyMs, success, errMessage, payloadText,
	)

	if strings.EqualFold(strings.TrimSpace(os.Getenv("AUDIT_LOG_TO_STDOUT")), "true") || strings.TrimSpace(os.Getenv("AUDIT_LOG_TO_STDOUT")) == "1" {
		line := map[string]any{
			"ts":          time.Now().UTC().Format(time.RFC3339Nano),
			"type":        "audit_log",
			"user_id":     userID,
			"api_key_id":  apiKeyID,
			"provider_id": providerID,
			"request_id":  requestID,
			"endpoint":    endpoint,
			"direction":   direction,
			"route_slug":  routeSlug,
			"model":       model,
			"http_status": httpStatus,
			"latency_ms":  latencyMs,
			"success":     success,
			"error":       errMessage,
			"payload":     payloadText,
		}
		if encoded, err := json.Marshal(line); err == nil {
			log.Println(string(encoded))
		}
	}
}

func truncateAuditPayload(payload []byte, maxBytes int) string {
	if maxBytes <= 0 || len(payload) <= maxBytes {
		return string(payload)
	}
	trimmed := payload[:maxBytes]
	return string(trimmed) + "\n...[truncated]"
}

// ── Model Costs ──────────────────────────────────────────────────────────────

// ListCosts returns all cost rules for the authenticated user.
func (h *Handler) ListCosts(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT mc.id, mc.user_id, mc.provider_id, COALESCE(p.name,'') AS provider_name,
		        mc.model, mc.input_cost_per_1m, mc.output_cost_per_1m, mc.currency,
		        mc.notes, mc.created_at, mc.updated_at
		 FROM model_costs mc
		 LEFT JOIN providers p ON p.id = mc.provider_id
		 WHERE mc.user_id = ?
		 ORDER BY mc.created_at DESC`, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()
	costs := []models.ModelCost{}
	for rows.Next() {
		var mc models.ModelCost
		if err := rows.Scan(&mc.ID, &mc.UserID, &mc.ProviderID, &mc.ProviderName,
			&mc.Model, &mc.InputCostPer1M, &mc.OutputCostPer1M, &mc.Currency,
			&mc.Notes, &mc.CreatedAt, &mc.UpdatedAt); err != nil {
			continue
		}
		costs = append(costs, mc)
	}
	writeJSON(w, http.StatusOK, costs)
}

// CreateCost adds a new cost rule for the authenticated user.
func (h *Handler) CreateCost(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	var mc models.ModelCost
	if err := json.NewDecoder(r.Body).Decode(&mc); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if mc.ProviderID == "" || mc.Model == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider_id and model are required"})
		return
	}
	if mc.Currency == "" {
		mc.Currency = "USD"
	}
	mc.ID = uuid.New().String()
	mc.UserID = userID
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO model_costs (id, user_id, provider_id, model, input_cost_per_1m, output_cost_per_1m, currency, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		mc.ID, mc.UserID, mc.ProviderID, mc.Model,
		mc.InputCostPer1M, mc.OutputCostPer1M, mc.Currency, mc.Notes,
	)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cost rule already exists for this provider and model"})
		return
	}
	writeJSON(w, http.StatusCreated, mc)
}

// UpdateCost modifies an existing cost rule owned by the user.
func (h *Handler) UpdateCost(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")
	var mc models.ModelCost
	if err := json.NewDecoder(r.Body).Decode(&mc); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if mc.Currency == "" {
		mc.Currency = "USD"
	}
	res, err := h.db.ExecContext(r.Context(),
		`UPDATE model_costs
		 SET provider_id=?, model=?, input_cost_per_1m=?, output_cost_per_1m=?, currency=?, notes=?
		 WHERE id=? AND user_id=?`,
		mc.ProviderID, mc.Model, mc.InputCostPer1M, mc.OutputCostPer1M, mc.Currency, mc.Notes, id, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cost rule not found"})
		return
	}
	mc.ID = id
	mc.UserID = userID
	writeJSON(w, http.StatusOK, mc)
}

// DeleteCost removes a cost rule owned by the user.
func (h *Handler) DeleteCost(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")
	res, err := h.db.ExecContext(r.Context(),
		`DELETE FROM model_costs WHERE id = ? AND user_id = ?`, id, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cost rule not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetCacheConfig returns Redis cache settings for the authenticated user.
func (h *Handler) GetCacheConfig(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	cfg, err := h.getCacheConfigByUser(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if cfg == nil {
		writeJSON(w, http.StatusOK, models.CacheConfig{
			Enabled:                false,
			SemanticEnabled:        false,
			SemanticThreshold:      0.9,
			SemanticMaxCandidates:  30,
			SemanticEmbeddingModel: "text-embedding-3-small",
			RedisAddr:              "localhost:6379",
			RedisUsername:          "",
			RedisDB:                0,
			DefaultTTLSeconds:      300,
			KeyPrefix:              "llm-gw",
			HasPassword:            false,
		})
		return
	}
	cfg.RedisPassword = ""
	writeJSON(w, http.StatusOK, cfg)
}

// UpsertCacheConfig creates or updates Redis cache settings for the authenticated user.
func (h *Handler) UpsertCacheConfig(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	var body struct {
		Enabled                bool    `json:"enabled"`
		SemanticEnabled        bool    `json:"semantic_enabled"`
		SemanticThreshold      float64 `json:"semantic_threshold"`
		SemanticMaxCandidates  int     `json:"semantic_max_candidates"`
		SemanticEmbeddingModel string  `json:"semantic_embedding_model"`
		RedisAddr              string  `json:"redis_addr"`
		RedisUsername          string  `json:"redis_username"`
		RedisPassword          string  `json:"redis_password"`
		ClearPassword          bool    `json:"clear_password"`
		RedisDB                int     `json:"redis_db"`
		DefaultTTLSeconds      int     `json:"default_ttl_seconds"`
		KeyPrefix              string  `json:"key_prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	body.RedisAddr = strings.TrimSpace(body.RedisAddr)
	body.RedisUsername = strings.TrimSpace(body.RedisUsername)
	body.KeyPrefix = strings.TrimSpace(body.KeyPrefix)
	body.SemanticEmbeddingModel = strings.TrimSpace(body.SemanticEmbeddingModel)
	if body.RedisAddr == "" {
		body.RedisAddr = "localhost:6379"
	}
	if body.KeyPrefix == "" {
		body.KeyPrefix = "llm-gw"
	}
	if body.DefaultTTLSeconds <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "default_ttl_seconds must be greater than zero"})
		return
	}
	if body.DefaultTTLSeconds > 86400 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "default_ttl_seconds must be <= 86400"})
		return
	}
	if body.RedisDB < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "redis_db must be >= 0"})
		return
	}
	if body.SemanticThreshold <= 0 {
		body.SemanticThreshold = 0.9
	}
	if body.SemanticThreshold < 0.5 || body.SemanticThreshold > 0.999 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "semantic_threshold must be between 0.5 and 0.999"})
		return
	}
	if body.SemanticMaxCandidates <= 0 {
		body.SemanticMaxCandidates = 30
	}
	if body.SemanticMaxCandidates > 200 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "semantic_max_candidates must be <= 200"})
		return
	}
	if body.SemanticEmbeddingModel == "" {
		body.SemanticEmbeddingModel = "text-embedding-3-small"
	}

	current, err := h.getCacheConfigByUser(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	passwordToStore := ""
	if current != nil && current.HasPassword {
		passwordToStore = current.RedisPassword
	}
	if body.ClearPassword {
		passwordToStore = ""
	} else if body.RedisPassword != "" {
		passwordToStore = body.RedisPassword
	}

	cfg := models.CacheConfig{
		Enabled:                body.Enabled,
		SemanticEnabled:        body.SemanticEnabled,
		SemanticThreshold:      body.SemanticThreshold,
		SemanticMaxCandidates:  body.SemanticMaxCandidates,
		SemanticEmbeddingModel: body.SemanticEmbeddingModel,
		RedisAddr:              body.RedisAddr,
		RedisUsername:          body.RedisUsername,
		RedisPassword:          passwordToStore,
		RedisDB:                body.RedisDB,
		DefaultTTLSeconds:      body.DefaultTTLSeconds,
		KeyPrefix:              body.KeyPrefix,
	}
	encryptedRedisPassword, encErr := encryptSecret(cfg.RedisPassword)
	if encErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "password encryption failed"})
		return
	}

	if cfg.Enabled {
		client := h.newRedisClient(cfg)
		defer client.Close()
		if pingErr := client.Ping(r.Context()).Err(); pingErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "redis connection failed"})
			return
		}
	}

	if current == nil {
		cfg.ID = uuid.New().String()
		_, err = h.db.ExecContext(r.Context(),
			`INSERT INTO cache_configs (id, user_id, enabled, semantic_enabled, semantic_threshold, semantic_max_candidates, semantic_embedding_model, redis_addr, redis_username, redis_password, redis_db, default_ttl_seconds, key_prefix)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			cfg.ID, userID, cfg.Enabled, cfg.SemanticEnabled, cfg.SemanticThreshold, cfg.SemanticMaxCandidates, cfg.SemanticEmbeddingModel,
			cfg.RedisAddr, cfg.RedisUsername, encryptedRedisPassword, cfg.RedisDB, cfg.DefaultTTLSeconds, cfg.KeyPrefix,
		)
	} else {
		cfg.ID = current.ID
		_, err = h.db.ExecContext(r.Context(),
			`UPDATE cache_configs
			 SET enabled=?, semantic_enabled=?, semantic_threshold=?, semantic_max_candidates=?, semantic_embedding_model=?, redis_addr=?, redis_username=?, redis_password=?, redis_db=?, default_ttl_seconds=?, key_prefix=?
			 WHERE user_id=?`,
			cfg.Enabled, cfg.SemanticEnabled, cfg.SemanticThreshold, cfg.SemanticMaxCandidates, cfg.SemanticEmbeddingModel,
			cfg.RedisAddr, cfg.RedisUsername, encryptedRedisPassword, cfg.RedisDB, cfg.DefaultTTLSeconds, cfg.KeyPrefix, userID,
		)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	cfg.UserID = userID
	cfg.HasPassword = strings.TrimSpace(cfg.RedisPassword) != ""
	cfg.RedisPassword = ""
	writeJSON(w, http.StatusOK, cfg)
}

// ── LLM Routes ──────────────────────────────────────────────────────────────

// ListRoutes returns all routes owned by the authenticated user.
func (h *Handler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT rt.id, rt.user_id, rt.name, rt.slug, rt.description,
		        rt.provider_id, COALESCE(p.name,'') AS provider_name,
		        rt.model, COALESCE(rt.system_prompt,'') AS system_prompt,
		        rt.temperature, rt.max_tokens, rt.stream_passthrough,
		        COALESCE(rt.prompt_version_id,''), rt.enforce_json_schema, COALESCE(rt.output_json_schema,''),
		        COALESCE(rt.failover_provider_ids,''), COALESCE(rt.allowed_models,''),
		        rt.enabled, rt.created_at, rt.updated_at
		 FROM llm_routes rt
		 LEFT JOIN providers p ON p.id = rt.provider_id
		 WHERE rt.user_id = ?
		 ORDER BY rt.created_at DESC`, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	routes := []models.LLMRoute{}
	for rows.Next() {
		var rt models.LLMRoute
		if err := rows.Scan(&rt.ID, &rt.UserID, &rt.Name, &rt.Slug, &rt.Description,
			&rt.ProviderID, &rt.ProviderName, &rt.Model, &rt.SystemPrompt,
			&rt.Temperature, &rt.MaxTokens, &rt.StreamPassthrough, &rt.PromptVersionID, &rt.EnforceJSONSchema, &rt.OutputJSONSchema,
			&rt.FailoverProviderIDs, &rt.AllowedModels,
			&rt.Enabled, &rt.CreatedAt, &rt.UpdatedAt); err != nil {
			continue
		}
		routes = append(routes, rt)
	}
	writeJSON(w, http.StatusOK, routes)
}

// GetRoute returns a single route by ID.
func (h *Handler) GetRoute(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")
	var rt models.LLMRoute
	err := h.db.QueryRowContext(r.Context(),
		`SELECT rt.id, rt.user_id, rt.name, rt.slug, rt.description,
		        rt.provider_id, COALESCE(p.name,'') AS provider_name,
		        rt.model, COALESCE(rt.system_prompt,'') AS system_prompt,
		        rt.temperature, rt.max_tokens, rt.stream_passthrough,
		        COALESCE(rt.prompt_version_id,''), rt.enforce_json_schema, COALESCE(rt.output_json_schema,''),
		        COALESCE(rt.failover_provider_ids,''), COALESCE(rt.allowed_models,''),
		        rt.enabled, rt.created_at, rt.updated_at
		 FROM llm_routes rt
		 LEFT JOIN providers p ON p.id = rt.provider_id
		 WHERE rt.id = ? AND rt.user_id = ?`, id, userID,
	).Scan(&rt.ID, &rt.UserID, &rt.Name, &rt.Slug, &rt.Description,
		&rt.ProviderID, &rt.ProviderName, &rt.Model, &rt.SystemPrompt,
		&rt.Temperature, &rt.MaxTokens, &rt.StreamPassthrough, &rt.PromptVersionID, &rt.EnforceJSONSchema, &rt.OutputJSONSchema,
		&rt.FailoverProviderIDs, &rt.AllowedModels,
		&rt.Enabled, &rt.CreatedAt, &rt.UpdatedAt)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "route not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	writeJSON(w, http.StatusOK, rt)
}

// CreateRoute creates a new LLM route for the authenticated user.
func (h *Handler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	var payload map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	body, _ := json.Marshal(payload)
	var rt models.LLMRoute
	if err := json.Unmarshal(body, &rt); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if _, ok := payload["stream_passthrough"]; !ok {
		rt.StreamPassthrough = true
	}

	if rt.Name == "" || rt.Slug == "" || rt.ProviderID == "" || rt.Model == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, slug, provider_id and model are required"})
		return
	}
	if rt.Temperature == 0 {
		rt.Temperature = 1.0
	}
	rt.ID = uuid.New().String()
	rt.UserID = userID
	rt.Enabled = true
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO llm_routes
		  (id, user_id, name, slug, description, provider_id, model, system_prompt, temperature, max_tokens, stream_passthrough, prompt_version_id, enforce_json_schema, output_json_schema, failover_provider_ids, allowed_models, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rt.ID, rt.UserID, rt.Name, rt.Slug, rt.Description, rt.ProviderID,
		rt.Model, rt.SystemPrompt, rt.Temperature, rt.MaxTokens, rt.StreamPassthrough, strings.TrimSpace(rt.PromptVersionID), rt.EnforceJSONSchema, strings.TrimSpace(rt.OutputJSONSchema),
		normalizeCSV(rt.FailoverProviderIDs), normalizeCSV(rt.AllowedModels), rt.Enabled,
	)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "slug already exists or provider not found"})
		return
	}
	writeJSON(w, http.StatusCreated, rt)
}

// UpdateRoute modifies an existing route owned by the user.
func (h *Handler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")
	var rt models.LLMRoute
	if err := json.NewDecoder(r.Body).Decode(&rt); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if rt.Temperature == 0 {
		rt.Temperature = 1.0
	}
	res, err := h.db.ExecContext(r.Context(),
		`UPDATE llm_routes
		 SET name=?, slug=?, description=?, provider_id=?, model=?,
		     system_prompt=?, temperature=?, max_tokens=?, stream_passthrough=?, prompt_version_id=?, enforce_json_schema=?, output_json_schema=?, failover_provider_ids=?, allowed_models=?, enabled=?
		 WHERE id=? AND user_id=?`,
		rt.Name, rt.Slug, rt.Description, rt.ProviderID, rt.Model,
		rt.SystemPrompt, rt.Temperature, rt.MaxTokens, rt.StreamPassthrough, strings.TrimSpace(rt.PromptVersionID), rt.EnforceJSONSchema, strings.TrimSpace(rt.OutputJSONSchema),
		normalizeCSV(rt.FailoverProviderIDs), normalizeCSV(rt.AllowedModels), rt.Enabled, id, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "route not found"})
		return
	}
	rt.ID = id
	rt.UserID = userID
	writeJSON(w, http.StatusOK, rt)
}

// DeleteRoute removes a route owned by the user.
func (h *Handler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")
	res, err := h.db.ExecContext(r.Context(),
		`DELETE FROM llm_routes WHERE id = ? AND user_id = ?`, id, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "route not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListProviderHealth returns provider circuit/open state for UI observability.
func (h *Handler) ListProviderHealth(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name, enabled FROM providers ORDER BY created_at ASC`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	health := []models.ProviderHealth{}
	for rows.Next() {
		var providerID, providerName string
		var enabled bool
		if scanErr := rows.Scan(&providerID, &providerName, &enabled); scanErr != nil {
			continue
		}

		providerCircuitMu.Lock()
		state := providerCircuits[providerID]
		providerCircuitMu.Unlock()

		circuitOpen := !state.openUntil.IsZero() && time.Now().Before(state.openUntil)
		var openUntil *time.Time
		if circuitOpen {
			t := state.openUntil
			openUntil = &t
		}

		health = append(health, models.ProviderHealth{
			ProviderID:          providerID,
			ProviderName:        providerName,
			Enabled:             enabled,
			CircuitOpen:         circuitOpen,
			ConsecutiveFailures: state.consecutiveFailures,
			OpenUntil:           openUntil,
		})
	}

	writeJSON(w, http.StatusOK, health)
}

// ListProviderKeyPoolStats returns runtime key-pool balancing telemetry.
func (h *Handler) ListProviderKeyPoolStats(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name, adapter, api_key, api_keys_json, enabled
		 FROM providers
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	type providerStat struct {
		ProviderID string                   `json:"provider_id"`
		Name       string                   `json:"name"`
		Adapter    string                   `json:"adapter"`
		Enabled    bool                     `json:"enabled"`
		KeyCount   int                      `json:"key_count"`
		Keys       []providerKeyRuntimeStat `json:"keys"`
	}

	result := make([]providerStat, 0)
	for rows.Next() {
		var provider models.Provider
		if scanErr := rows.Scan(&provider.ID, &provider.Name, &provider.Adapter, &provider.APIKey, &provider.APIKeysJSON, &provider.Enabled); scanErr != nil {
			continue
		}
		keys := providerKeys(provider)
		result = append(result, providerStat{
			ProviderID: provider.ID,
			Name:       provider.Name,
			Adapter:    provider.Adapter,
			Enabled:    provider.Enabled,
			KeyCount:   len(keys),
			Keys:       providerKeyRuntimeSnapshot(provider.ID, keys),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listCandidateProviders(ctx context.Context, preferPrimary bool, primaryID, routeFailoverProviders, apiKeyAllowedProviders string) ([]models.Provider, error) {
	providers := []models.Provider{}
	apiKeyAllowedSet := csvToSet(apiKeyAllowedProviders)
	routeFailoverSet := csvToSet(routeFailoverProviders)
	fallbackAllowedSet := combineAllowedProviderSets(routeFailoverProviders, apiKeyAllowedProviders)
	if preferPrimary && primaryID != "" {
		var primary models.Provider
		err := h.db.QueryRowContext(ctx,
			`SELECT id, name, base_url, adapter, api_version, api_key, api_keys_json, enabled, created_at, updated_at FROM providers WHERE id = ? AND enabled = 1`,
			primaryID,
		).Scan(&primary.ID, &primary.Name, &primary.BaseURL, &primary.Adapter, &primary.APIVersion, &primary.APIKey, &primary.APIKeysJSON, &primary.Enabled, &primary.CreatedAt, &primary.UpdatedAt)
		if err == nil {
			if decryptErr := decryptProviderInPlace(&primary); decryptErr != nil {
				return nil, decryptErr
			}
		}
		if err == nil && providerAllowed(primary.ID, apiKeyAllowedSet) {
			providers = append(providers, primary)
		}
	}

	query := `SELECT id, name, base_url, adapter, api_version, api_key, api_keys_json, enabled, created_at, updated_at FROM providers WHERE enabled = 1`
	args := []interface{}{}
	if primaryID != "" {
		query += ` AND id <> ?`
		args = append(args, primaryID)
	}
	query += ` ORDER BY created_at ASC`

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p models.Provider
		if scanErr := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.Adapter, &p.APIVersion, &p.APIKey, &p.APIKeysJSON, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); scanErr != nil {
			continue
		}
		if decryptErr := decryptProviderInPlace(&p); decryptErr != nil {
			return nil, decryptErr
		}
		if routeFailoverSet != nil && !routeFailoverSet[p.ID] {
			continue
		}
		if !providerAllowed(p.ID, fallbackAllowedSet) {
			continue
		}
		providers = append(providers, p)
	}
	return providers, nil
}

func (h *Handler) executeProviderRequest(
	ctx context.Context,
	userID string,
	apiKeyID *string,
	requestID string,
	routeSlug string,
	model string,
	providers []models.Provider,
	path string,
	bodyBytes []byte,
	timeout time.Duration,
	stream bool,
) (*models.Provider, *http.Response, int, error) {
	maxRetries := handlerEnvInt("PROVIDER_MAX_RETRIES", 2)
	lastLatency := 0
	client := &http.Client{Timeout: timeout}
	tracer := otel.Tracer("llm-gateway/provider")

	for idx := range providers {
		provider := &providers[idx]
		if providerCircuitOpen(provider.ID) {
			continue
		}
		keys := providerKeys(*provider)
		if len(keys) == 0 {
			continue
		}

		for attempt := 0; attempt <= maxRetries; attempt++ {
			selectedKey := provider.APIKey
			if shouldEnableKeyPool() {
				if key, ok := selectProviderAPIKey(provider.ID, keys); ok {
					selectedKey = key
				}
			}
			if strings.TrimSpace(selectedKey) == "" {
				break
			}

			providerWithKey := *provider
			providerWithKey.APIKey = selectedKey
			targetURL, headers, outboundBody, requestBuildErr := buildProviderRequest(providerWithKey, path, bodyBytes)
			if requestBuildErr != nil {
				if errors.Is(requestBuildErr, errProviderUnsupported) {
					break
				}
				h.logAudit(ctx, userID, apiKeyID, &provider.ID, requestID, path,
					"gateway_to_llm", routeSlug, model, 0, 0, false, requestBuildErr.Error(), nil)
				break
			}

			spanCtx, span := tracer.Start(ctx, "provider.request")
			span.SetAttributes(
				attribute.String("llm.provider_id", provider.ID),
				attribute.String("llm.provider_name", provider.Name),
				attribute.String("llm.model", model),
				attribute.String("http.url", targetURL),
				attribute.String("http.path", path),
				attribute.Int("retry.attempt", attempt),
			)

			start := time.Now()
			proxyReq, _ := http.NewRequestWithContext(spanCtx, http.MethodPost, targetURL, bytes.NewReader(outboundBody))
			proxyReq.Header = headers.Clone()
			h.logAudit(ctx, userID, apiKeyID, &provider.ID, requestID, path,
				"gateway_to_llm", routeSlug, model, 0, 0, true, "", outboundBody)

			resp, err := client.Do(proxyReq)
			lastLatency = int(time.Since(start).Milliseconds())
			span.SetAttributes(attribute.Int("latency_ms", lastLatency))
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.End()
				h.logAudit(ctx, userID, apiKeyID, &provider.ID, requestID, path,
					"llm_to_gateway", routeSlug, model, http.StatusBadGateway, lastLatency, false, err.Error(), nil)
				recordProviderFailure(provider.ID)
				markProviderAPIKeyCooldown(provider.ID, selectedKey, keyCooldownForError())
				if attempt < maxRetries {
					time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
					continue
				}
				break
			}

			respBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

			if shouldRetryStatus(resp.StatusCode) {
				span.SetStatus(codes.Error, "retryable provider status")
				span.End()
				h.logAudit(ctx, userID, apiKeyID, &provider.ID, requestID, path,
					"llm_to_gateway", routeSlug, model, resp.StatusCode, lastLatency, false, "retryable provider status", respBytes)
				recordProviderFailure(provider.ID)
				markProviderAPIKeyCooldown(provider.ID, selectedKey, keyCooldownForStatus(resp.StatusCode))
				if attempt < maxRetries {
					time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
					continue
				}
				if idx < len(providers)-1 {
					break
				}
				resp = cloneHTTPResponse(resp, respBytes)
				return provider, resp, lastLatency, nil
			}

			normalizedBytes := respBytes
			if !stream {
				normalizedBytes, err = normalizeProviderResponse(*provider, path, resp.StatusCode, respBytes)
				if err != nil {
					if errors.Is(err, errProviderUnsupported) {
						span.End()
						break
					}
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					span.End()
					h.logAudit(ctx, userID, apiKeyID, &provider.ID, requestID, path,
						"llm_to_gateway", routeSlug, model, http.StatusBadGateway, lastLatency, false, err.Error(), respBytes)
					recordProviderFailure(provider.ID)
					markProviderAPIKeyCooldown(provider.ID, selectedKey, keyCooldownForError())
					break
				}
			}
			resp = cloneHTTPResponse(resp, normalizedBytes)
			h.logAudit(ctx, userID, apiKeyID, &provider.ID, requestID, path,
				"llm_to_gateway", routeSlug, model, resp.StatusCode, lastLatency, true, "", normalizedBytes)

			recordProviderSuccess(provider.ID)
			span.SetStatus(codes.Ok, "")
			span.End()
			if shouldApplyLatencyPenalty(lastLatency) {
				markProviderAPIKeyCooldown(provider.ID, selectedKey, providerPenaltyCooldown())
			}
			return provider, resp, lastLatency, nil
		}
	}

	return nil, nil, lastLatency, fmt.Errorf("all providers unavailable")
}

func (h *Handler) calculateRequestCost(ctx context.Context, userID, providerID, model string, promptTokens, completionTokens int) float64 {
	if promptTokens == 0 && completionTokens == 0 {
		return 0
	}
	var inputRate, outputRate float64
	if err := h.db.QueryRowContext(ctx,
		`SELECT input_cost_per_1m, output_cost_per_1m FROM model_costs
		 WHERE user_id = ? AND provider_id = ? AND model = ?`,
		userID, providerID, model,
	).Scan(&inputRate, &outputRate); err != nil {
		return 0
	}
	return float64(promptTokens)/1_000_000*inputRate + float64(completionTokens)/1_000_000*outputRate
}

func (h *Handler) getCacheConfigByUser(ctx context.Context, userID string) (*models.CacheConfig, error) {
	var cfg models.CacheConfig
	var password sql.NullString
	err := h.db.QueryRowContext(ctx,
		`SELECT id, user_id, enabled, semantic_enabled, semantic_threshold, semantic_max_candidates, COALESCE(semantic_embedding_model,''),
		        redis_addr, COALESCE(redis_username,''), redis_password,
		        redis_db, default_ttl_seconds, key_prefix, created_at, updated_at
		 FROM cache_configs WHERE user_id = ?`,
		userID,
	).Scan(
		&cfg.ID, &cfg.UserID, &cfg.Enabled, &cfg.SemanticEnabled, &cfg.SemanticThreshold, &cfg.SemanticMaxCandidates, &cfg.SemanticEmbeddingModel,
		&cfg.RedisAddr, &cfg.RedisUsername, &password,
		&cfg.RedisDB, &cfg.DefaultTTLSeconds, &cfg.KeyPrefix, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cfg.HasPassword = password.Valid && strings.TrimSpace(password.String) != ""
	if cfg.HasPassword {
		decryptedPassword, decErr := decryptSecret(password.String)
		if decErr != nil {
			return nil, decErr
		}
		cfg.RedisPassword = decryptedPassword
	}
	if cfg.DefaultTTLSeconds <= 0 {
		cfg.DefaultTTLSeconds = 300
	}
	if cfg.SemanticThreshold <= 0 {
		cfg.SemanticThreshold = 0.9
	}
	if cfg.SemanticMaxCandidates <= 0 {
		cfg.SemanticMaxCandidates = 30
	}
	if strings.TrimSpace(cfg.SemanticEmbeddingModel) == "" {
		cfg.SemanticEmbeddingModel = "text-embedding-3-small"
	}
	if strings.TrimSpace(cfg.KeyPrefix) == "" {
		cfg.KeyPrefix = "llm-gw"
	}
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		cfg.RedisAddr = "localhost:6379"
	}
	return &cfg, nil
}

func (h *Handler) newRedisClient(cfg models.CacheConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Username: cfg.RedisUsername,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
}

func (h *Handler) cacheKey(cfg models.CacheConfig, userID, model string, body []byte) string {
	hash := sha256.Sum256(body)
	prefix := strings.TrimSpace(cfg.KeyPrefix)
	if prefix == "" {
		prefix = "llm-gw"
	}
	return fmt.Sprintf("%s:%s:%s:%s", prefix, userID, model, hex.EncodeToString(hash[:]))
}

func (h *Handler) getCachedChatCompletion(ctx context.Context, userID, model string, body []byte) (*cachePayload, bool, error) {
	cfg, err := h.getCacheConfigByUser(ctx, userID)
	if err != nil || cfg == nil || !cfg.Enabled {
		return nil, false, err
	}
	client := h.newRedisClient(*cfg)
	defer client.Close()

	key := h.cacheKey(*cfg, userID, model, body)
	cached, getErr := client.Get(ctx, key).Bytes()
	if getErr == redis.Nil {
		return nil, false, nil
	}
	if getErr != nil {
		return nil, false, getErr
	}

	var payload cachePayload
	if unmarshalErr := json.Unmarshal(cached, &payload); unmarshalErr != nil {
		return nil, false, unmarshalErr
	}
	return &payload, true, nil
}

func (h *Handler) setCachedChatCompletion(ctx context.Context, userID, model string, body []byte, providerID string, statusCode int, responseBody []byte, promptText string, promptEmbedding []float64) error {
	cfg, err := h.getCacheConfigByUser(ctx, userID)
	if err != nil || cfg == nil || !cfg.Enabled {
		return err
	}
	if cfg.DefaultTTLSeconds <= 0 {
		return nil
	}
	payload := cachePayload{
		StatusCode:      statusCode,
		ProviderID:      providerID,
		Body:            responseBody,
		PromptText:      promptText,
		PromptEmbedding: promptEmbedding,
	}
	encoded, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return marshalErr
	}

	client := h.newRedisClient(*cfg)
	defer client.Close()
	key := h.cacheKey(*cfg, userID, model, body)
	if err := client.Set(ctx, key, encoded, time.Duration(cfg.DefaultTTLSeconds)*time.Second).Err(); err != nil {
		return err
	}

	if cfg.SemanticEnabled && len(promptEmbedding) > 0 {
		indexKey := cacheSemanticIndexKey(*cfg, userID, model)
		if err := client.LRem(ctx, indexKey, 0, key).Err(); err != nil && err != redis.Nil {
			return nil
		}
		if err := client.LPush(ctx, indexKey, key).Err(); err == nil {
			_ = client.LTrim(ctx, indexKey, 0, int64(cfg.SemanticMaxCandidates-1)).Err()
			_ = client.Expire(ctx, indexKey, time.Duration(cfg.DefaultTTLSeconds*2)*time.Second).Err()
		}
	}

	return nil
}

func parseUsageTokens(respBytes []byte) (int, int, int) {
	var usage struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &usage); err != nil {
		return 0, 0, 0
	}
	return usage.Usage.PromptTokens, usage.Usage.CompletionTokens, usage.Usage.TotalTokens
}

func copyProviderHeaders(w http.ResponseWriter, resp *http.Response) {
	for key, values := range resp.Header {
		canonical := http.CanonicalHeaderKey(key)
		if canonical == "Content-Length" {
			continue
		}
		// Do not forward headers that could override gateway security policy,
		// expose internal auth challenges, or set cookies on the client.
		if providerResponseHeaderDenylist[canonical] {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
}

func shouldRetryStatus(status int) bool {
	return status == http.StatusUnauthorized ||
		status == http.StatusForbidden ||
		status == http.StatusRequestTimeout ||
		status == http.StatusTooManyRequests ||
		status >= http.StatusInternalServerError
}

func providerCircuitOpen(providerID string) bool {
	providerCircuitMu.Lock()
	defer providerCircuitMu.Unlock()
	state := providerCircuits[providerID]
	if state.openUntil.IsZero() {
		return false
	}
	if time.Now().Before(state.openUntil) {
		return true
	}
	state.openUntil = time.Time{}
	providerCircuits[providerID] = state
	return false
}

func recordProviderFailure(providerID string) {
	providerCircuitMu.Lock()
	defer providerCircuitMu.Unlock()
	state := providerCircuits[providerID]
	state.consecutiveFailures++
	if state.consecutiveFailures >= handlerEnvInt("CIRCUIT_BREAKER_THRESHOLD", 3) {
		state.openUntil = time.Now().Add(time.Duration(handlerEnvInt("CIRCUIT_BREAKER_OPEN_SECONDS", 60)) * time.Second)
		state.consecutiveFailures = 0
	}
	providerCircuits[providerID] = state
}

func recordProviderSuccess(providerID string) {
	providerCircuitMu.Lock()
	defer providerCircuitMu.Unlock()
	providerCircuits[providerID] = providerCircuitState{}
}

func handlerEnvInt(name string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func normalizeCSV(v string) string {
	items := splitCSV(v)
	if len(items) == 0 {
		return ""
	}
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return strings.Join(out, ",")
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := []string{}
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func isModelAllowed(model, csv string) bool {
	allowed := splitCSV(csv)
	if len(allowed) == 0 {
		return true
	}
	for _, m := range allowed {
		if strings.EqualFold(m, model) {
			return true
		}
	}
	return false
}

func combineAllowedProviderSets(a, b string) map[string]bool {
	setA := csvToSet(a)
	setB := csvToSet(b)
	if setA == nil && setB == nil {
		return nil
	}
	if setA == nil {
		return setB
	}
	if setB == nil {
		return setA
	}
	out := map[string]bool{}
	for id := range setA {
		if setB[id] {
			out[id] = true
		}
	}
	return out
}

func csvToSet(v string) map[string]bool {
	items := splitCSV(v)
	if len(items) == 0 {
		return nil
	}
	out := map[string]bool{}
	for _, item := range items {
		out[item] = true
	}
	return out
}

func providerAllowed(providerID string, allowed map[string]bool) bool {
	if allowed == nil {
		return true
	}
	return allowed[providerID]
}

// isValidProviderURL checks that a provider base URL is safe to use as an
// outbound request target, blocking SSRF via private/internal IP ranges.
// Note: DNS-rebinding SSRF (hostname that resolves to private IP) is not
// caught here; network-level egress filtering is recommended for full protection.
func isValidProviderURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	// Block loopback by hostname (case-insensitive).
	lower := strings.ToLower(host)
	if lower == "localhost" || lower == "::1" {
		return false
	}
	// If the host is an IP address, check it against private ranges.
	if ip := net.ParseIP(host); ip != nil {
		return !isPrivateOrReservedIP(ip)
	}
	return true
}

// isPrivateOrReservedIP returns true if ip falls within a private or reserved
// IP range that should never be an outbound provider request target.
func isPrivateOrReservedIP(ip net.IP) bool {
	for _, network := range privateIPNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// generateJWT creates a signed HS256 JWT for the given user ID.
func generateJWT(userID, role string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	claims := jwt.MapClaims{
		"sub": userID,
		"role": role,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Ensure fmt is used (avoids unused import error when some branches are removed).
var _ = fmt.Sprintf
