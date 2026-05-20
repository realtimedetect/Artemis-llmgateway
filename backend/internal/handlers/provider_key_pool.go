package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"llm-gatway/internal/models"
)

type providerKeyBalancer struct {
	cursor    int
	cooldowns map[string]time.Time
	selected  map[string]int
}

type providerKeyRuntimeStat struct {
	KeyID                    string     `json:"key_id"`
	SelectionCount           int        `json:"selection_count"`
	CooldownUntil            *time.Time `json:"cooldown_until,omitempty"`
	CooldownRemainingSeconds int        `json:"cooldown_remaining_seconds"`
	Available                bool       `json:"available"`
}

var (
	providerKeyBalancerMu sync.Mutex
	providerKeyBalancers  = map[string]*providerKeyBalancer{}
)

func providerKeys(provider models.Provider) []string {
	keys := make([]string, 0, 4)
	primary, _ := decryptSecret(provider.APIKey)
	primary = strings.TrimSpace(primary)
	if primary != "" {
		keys = append(keys, primary)
	}

	if strings.TrimSpace(provider.APIKeysJSON) != "" {
		var extras []string
		if json.Unmarshal([]byte(provider.APIKeysJSON), &extras) == nil {
			for _, item := range extras {
				plain, err := decryptSecret(item)
				if err != nil {
					continue
				}
				keys = append(keys, plain)
			}
		}
	}

	clean := make([]string, 0, len(keys))
	seen := map[string]bool{}
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		clean = append(clean, trimmed)
	}
	return clean
}

func decryptProviderInPlace(provider *models.Provider) error {
	if provider == nil {
		return nil
	}
	if strings.TrimSpace(provider.APIKey) != "" {
		plain, err := decryptSecret(provider.APIKey)
		if err != nil {
			return err
		}
		provider.APIKey = plain
	}
	if strings.TrimSpace(provider.APIKeysJSON) != "" {
		var raw []string
		if err := json.Unmarshal([]byte(provider.APIKeysJSON), &raw); err == nil {
			decoded := make([]string, 0, len(raw))
			for _, item := range raw {
				plain, decErr := decryptSecret(item)
				if decErr != nil {
					return decErr
				}
				decoded = append(decoded, plain)
			}
			provider.APIKeysJSON = encodeAPIKeyPoolJSON(decoded)
		}
	}
	return nil
}

func normalizeProviderAPIKeys(primary string, extras []string) []string {
	trimmedPrimary := strings.TrimSpace(primary)
	clean := make([]string, 0, len(extras))
	seen := map[string]bool{}
	if trimmedPrimary != "" {
		seen[trimmedPrimary] = true
	}
	for _, key := range extras {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		clean = append(clean, trimmed)
	}
	sort.Strings(clean)
	return clean
}

func providerKeyCount(provider models.Provider) int {
	return len(providerKeys(provider))
}

func selectProviderAPIKey(providerID string, keys []string) (string, bool) {
	if len(keys) == 0 {
		return "", false
	}

	now := time.Now()
	providerKeyBalancerMu.Lock()
	defer providerKeyBalancerMu.Unlock()

	state := providerKeyBalancers[providerID]
	if state == nil {
		state = &providerKeyBalancer{cooldowns: map[string]time.Time{}, selected: map[string]int{}}
		providerKeyBalancers[providerID] = state
	}

	start := 0
	if len(keys) > 0 {
		start = state.cursor % len(keys)
	}

	for offset := 0; offset < len(keys); offset++ {
		idx := (start + offset) % len(keys)
		candidate := keys[idx]
		until := state.cooldowns[candidate]
		if !until.IsZero() && now.Before(until) {
			continue
		}
		state.cursor = idx + 1
		state.selected[candidate] = state.selected[candidate] + 1
		return candidate, true
	}

	return "", false
}

func markProviderAPIKeyCooldown(providerID, key string, duration time.Duration) {
	if strings.TrimSpace(providerID) == "" || strings.TrimSpace(key) == "" || duration <= 0 {
		return
	}
	providerKeyBalancerMu.Lock()
	defer providerKeyBalancerMu.Unlock()
	state := providerKeyBalancers[providerID]
	if state == nil {
		state = &providerKeyBalancer{cooldowns: map[string]time.Time{}, selected: map[string]int{}}
		providerKeyBalancers[providerID] = state
	}
	state.cooldowns[key] = time.Now().Add(duration)
}

func providerKeyRuntimeSnapshot(providerID string, keys []string) []providerKeyRuntimeStat {
	providerKeyBalancerMu.Lock()
	defer providerKeyBalancerMu.Unlock()

	state := providerKeyBalancers[providerID]
	now := time.Now()
	stats := make([]providerKeyRuntimeStat, 0, len(keys))

	for _, key := range keys {
		stat := providerKeyRuntimeStat{
			KeyID:          maskedKeyID(key),
			SelectionCount: 0,
			Available:      true,
		}
		if state != nil {
			stat.SelectionCount = state.selected[key]
			if until := state.cooldowns[key]; !until.IsZero() {
				if now.Before(until) {
					stat.Available = false
					remaining := int(until.Sub(now).Seconds())
					if remaining < 1 {
						remaining = 1
					}
					stat.CooldownRemainingSeconds = remaining
					copyUntil := until
					stat.CooldownUntil = &copyUntil
				}
			}
		}
		stats = append(stats, stat)
	}

	return stats
}

func maskedKeyID(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return "key_" + hex.EncodeToString(sum[:])[:12]
}

func keyCooldownForStatus(status int) time.Duration {
	switch status {
	case 429:
		return time.Duration(handlerEnvInt("PROVIDER_KEY_429_COOLDOWN_SECONDS", 45)) * time.Second
	case 401, 403:
		return time.Duration(handlerEnvInt("PROVIDER_KEY_AUTH_COOLDOWN_SECONDS", 300)) * time.Second
	default:
		return time.Duration(handlerEnvInt("PROVIDER_KEY_ERROR_COOLDOWN_SECONDS", 10)) * time.Second
	}
}

func keyCooldownForError() time.Duration {
	return time.Duration(handlerEnvInt("PROVIDER_KEY_ERROR_COOLDOWN_SECONDS", 10)) * time.Second
}

func parseAPIKeyPoolJSON(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var keys []string
	if json.Unmarshal([]byte(raw), &keys) != nil {
		return []string{}
	}
	return keys
}

func encodeAPIKeyPoolJSON(keys []string) string {
	encoded, err := json.Marshal(keys)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func providerLatencyThresholdMs() int {
	return handlerEnvInt("PROVIDER_HIGH_LATENCY_MS", 0)
}

func providerPenaltyCooldown() time.Duration {
	seconds := handlerEnvInt("PROVIDER_LATENCY_PENALTY_SECONDS", 15)
	if seconds < 0 {
		seconds = 0
	}
	return time.Duration(seconds) * time.Second
}

func shouldApplyLatencyPenalty(latencyMs int) bool {
	threshold := providerLatencyThresholdMs()
	return threshold > 0 && latencyMs > threshold
}

func shouldEnableKeyPool() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("PROVIDER_KEY_POOL_ENABLED")))
	if value == "" {
		return true
	}
	return value != "0" && value != "false" && value != "no"
}
