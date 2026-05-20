package models

import "time"

// User represents a gateway user.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Role      string    `json:"role"`
	PlanID    string    `json:"plan_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Plan defines a license plan and optional monthly token cap.
// If MonthlyTokenLimit is nil, the plan is unlimited.
type Plan struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	MonthlyTokenLimit *int64  `json:"monthly_token_limit,omitempty"`
	Description       string  `json:"description,omitempty"`
}

// Provider holds LLM provider configuration.
type Provider struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	BaseURL   string    `json:"base_url"`
	Adapter   string    `json:"adapter"`
	APIVersion string   `json:"api_version,omitempty"`
	APIKey    string    `json:"-"` // Never serialised to JSON
	APIKeysJSON string  `json:"-"` // Stored as JSON array for key-pool load balancing
	KeyCount  int       `json:"key_count,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKey represents a gateway API key (key material is only returned on creation).
type APIKey struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	Name               string     `json:"name"`
	KeyPrefix          string     `json:"key_prefix"`
	GroupID            *string    `json:"group_id,omitempty"`
	GroupName          string     `json:"group_name,omitempty"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	AllowedProviderIDs string     `json:"allowed_provider_ids,omitempty"`
	AllowedModels      string     `json:"allowed_models,omitempty"`
	RequestCount       int        `json:"request_count,omitempty"`
	TotalTokens        int        `json:"total_tokens,omitempty"`
	TotalCostUSD       float64    `json:"total_cost_usd,omitempty"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
	// PlainKey is only populated when first created.
	PlainKey string `json:"key,omitempty"`
}

// Request is a logged LLM request.
type Request struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	APIKeyID         *string   `json:"api_key_id,omitempty"`
	ProviderID       *string   `json:"provider_id,omitempty"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	TTFTMs           int       `json:"ttft_ms"`
	LatencyMs        int       `json:"latency_ms"`
	Status           int       `json:"status"`
	CostUSD          float64   `json:"cost_usd"`
	CreatedAt        time.Time `json:"created_at"`
}

// ChatCompletionRequest mirrors the OpenAI chat/completions request body.
type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// Message is a single chat turn.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// UsageSummary holds aggregated usage figures.
type UsageSummary struct {
	TotalRequests int     `json:"total_requests"`
	TotalTokens   int     `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost_usd"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
}

// LoginRequest is the JSON body for /api/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is returned after a successful login or register.
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// ModelCost holds per-user token pricing for a specific provider + model.
type ModelCost struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	ProviderID      string    `json:"provider_id"`
	ProviderName    string    `json:"provider_name,omitempty"`
	Model           string    `json:"model"`
	InputCostPer1M  float64   `json:"input_cost_per_1m"`
	OutputCostPer1M float64   `json:"output_cost_per_1m"`
	Currency        string    `json:"currency"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CostGroup is a logical spend bucket for API keys and request aggregation.
type CostGroup struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CostGroupSpend is aggregated usage for a spend bucket.
type CostGroupSpend struct {
	GroupID      string  `json:"group_id"`
	GroupName    string  `json:"group_name"`
	Requests     int     `json:"requests"`
	TotalTokens  int     `json:"total_tokens"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

// CacheConfig stores per-user Redis cache settings.
type CacheConfig struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	Enabled           bool      `json:"enabled"`
	SemanticEnabled   bool      `json:"semantic_enabled"`
	SemanticThreshold float64   `json:"semantic_threshold"`
	SemanticMaxCandidates int   `json:"semantic_max_candidates"`
	SemanticEmbeddingModel string `json:"semantic_embedding_model"`
	RedisAddr         string    `json:"redis_addr"`
	RedisUsername     string    `json:"redis_username"`
	RedisPassword     string    `json:"redis_password,omitempty"`
	RedisDB           int       `json:"redis_db"`
	DefaultTTLSeconds int       `json:"default_ttl_seconds"`
	KeyPrefix         string    `json:"key_prefix"`
	HasPassword       bool      `json:"has_password,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// RoutingConfig stores per-user smart routing preferences.
type RoutingConfig struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id"`
	SmartEnabled        bool      `json:"smart_enabled"`
	CostWeight          float64   `json:"cost_weight"`
	PerformanceWeight   float64   `json:"performance_weight"`
	ComplexityThreshold int       `json:"complexity_threshold"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// LLMRoute maps a named slug to a specific provider + model with optional defaults.
type LLMRoute struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id"`
	Name                string    `json:"name"`
	Slug                string    `json:"slug"`
	Description         string    `json:"description"`
	ProviderID          string    `json:"provider_id"`
	ProviderName        string    `json:"provider_name,omitempty"` // joined, read-only
	Model               string    `json:"model"`
	SystemPrompt        string    `json:"system_prompt,omitempty"`
	Temperature         float64   `json:"temperature"`
	MaxTokens           int       `json:"max_tokens"`
	StreamPassthrough   bool      `json:"stream_passthrough"`
	FailoverProviderIDs string    `json:"failover_provider_ids,omitempty"`
	AllowedModels       string    `json:"allowed_models,omitempty"`
	PromptVersionID     string    `json:"prompt_version_id,omitempty"`
	EnforceJSONSchema   bool      `json:"enforce_json_schema"`
	OutputJSONSchema    string    `json:"output_json_schema,omitempty"`
	Enabled             bool      `json:"enabled"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// PromptTemplate is a centrally managed prompt collection with version history.
type PromptTemplate struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	ActiveVersionID string    `json:"active_version_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PromptVersion stores one immutable version for a prompt template.
type PromptVersion struct {
	ID           string    `json:"id"`
	TemplateID   string    `json:"template_id"`
	UserID       string    `json:"user_id"`
	Version      int       `json:"version"`
	Content      string    `json:"content"`
	TestInput    string    `json:"test_input,omitempty"`
	TestOutput   string    `json:"test_output,omitempty"`
	TestStatus   int       `json:"test_status"`
	CreatedAt    time.Time `json:"created_at"`
	ActivatedAt  time.Time `json:"activated_at"`
	IsActive     bool      `json:"is_active"`
}

type ProviderHealth struct {
	ProviderID          string     `json:"provider_id"`
	ProviderName        string     `json:"provider_name"`
	Enabled             bool       `json:"enabled"`
	CircuitOpen         bool       `json:"circuit_open"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	OpenUntil           *time.Time `json:"open_until,omitempty"`
}

// AuditLog captures request/response traffic between gateway and providers.
type AuditLog struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	APIKeyID   *string   `json:"api_key_id,omitempty"`
	ProviderID *string   `json:"provider_id,omitempty"`
	RequestID  string    `json:"request_id"`
	Endpoint   string    `json:"endpoint"`
	Direction  string    `json:"direction"`
	RouteSlug  string    `json:"route_slug"`
	Model      string    `json:"model"`
	HTTPStatus int       `json:"http_status"`
	LatencyMs  int       `json:"latency_ms"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	Payload    string    `json:"payload"`
	CreatedAt  time.Time `json:"created_at"`
}

// PolicyScope defines whether a policy is global or model-specific.
type PolicyScope string

const (
	PolicyScopeGlobal PolicyScope = "global"
	PolicyScopeLocal  PolicyScope = "local"
)

// PolicyAction defines the result of a policy evaluation.
type PolicyAction string

const (
	PolicyActionAllow PolicyAction = "allow"
	PolicyActionDeny  PolicyAction = "deny"
)

// PolicyFieldTarget defines which request field to match against.
type PolicyFieldTarget string

const (
	PolicyTargetModel        PolicyFieldTarget = "model"
	PolicyTargetContent      PolicyFieldTarget = "content"
	PolicyTargetUser         PolicyFieldTarget = "user"
	PolicyTargetProvider     PolicyFieldTarget = "provider"
	PolicyTargetPrompt       PolicyFieldTarget = "prompt"
	PolicyTargetContentFull  PolicyFieldTarget = "content_full"
)

// Policy defines a regex-based rule that can be applied globally or to specific models.
type Policy struct {
	ID              string         `json:"id"`
	UserID          string         `json:"user_id"`
	Name            string         `json:"name"`
	Description     string         `json:"description,omitempty"`
	Scope           PolicyScope    `json:"scope"` // "global" or "local"
	ModelName       *string        `json:"model_name,omitempty"` // Required for local policies
	Pattern         string         `json:"pattern"` // Regex pattern to match
	Target          PolicyFieldTarget `json:"target"` // Which field to apply pattern to
	Action          PolicyAction   `json:"action"` // "allow" or "deny"
	Priority        int            `json:"priority"` // Lower number = higher priority
	Enabled         bool           `json:"enabled"`
	Notes           string         `json:"notes,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// PolicyEvaluationResult contains the result of policy evaluation.
type PolicyEvaluationResult struct {
	Allowed      bool     `json:"allowed"`
	MatchedRules []string `json:"matched_rules,omitempty"` // IDs of policies that matched
	DenyReason   string   `json:"deny_reason,omitempty"`
}

// PolicyListResponse wraps a list of policies with metadata.
type PolicyListResponse struct {
	Policies []Policy `json:"policies"`
	Total    int      `json:"total"`
}

// PolicyEvaluationRequest is the request body for manual policy evaluation.
type PolicyEvaluationRequest struct {
	ModelName   string `json:"model_name"`
	PromptText  string `json:"prompt_text,omitempty"`
	ContentText string `json:"content_text,omitempty"`
}

// ============= TELEMETRY MODELS =============

// LiveMetrics represents real-time gateway metrics snapshot
type LiveMetrics struct {
	Timestamp             time.Time              `json:"timestamp"`
	TotalRequests         int64                  `json:"total_requests"`
	RequestsPerSecond     float64                `json:"requests_per_second"`
	TotalCostUSD          float64                `json:"total_cost_usd"`
	AverageLatencyMs      float64                `json:"average_latency_ms"`
	P50LatencyMs          float64                `json:"p50_latency_ms"`
	P90LatencyMs          float64                `json:"p90_latency_ms"`
	P99LatencyMs          float64                `json:"p99_latency_ms"`
	MaxLatencyMs          float64                `json:"max_latency_ms"`
	SuccessRate           float64                `json:"success_rate"` // percentage
	TotalTokens           int64                  `json:"total_tokens"`
	ProviderMetrics       []ProviderMetrics      `json:"provider_metrics"`
	ActiveProviders       int                    `json:"active_providers"`
	FailedRequests        int64                  `json:"failed_requests"`
	TimeWindow            string                 `json:"time_window"` // "1m", "5m", "1h"
}

// ProviderMetrics contains metrics for a single LLM provider
type ProviderMetrics struct {
	ProviderID       string    `json:"provider_id"`
	ProviderName     string    `json:"provider_name"`
	RequestCount     int64     `json:"request_count"`
	SuccessCount     int64     `json:"success_count"`
	FailureCount     int64     `json:"failure_count"`
	TotalTokens      int64     `json:"total_tokens"`
	TotalCostUSD     float64   `json:"total_cost_usd"`
	AverageLatencyMs float64   `json:"average_latency_ms"`
	P50LatencyMs     float64   `json:"p50_latency_ms"`
	P90LatencyMs     float64   `json:"p90_latency_ms"`
	P99LatencyMs     float64   `json:"p99_latency_ms"`
	LastRequestAt    time.Time `json:"last_request_at,omitempty"`
	Enabled          bool      `json:"enabled"`
}

// RequestMetric stores individual request telemetry data
type RequestMetric struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	APIKeyID         *string   `json:"api_key_id,omitempty"`
	ProviderID       string    `json:"provider_id"`
	ProviderName     string    `json:"provider_name"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	LatencyMs        int       `json:"latency_ms"` // End-to-end latency
	TTFTMs           int       `json:"ttft_ms"`   // Time-to-first-token
	Status           int       `json:"status"`    // HTTP status
	CostUSD          float64   `json:"cost_usd"`
	RequestID        string    `json:"request_id"`
	Success          bool      `json:"success"`
	Error            string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// LatencyPercentiles stores calculated percentiles for reporting
type LatencyPercentiles struct {
	P50  float64 `json:"p50"`
	P75  float64 `json:"p75"`
	P90  float64 `json:"p90"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	P999 float64 `json:"p999"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Avg  float64 `json:"avg"`
}

// RouteLog represents a single route log entry for live display
type RouteLog struct {
	ID           string    `json:"id"`
	RequestID    string    `json:"request_id"`
	UserID       string    `json:"user_id"`
	SourceRoute  string    `json:"source_route"`
	TargetProvider string  `json:"target_provider"`
	ProviderName string    `json:"provider_name"`
	Model        string    `json:"model"`
	LatencyMs    int       `json:"latency_ms"`
	Tokens       int       `json:"tokens"`
	CostUSD      float64   `json:"cost_usd"`
	Status       int       `json:"status"`
	Error        string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// TelemetrySnapshot represents a snapshot for time-series storage
type TelemetrySnapshot struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Timestamp    time.Time `json:"timestamp"`
	TPS          float64   `json:"tps"`              // Transactions per second
	AvgLatency   float64   `json:"avg_latency_ms"`
	P50Latency   float64   `json:"p50_latency_ms"`
	P90Latency   float64   `json:"p90_latency_ms"`
	P99Latency   float64   `json:"p99_latency_ms"`
	SuccessRate  float64   `json:"success_rate"`    // percentage
	TotalCost    float64   `json:"total_cost_usd"`
	TotalRequests int64    `json:"total_requests"`
	FailedRequests int64   `json:"failed_requests"`
	TotalTokens  int64     `json:"total_tokens"`
}

// ModelMetricsSnapshot contains per-model metrics snapshot
type ModelMetricsSnapshot struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Model        string    `json:"model"`
	Provider     string    `json:"provider"`
	Timestamp    time.Time `json:"timestamp"`
	RequestCount int64     `json:"request_count"`
	AvgLatency   float64   `json:"avg_latency_ms"`
	P90Latency   float64   `json:"p90_latency_ms"`
	TotalCost    float64   `json:"total_cost_usd"`
	SuccessRate  float64   `json:"success_rate"`
}
