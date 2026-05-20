package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"

	"llm-gatway/internal/database"
	"llm-gatway/internal/handlers"
	"llm-gatway/internal/middleware"
	"llm-gatway/internal/telemetry"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading environment variables directly")
	}

	// Enforce a minimum JWT secret length to prevent trivially-guessable tokens.
	if len(os.Getenv("JWT_SECRET")) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters long")
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "llm-gateway"
	}
	shutdownTracing, err := telemetry.InitTracer(context.Background(), serviceName)
	if err != nil {
		log.Printf("OpenTelemetry init failed: %v", err)
	}
	defer func() {
		_ = shutdownTracing(context.Background())
	}()

	h := handlers.New(db)

	// Initialize policy engine after creating handlers
	if err := h.InitPolicies(); err != nil {
		log.Printf("Warning: Failed to initialize policy engine: %v", err)
	}

	// Initialize telemetry aggregator (60-second window, keep last 10000 metrics)
	aggregator := telemetry.NewAggregator(time.Minute, 10000)
	h.SetAggregator(aggregator)
	log.Println("Telemetry aggregator initialized")

	r := chi.NewRouter()

	// Public routes
	r.Get("/health", h.HealthCheck)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimit(authRateLimit, time.Minute, middleware.RateLimitIPKey))
		r.Post("/api/auth/login", h.Login)
	})

	// Inference routes accept either JWT or gateway API key.
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthOrAPIKeyRequired(db))
		r.Use(middleware.RateLimit(apiRateLimit, time.Minute, middleware.RateLimitUserKey))
		r.Use(middleware.PolicyCheck) // Enforce policies

		// LLM gateway
		r.Post("/api/chat/completions", h.ChatCompletions)
		r.Post("/api/embeddings", h.Embeddings)
		r.Post("/api/agent/run", h.AgentRun)
		r.Post("/v1/chat/completions", h.ChatCompletions)
		r.Post("/v1/embeddings", h.Embeddings)
		r.Post("/v1/agent/run", h.AgentRun)
		r.Get("/v1/models", h.ListUnifiedModels)
	})

	// Management routes remain JWT-only.
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthRequired)
		r.Use(middleware.RateLimit(apiRateLimit, time.Minute, middleware.RateLimitUserKey))
		r.Use(middleware.ReadOnlyForNonAdmin)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin)
			r.Post("/api/users", h.AdminCreateUser)
			r.Get("/api/admin/users", h.ListUsersForAdmin)
			r.Get("/api/admin/plans", h.ListPlans)
			r.Get("/api/admin/license/status", h.GetAdminLicenseStatus)
			r.Post("/api/admin/license/activate", h.ActivateProfessionalLicense)
			r.Put("/api/admin/users/{id}/plan", h.UpdateUserPlan)
			r.Get("/api/admin/providers/key-pool-stats", h.ListProviderKeyPoolStats)
		})

		// API key management
		r.Get("/api/keys", h.ListAPIKeys)
		r.Post("/api/keys", h.CreateAPIKey)
		r.Put("/api/keys/{id}/group", h.AssignAPIKeyGroup)
		r.Delete("/api/keys/{id}", h.DeleteAPIKey)

		// Provider management
		r.Get("/api/providers", h.ListProviders)
		r.Get("/api/providers/health", h.ListProviderHealth)
		r.Post("/api/providers", h.CreateProvider)
		r.Put("/api/providers/{id}", h.UpdateProvider)
		r.Delete("/api/providers/{id}", h.DeleteProvider)

		// LLM route management
		r.Get("/api/routes", h.ListRoutes)
		r.Get("/api/routes/{id}", h.GetRoute)
		r.Post("/api/routes", h.CreateRoute)
		r.Put("/api/routes/{id}", h.UpdateRoute)
		r.Delete("/api/routes/{id}", h.DeleteRoute)
		r.Get("/api/prompts/templates", h.ListPromptTemplates)
		r.Post("/api/prompts/templates", h.CreatePromptTemplate)
		r.Get("/api/prompts/templates/{id}/versions", h.ListPromptVersions)
		r.Post("/api/prompts/templates/{id}/versions", h.CreatePromptVersion)
		r.Put("/api/prompts/templates/{id}/active", h.ActivatePromptVersion)
		r.Post("/api/prompts/test", h.TestPromptVersion)
		r.Get("/api/routing/config", h.GetRoutingConfig)
		r.Put("/api/routing/config", h.UpsertRoutingConfig)

		// Cost configuration
		r.Get("/api/costs", h.ListCosts)
		r.Post("/api/costs", h.CreateCost)
		r.Put("/api/costs/{id}", h.UpdateCost)
		r.Delete("/api/costs/{id}", h.DeleteCost)
		r.Get("/api/cost-groups", h.ListCostGroups)
		r.Post("/api/cost-groups", h.CreateCostGroup)
		r.Put("/api/cost-groups/{id}", h.UpdateCostGroup)
		r.Delete("/api/cost-groups/{id}", h.DeleteCostGroup)

		// User groups & team management
		r.Get("/api/user-groups", h.ListGroups)
		r.Post("/api/user-groups", h.CreateGroup)
		r.Get("/api/user-groups/{groupID}", h.GetGroup)
		r.Put("/api/user-groups/{groupID}", h.UpdateGroup)
		r.Delete("/api/user-groups/{groupID}", h.DeleteGroup)
		r.Post("/api/user-groups/{groupID}/members", h.AddMember)
		r.Get("/api/user-groups/{groupID}/members", h.ListMembers)
		r.Delete("/api/user-groups/{groupID}/members/{memberID}", h.RemoveMember)
		r.Get("/api/user-groups/{groupID}/analytics", h.GetGroupAnalytics)
		r.Get("/api/user-groups/{groupID}/breakdown", h.GetGroupBreakdown)

		// Cache configuration
		r.Get("/api/cache/config", h.GetCacheConfig)
		r.Put("/api/cache/config", h.UpsertCacheConfig)

		// Policy management (global and per-model)
		r.Get("/api/policies", h.GetPolicies)
		r.Post("/api/policies", h.CreatePolicy)
		r.Get("/api/policies/{id}", h.GetPolicy)
		r.Put("/api/policies/{id}", h.UpdatePolicy)
		r.Delete("/api/policies/{id}", h.DeletePolicy)
		r.Post("/api/policies/evaluate", h.EvaluatePolicy)
		r.Get("/api/policies/model/{model_name}", h.GetPoliciesForModel)
		r.Get("/api/policies/{id}/metrics", h.GetAuditMetricsForPolicy)

		// Telemetry and real-time metrics
		r.Get("/api/telemetry/live", h.GetLiveMetrics)
		r.Get("/api/telemetry/providers", h.GetProviderMetrics)
		r.Get("/api/telemetry/providers/{provider_id}", h.GetProviderMetric)
		r.Get("/api/telemetry/percentiles", h.GetLatencyPercentiles)
		r.Get("/api/telemetry/routes/log", h.GetRouteLog)
		r.Get("/api/telemetry/dashboard", h.GetDashboardSnapshot)
		r.Get("/api/telemetry/stats", h.GetMetricsStats)
		r.Get("/api/telemetry/health", h.TelemetryHealthCheck)

		// Usage & analytics
		r.Get("/api/usage", h.GetUsage)
		r.Get("/api/requests", h.ListRequests)
		r.Get("/api/audits", h.ListAudits)
		r.Get("/api/analytics/observability", h.GetObservabilityMetrics)
		r.Get("/api/analytics/api-keys", h.GetAPIKeyAnalytics)
		r.Get("/api/analytics/cost-breakdown", h.GetCostBreakdown)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("LLM Gateway server running on :%s\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnvInt(name string, fallback int) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Printf("Invalid %s=%q, using default %d", name, v, fallback)
		return fallback
	}
	return n
}

// limitRequestBody caps incoming request body size to prevent memory exhaustion.
// LLM requests with large contexts are well under 10 MiB in practice.
const maxRequestBodyBytes = 10 * 1024 * 1024 // 10 MiB

func limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// securityHeaders adds HTTP response headers that harden against common browser-
// based attacks. These are especially important because the frontend SPA reads
// JWT tokens from localStorage, making XSS particularly dangerous.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}
