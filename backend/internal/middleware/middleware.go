package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserIDKey contextKey = "userID"
const UserRoleKey contextKey = "userRole"
const APIKeyIDKey contextKey = "apiKeyID"
const APIKeyAllowedProvidersKey contextKey = "apiKeyAllowedProviders"
const APIKeyAllowedModelsKey contextKey = "apiKeyAllowedModels"

type rateEntry struct {
	windowStart time.Time
	count       int
}

var (
	rateMu      sync.Mutex
	rateEntries = map[string]rateEntry{}
)

// RateLimit enforces a fixed-window in-memory request limit using a key extractor.
func RateLimit(limit int, window time.Duration, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	if keyFunc == nil {
		keyFunc = RateLimitIPKey
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if key == "" {
				key = "anon"
			}

			now := time.Now().UTC()
			rateMu.Lock()
			entry := rateEntries[key]
			if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= window {
				entry = rateEntry{windowStart: now, count: 1}
				rateEntries[key] = entry
				rateMu.Unlock()
				next.ServeHTTP(w, r)
				return
			}

			if entry.count >= limit {
				retryIn := int(window.Seconds()) - int(now.Sub(entry.windowStart).Seconds())
				if retryIn < 1 {
					retryIn = 1
				}
				rateMu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", strconv.Itoa(retryIn))
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}

			entry.count++
			rateEntries[key] = entry
			rateMu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitUserKey keys the limit using authenticated user ID if present, otherwise by IP.
func RateLimitUserKey(r *http.Request) string {
	if apiKeyID, ok := r.Context().Value(APIKeyIDKey).(string); ok && apiKeyID != "" {
		return "k:" + apiKeyID
	}
	if userID, ok := r.Context().Value(UserIDKey).(string); ok && userID != "" {
		return "u:" + userID
	}
	return "ip:" + clientIP(r)
}

// RateLimitIPKey keys the limit using client IP for unauthenticated routes.
func RateLimitIPKey(r *http.Request) string {
	return "ip:" + clientIP(r)
}

func clientIP(r *http.Request) string {
	// X-Forwarded-For and X-Real-IP headers can be spoofed by clients.
	// Only trust them when TRUST_PROXY=1 is explicitly set, indicating the
	// application is running behind a trusted reverse proxy.
	if strings.TrimSpace(os.Getenv("TRUST_PROXY")) != "" {
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				ip := strings.TrimSpace(parts[0])
				if ip != "" {
					return ip
				}
			}
		}
		ip := strings.TrimSpace(r.Header.Get("X-Real-IP"))
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

// AuthRequired validates the JWT bearer token and injects the user ID into the request context.
func AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr, err := bearerToken(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, err.Error())
			return
		}

		userID, role, err := validateJWT(tokenStr)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, UserRoleKey, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthOrAPIKeyRequired allows either a user JWT or a gateway API key for inference endpoints.
func AuthOrAPIKeyRequired(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			credential, source, err := requestCredential(r)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, err.Error())
				return
			}

			if source == "x-api-key" || strings.HasPrefix(credential, "gw-") {
				userID, apiKeyID, allowedProviders, allowedModels, authErr := validateAPIKey(r.Context(), db, credential)
				if authErr != nil {
					writeAuthError(w, http.StatusUnauthorized, authErr.Error())
					return
				}
				ctx := context.WithValue(r.Context(), UserIDKey, userID)
				ctx = context.WithValue(ctx, APIKeyIDKey, apiKeyID)
				ctx = context.WithValue(ctx, APIKeyAllowedProvidersKey, allowedProviders)
				ctx = context.WithValue(ctx, APIKeyAllowedModelsKey, allowedModels)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			userID, role, authErr := validateJWT(credential)
			if authErr != nil {
				writeAuthError(w, http.StatusUnauthorized, authErr.Error())
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, UserRoleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ReadOnlyForNonAdmin blocks mutating requests for non-admin users.
func ReadOnlyForNonAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		role, _ := r.Context().Value(UserRoleKey).(string)
		if strings.EqualFold(role, "admin") {
			next.ServeHTTP(w, r)
			return
		}
		writeAuthError(w, http.StatusForbidden, "read-only access for non-admin users")
	})
}

// RequireAdmin ensures the authenticated user has admin role.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(UserRoleKey).(string)
		if !strings.EqualFold(role, "admin") {
			writeAuthError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestCredential(r *http.Request) (string, string, error) {
	if apiKey := strings.TrimSpace(r.Header.Get("X-API-Key")); apiKey != "" {
		return apiKey, "x-api-key", nil
	}
	tokenStr, err := bearerToken(r)
	if err != nil {
		return "", "", err
	}
	return tokenStr, "authorization", nil
}

func bearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errString("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errString("invalid authorization header format")
	}
	return parts[1], nil
}

func validateJWT(tokenStr string) (string, string, error) {
	secret := os.Getenv("JWT_SECRET")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return "", "", errString("invalid or expired token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errString("invalid token claims")
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return "", "", errString("invalid token subject")
	}

	role, _ := claims["role"].(string)
	if strings.TrimSpace(role) == "" {
		role = "user"
	}

	return userID, role, nil
}

func validateAPIKey(ctx context.Context, db *sql.DB, rawKey string) (string, string, string, string, error) {
	if len(rawKey) < 10 {
		return "", "", "", "", errString("invalid api key")
	}
	prefix := rawKey[:10]
	rows, err := db.QueryContext(ctx,
		`SELECT id, user_id, key_hash, expires_at, allowed_provider_ids, allowed_models FROM api_keys WHERE key_prefix = ?`, prefix,
	)
	if err != nil {
		return "", "", "", "", errString("database error")
	}
	defer rows.Close()

	now := time.Now().UTC()
	for rows.Next() {
		var apiKeyID, userID, keyHash string
		var expiresAt sql.NullTime
		var allowedProviderIDs, allowedModels string
		if scanErr := rows.Scan(&apiKeyID, &userID, &keyHash, &expiresAt, &allowedProviderIDs, &allowedModels); scanErr != nil {
			continue
		}
		if expiresAt.Valid && expiresAt.Time.Before(now) {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(rawKey)) == nil {
			return userID, apiKeyID, allowedProviderIDs, allowedModels, nil
		}
	}
	return "", "", "", "", errString("invalid api key")
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Use json.Marshal to prevent JSON injection from special characters in message.
	b, _ := json.Marshal(map[string]string{"error": message})
	_, _ = w.Write(b)
}

type errString string

func (e errString) Error() string { return string(e) }

// PolicyCheckHandler is a function type that evaluates policies for a request.
// This is injected by the main package after the policy engine is initialized.
var PolicyCheckHandler func(userID string, model string, content string) (bool, string) = func(userID, model, content string) (bool, string) {
	// Default: allow all requests if policy engine is not set
	return true, ""
}

// PolicyCheck enforces policy rules on inference requests.
// This middleware checks global and model-specific policies to allow/deny requests.
func PolicyCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value(UserIDKey).(string)
		if userID == "" {
			// No user context, skip policy check
			next.ServeHTTP(w, r)
			return
		}

		// For POST requests, extract model and content from JSON body
		if r.Method == http.MethodPost {
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				// If we can't parse the body, let the handler deal with it
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}

			// Restore the body for the next handler
			bodyBytes, _ := json.Marshal(body)
			r.Body = http.NoBody // We've already read it
			// Create a new body reader for the next handler
			r = r.Clone(r.Context())
			r.Body = strings.NewReader(string(bodyBytes))

			// Extract model and content
			model, _ := body["model"].(string)
			var content string
			if messages, ok := body["messages"].([]interface{}); ok && len(messages) > 0 {
				if msg, ok := messages[len(messages)-1].(map[string]interface{}); ok {
					if c, ok := msg["content"].(string); ok {
						content = c
					}
				}
			}

			// Evaluate policies
			allowed, denyReason := PolicyCheckHandler(userID, model, content)
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": denyReason})
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
