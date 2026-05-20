package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/google/uuid"
	"llm-gatway/internal/models"
	"llm-gatway/internal/policies"
	"llm-gatway/internal/middleware"
)

var policyEngine *policies.Engine

// InitPolicies initializes the policy engine (must be called from main).
func (h *Handler) InitPolicies() error {
	policyEngine = policies.NewEngine()

	// Load all policies from database
	allPolicies, err := h.getAllPoliciesFromDB()
	if err != nil {
		return fmt.Errorf("failed to load policies: %v", err)
	}

	if err := policyEngine.LoadPolicies(allPolicies); err != nil {
		return err
	}

	// Set the policy check handler in middleware
	middleware.PolicyCheckHandler = h.makePolicyChecker()

	return nil
}

// makePolicyChecker returns a function that checks policies for a request.
func (h *Handler) makePolicyChecker() func(userID string, model string, content string) (bool, string) {
	return func(userID string, model string, content string) (bool, string) {
		if policyEngine == nil {
			return true, "" // Allow if engine not initialized
		}

		ctx := &policies.EvaluationContext{
			Model:       model,
			UserID:      userID,
			Content:     content,
			ContentFull: content,
		}

		result, err := policyEngine.EvaluateRequest(model, ctx)
		if err != nil {
			// Log error but allow request to proceed
			fmt.Printf("Policy evaluation error: %v\n", err)
			return true, ""
		}

		return result.Allowed, result.DenyReason
	}
}

// CreatePolicy creates a new policy.
// POST /api/policies
func (h *Handler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(models.User)

	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Scope       models.PolicyScope     `json:"scope"`
		ModelName   *string                `json:"model_name,omitempty"`
		Pattern     string                 `json:"pattern"`
		Target      models.PolicyFieldTarget `json:"target"`
		Action      models.PolicyAction    `json:"action"`
		Priority    int                    `json:"priority"`
		Enabled     bool                   `json:"enabled"`
		Notes       string                 `json:"notes,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.Pattern == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pattern is required"})
		return
	}

	// Validate regex pattern
	if _, err := regexp.Compile(req.Pattern); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid regex pattern: %v", err)})
		return
	}

	// Validate scope-specific requirements
	if req.Scope == models.PolicyScopeLocal && (req.ModelName == nil || *req.ModelName == "") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model_name is required for local policies"})
		return
	}

	if req.Scope == models.PolicyScopeGlobal {
		req.ModelName = nil // Clear model name for global policies
	}

	policy := models.Policy{
		ID:          uuid.New().String(),
		UserID:      user.ID,
		Name:        req.Name,
		Description: req.Description,
		Scope:       req.Scope,
		ModelName:   req.ModelName,
		Pattern:     req.Pattern,
		Target:      req.Target,
		Action:      req.Action,
		Priority:    req.Priority,
		Enabled:     req.Enabled,
		Notes:       req.Notes,
	}

	// Insert into database
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO policies (id, user_id, name, description, scope, model_name, pattern, target, action, priority, enabled, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		policy.ID, policy.UserID, policy.Name, policy.Description,
		policy.Scope, policy.ModelName, policy.Pattern, policy.Target, policy.Action, policy.Priority, policy.Enabled, policy.Notes,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create policy"})
		return
	}

	// Reload policies in engine
	if err := h.ReloadPoliciesEngine(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "policy created but failed to update engine"})
		return
	}

	writeJSON(w, http.StatusCreated, policy)
}

// GetPolicies retrieves all policies for the user.
// GET /api/policies
func (h *Handler) GetPolicies(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(models.User)

	scope := r.URL.Query().Get("scope") // "global", "local", or empty for all
	model := r.URL.Query().Get("model")

	var rows *sql.Rows
	var err error

	if scope != "" {
		if model != "" && scope == "local" {
			rows, err = h.db.QueryContext(r.Context(),
				`SELECT id, user_id, name, description, scope, model_name, pattern, target, action, priority, enabled, notes, created_at, updated_at
				 FROM policies WHERE user_id = ? AND scope = ? AND model_name = ? ORDER BY priority, created_at ASC`,
				user.ID, scope, model,
			)
		} else {
			rows, err = h.db.QueryContext(r.Context(),
				`SELECT id, user_id, name, description, scope, model_name, pattern, target, action, priority, enabled, notes, created_at, updated_at
				 FROM policies WHERE user_id = ? AND scope = ? ORDER BY priority, created_at ASC`,
				user.ID, scope,
			)
		}
	} else if model != "" {
		// Get policies for a specific model (global + local)
		rows, err = h.db.QueryContext(r.Context(),
			`SELECT id, user_id, name, description, scope, model_name, pattern, target, action, priority, enabled, notes, created_at, updated_at
			 FROM policies WHERE user_id = ? AND (scope = 'global' OR model_name = ?) ORDER BY priority, created_at ASC`,
			user.ID, model,
		)
	} else {
		rows, err = h.db.QueryContext(r.Context(),
			`SELECT id, user_id, name, description, scope, model_name, pattern, target, action, priority, enabled, notes, created_at, updated_at
			 FROM policies WHERE user_id = ? ORDER BY priority, created_at ASC`,
			user.ID,
		)
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to retrieve policies"})
		return
	}
	defer rows.Close()

	policies := []models.Policy{}
	for rows.Next() {
		var p models.Policy
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Scope, &p.ModelName,
			&p.Pattern, &p.Target, &p.Action, &p.Priority, &p.Enabled, &p.Notes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to parse policies"})
			return
		}
		policies = append(policies, p)
	}

	writeJSON(w, http.StatusOK, models.PolicyListResponse{
		Policies: policies,
		Total:    len(policies),
	})
}

// GetPolicy retrieves a single policy.
// GET /api/policies/{id}
func (h *Handler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(models.User)
	policyID := r.PathValue("id")

	var p models.Policy
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, user_id, name, description, scope, model_name, pattern, target, action, priority, enabled, notes, created_at, updated_at
		 FROM policies WHERE id = ? AND user_id = ?`,
		policyID, user.ID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Scope, &p.ModelName,
		&p.Pattern, &p.Target, &p.Action, &p.Priority, &p.Enabled, &p.Notes, &p.CreatedAt, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "policy not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to retrieve policy"})
		return
	}

	writeJSON(w, http.StatusOK, p)
}

// UpdatePolicy updates an existing policy.
// PUT /api/policies/{id}
func (h *Handler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(models.User)
	policyID := r.PathValue("id")

	var req struct {
		Name        *string                  `json:"name,omitempty"`
		Description *string                  `json:"description,omitempty"`
		Pattern     *string                  `json:"pattern,omitempty"`
		Target      *models.PolicyFieldTarget `json:"target,omitempty"`
		Action      *models.PolicyAction     `json:"action,omitempty"`
		Priority    *int                     `json:"priority,omitempty"`
		Enabled     *bool                    `json:"enabled,omitempty"`
		Notes       *string                  `json:"notes,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate regex if provided
	if req.Pattern != nil && *req.Pattern != "" {
		if _, err := regexp.Compile(*req.Pattern); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid regex pattern: %v", err)})
			return
		}
	}

	// Fetch existing policy first
	var p models.Policy
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, user_id, name, description, scope, model_name, pattern, target, action, priority, enabled, notes, created_at, updated_at
		 FROM policies WHERE id = ? AND user_id = ?`,
		policyID, user.ID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Scope, &p.ModelName,
		&p.Pattern, &p.Target, &p.Action, &p.Priority, &p.Enabled, &p.Notes, &p.CreatedAt, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "policy not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to retrieve policy"})
		return
	}

	// Apply updates
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.Pattern != nil {
		p.Pattern = *req.Pattern
	}
	if req.Target != nil {
		p.Target = *req.Target
	}
	if req.Action != nil {
		p.Action = *req.Action
	}
	if req.Priority != nil {
		p.Priority = *req.Priority
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
	if req.Notes != nil {
		p.Notes = *req.Notes
	}

	// Update in database
	_, err = h.db.ExecContext(r.Context(),
		`UPDATE policies SET name = ?, description = ?, pattern = ?, target = ?, action = ?, priority = ?, enabled = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND user_id = ?`,
		p.Name, p.Description, p.Pattern, p.Target, p.Action, p.Priority, p.Enabled, p.Notes, policyID, user.ID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update policy"})
		return
	}

	// Reload policies in engine
	if err := h.ReloadPoliciesEngine(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "policy updated but failed to update engine"})
		return
	}

	writeJSON(w, http.StatusOK, p)
}

// DeletePolicy deletes a policy.
// DELETE /api/policies/{id}
func (h *Handler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(models.User)
	policyID := r.PathValue("id")

	result, err := h.db.ExecContext(r.Context(),
		`DELETE FROM policies WHERE id = ? AND user_id = ?`,
		policyID, user.ID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete policy"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "policy not found"})
		return
	}

	// Reload policies in engine
	if err := h.ReloadPoliciesEngine(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "policy deleted but failed to update engine"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "policy deleted successfully"})
}

// EvaluatePolicy evaluates policies against a sample request for testing.
// POST /api/policies/evaluate
func (h *Handler) EvaluatePolicy(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(models.User)

	var req models.PolicyEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.ModelName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model_name is required"})
		return
	}

	if policyEngine == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "policy engine not initialized"})
		return
	}

	ctx := &policies.EvaluationContext{
		Model:       req.ModelName,
		UserID:      user.ID,
		Prompt:      req.PromptText,
		Content:     req.ContentText,
		ContentFull: req.ContentText,
	}

	result, err := policyEngine.EvaluateRequest(req.ModelName, ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("evaluation error: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetPoliciesForModel returns applicable policies for a specific model.
// GET /api/policies/model/{model_name}
func (h *Handler) GetPoliciesForModel(w http.ResponseWriter, r *http.Request) {
	modelName := r.PathValue("model_name")

	if policyEngine == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "policy engine not initialized"})
		return
	}

	policies := policyEngine.ListPoliciesForModel(modelName)
	writeJSON(w, http.StatusOK, models.PolicyListResponse{
		Policies: policies,
		Total:    len(policies),
	})
}

// ReloadPoliciesEngine reloads all policies from the database into the engine.
func (h *Handler) ReloadPoliciesEngine() error {
	allPolicies, err := h.getAllPoliciesFromDB()
	if err != nil {
		return err
	}

	if policyEngine == nil {
		return fmt.Errorf("policy engine not initialized")
	}

	return policyEngine.LoadPolicies(allPolicies)
}

// getAllPoliciesFromDB retrieves all policies from database.
func (h *Handler) getAllPoliciesFromDB() ([]models.Policy, error) {
	rows, err := h.db.Query(
		`SELECT id, user_id, name, description, scope, model_name, pattern, target, action, priority, enabled, notes, created_at, updated_at
		 FROM policies ORDER BY priority, created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := []models.Policy{}
	for rows.Next() {
		var p models.Policy
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Scope, &p.ModelName,
			&p.Pattern, &p.Target, &p.Action, &p.Priority, &p.Enabled, &p.Notes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}

	return policies, rows.Err()
}

// GetAuditMetricsForPolicy returns metrics about how often a policy matches.
// GET /api/policies/{id}/metrics
func (h *Handler) GetAuditMetricsForPolicy(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(models.User)
	policyID := r.PathValue("id")
	days := r.URL.Query().Get("days")

	if days == "" {
		days = "7"
	}

	daysInt, err := strconv.Atoi(days)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid days parameter"})
		return
	}

	// Verify policy exists
	var policyName string
	err = h.db.QueryRowContext(r.Context(),
		`SELECT name FROM policies WHERE id = ? AND user_id = ?`,
		policyID, user.ID,
	).Scan(&policyName)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "policy not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to retrieve policy"})
		return
	}

	// Get metrics (count of matches in last N days)
	// Note: This would require audit log entries to track which policies matched
	// For now, return a basic response
	metrics := map[string]interface{}{
		"policy_id":   policyID,
		"policy_name": policyName,
		"days":        daysInt,
		"total_matches": 0, // Would be populated from audit logs
		"allow_count": 0,
		"deny_count": 0,
	}

	writeJSON(w, http.StatusOK, metrics)
}
