package policies

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"llm-gatway/internal/models"
)

// Engine evaluates policies against requests.
type Engine struct {
	policies map[string][]models.Policy // keyed by scope (global, model-name)
	compiled map[string]*regexp.Regexp   // cache of compiled regex patterns
	mu       sync.RWMutex
}

// NewEngine creates a new policy evaluation engine.
func NewEngine() *Engine {
	return &Engine{
		policies: make(map[string][]models.Policy),
		compiled: make(map[string]*regexp.Regexp),
	}
}

// LoadPolicies loads a set of policies into the engine.
// This should be called at startup and when policies change.
func (e *Engine) LoadPolicies(policies []models.Policy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing data
	e.policies = make(map[string][]models.Policy)

	// Group policies by scope
	globalPolicies := []models.Policy{}
	modelPolicies := make(map[string][]models.Policy)

	for _, policy := range policies {
		if !policy.Enabled {
			continue // Skip disabled policies
		}

		// Validate regex pattern early
		if _, err := regexp.Compile(policy.Pattern); err != nil {
			return fmt.Errorf("invalid regex pattern for policy %s: %v", policy.ID, err)
		}

		if policy.Scope == models.PolicyScopeGlobal {
			globalPolicies = append(globalPolicies, policy)
		} else if policy.Scope == models.PolicyScopeLocal && policy.ModelName != nil {
			modelName := *policy.ModelName
			modelPolicies[modelName] = append(modelPolicies[modelName], policy)
		}
	}

	// Sort by priority (lower number = higher priority)
	sortByPriority(globalPolicies)
	for _, policies := range modelPolicies {
		sortByPriority(policies)
	}

	e.policies["global"] = globalPolicies

	for modelName, modelPolicies := range modelPolicies {
		e.policies[modelName] = modelPolicies
	}

	// Pre-compile all patterns for fast evaluation
	e.compiled = make(map[string]*regexp.Regexp)
	for _, policy := range policies {
		if policy.Enabled {
			compiled, _ := regexp.Compile(policy.Pattern)
			e.compiled[policy.ID] = compiled
		}
	}

	return nil
}

// EvaluateRequest evaluates policies for a specific request.
// Returns true if the request is allowed, false if it should be denied.
func (e *Engine) EvaluateRequest(modelName string, request *EvaluationContext) (models.PolicyEvaluationResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := models.PolicyEvaluationResult{
		Allowed:      true,
		MatchedRules: []string{},
	}

	// Get effective policies: global + model-specific
	effectivePolicies := []models.Policy{}

	if globalPolicies, exists := e.policies["global"]; exists {
		effectivePolicies = append(effectivePolicies, globalPolicies...)
	}

	if modelPolicies, exists := e.policies[modelName]; exists {
		effectivePolicies = append(effectivePolicies, modelPolicies...)
	}

	// Sort by priority to ensure evaluation order
	sortByPriority(effectivePolicies)

	// Evaluate each policy in priority order
	for _, policy := range effectivePolicies {
		matches, err := e.matchPolicy(policy, request)
		if err != nil {
			return result, fmt.Errorf("error evaluating policy %s: %v", policy.ID, err)
		}

		if matches {
			result.MatchedRules = append(result.MatchedRules, policy.ID)

			if policy.Action == models.PolicyActionDeny {
				result.Allowed = false
				result.DenyReason = fmt.Sprintf("Policy '%s' denied the request", policy.Name)
				return result // Stop at first deny (deny takes precedence)
			}
		}
	}

	return result, nil
}

// matchPolicy checks if a single policy matches the request context.
func (e *Engine) matchPolicy(policy models.Policy, ctx *EvaluationContext) (bool, error) {
	if !policy.Enabled {
		return false, nil
	}

	compiledRegex, exists := e.compiled[policy.ID]
	if !exists {
		var err error
		compiledRegex, err = regexp.Compile(policy.Pattern)
		if err != nil {
			return false, err
		}
		e.compiled[policy.ID] = compiledRegex
	}

	// Extract the target field from the context
	targetText := ""
	switch policy.Target {
	case models.PolicyTargetModel:
		targetText = ctx.Model
	case models.PolicyTargetContent:
		// Match against individual messages
		targetText = ctx.Content
	case models.PolicyTargetUser:
		targetText = ctx.UserID
	case models.PolicyTargetProvider:
		targetText = ctx.Provider
	case models.PolicyTargetPrompt:
		targetText = ctx.Prompt
	case models.PolicyTargetContentFull:
		// Match against concatenated messages
		targetText = ctx.ContentFull
	default:
		return false, fmt.Errorf("unknown policy target: %s", policy.Target)
	}

	// Match the regex against the target text
	return compiledRegex.MatchString(targetText), nil
}

// EvaluateManual evaluates policies against manual input (for testing).
func (e *Engine) EvaluateManual(modelName string, prompt string, content string, contentFull string) (models.PolicyEvaluationResult, error) {
	ctx := &EvaluationContext{
		Model:       modelName,
		Prompt:      prompt,
		Content:     content,
		ContentFull: contentFull,
	}
	return e.EvaluateRequest(modelName, ctx)
}

// ListPoliciesForModel returns all applicable policies for a model (global + local).
func (e *Engine) ListPoliciesForModel(modelName string) []models.Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []models.Policy
	if globalPolicies, exists := e.policies["global"]; exists {
		result = append(result, globalPolicies...)
	}
	if modelPolicies, exists := e.policies[modelName]; exists {
		result = append(result, modelPolicies...)
	}
	return result
}

// EvaluationContext holds the request context for policy evaluation.
type EvaluationContext struct {
	Model       string // The LLM model being used
	UserID      string // The user making the request
	Provider    string // The provider
	Prompt      string // The system prompt (if any)
	Content     string // Single message content
	ContentFull string // Full conversation content
}

// Helper function to sort policies by priority (ascending).
func sortByPriority(policies []models.Policy) {
	sort.Slice(policies, func(i, j int) bool {
		if policies[i].Priority != policies[j].Priority {
			return policies[i].Priority < policies[j].Priority
		}
		// If priority is same, sort by creation time (stable order)
		return policies[i].CreatedAt.Before(policies[j].CreatedAt)
	})
}
