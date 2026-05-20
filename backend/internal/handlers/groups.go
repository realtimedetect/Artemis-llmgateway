package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"llm-gatway/internal/middleware"
)

type UserGroup struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MemberCount int       `json:"member_count"`
}

type GroupMember struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type GroupToken struct {
	GroupID       string  `json:"group_id"`
	GroupName     string  `json:"group_name"`
	TotalRequests int     `json:"total_requests"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
}

type GroupAnalytics struct {
	GroupID         string  `json:"group_id"`
	GroupName       string  `json:"group_name"`
	TotalRequests   int     `json:"total_requests"`
	TotalTokens     int64   `json:"total_tokens"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
	AvgLatencyMs    int     `json:"avg_latency_ms"`
	MemberCount     int     `json:"member_count"`
	TopModel        string  `json:"top_model"`
	TopProvider     string  `json:"top_provider"`
}

// CreateGroup creates a new user group
func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Group name is required", http.StatusBadRequest)
		return
	}

	groupID := uuid.New().String()
	now := time.Now()

	_, err := h.db.Exec(
		`INSERT INTO user_groups (id, owner_id, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		groupID, userID, req.Name, req.Description, now, now,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			http.Error(w, "A group with that name already exists", http.StatusConflict)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to create group: %v", err), http.StatusInternalServerError)
		return
	}

	group := UserGroup{
		ID:          groupID,
		OwnerID:     userID,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
		MemberCount: 0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(group)
}

// ListGroups lists all groups owned by the user
func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	rows, err := h.db.Query(
		`SELECT g.id, g.owner_id, g.name, g.description, g.created_at, g.updated_at,
		        COALESCE(COUNT(m.id), 0) as member_count
		 FROM user_groups g
		 LEFT JOIN user_group_members m ON g.id = m.group_id
		 WHERE g.owner_id = ?
		 GROUP BY g.id
		 ORDER BY g.created_at DESC`,
		userID,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list groups: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	groups := []UserGroup{}
	for rows.Next() {
		var g UserGroup
		if err := rows.Scan(&g.ID, &g.OwnerID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt, &g.MemberCount); err != nil {
			http.Error(w, fmt.Sprintf("Failed to scan group: %v", err), http.StatusInternalServerError)
			return
		}
		groups = append(groups, g)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

// GetGroup retrieves a specific group with members
func (h *Handler) GetGroup(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	groupID := r.PathValue("groupID")

	var g UserGroup
	err := h.db.QueryRow(
		`SELECT id, owner_id, name, description, created_at, updated_at
		 FROM user_groups
		 WHERE id = ? AND owner_id = ?`,
		groupID, userID,
	).Scan(&g.ID, &g.OwnerID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get group: %v", err), http.StatusInternalServerError)
		return
	}

	// Count members
	err = h.db.QueryRow(`SELECT COUNT(*) FROM user_group_members WHERE group_id = ?`, groupID).Scan(&g.MemberCount)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to count members: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g)
}

// UpdateGroup updates a group name and description
func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	groupID := r.PathValue("groupID")

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Group name is required", http.StatusBadRequest)
		return
	}

	result, err := h.db.Exec(
		`UPDATE user_groups SET name = ?, description = ?, updated_at = ?
		 WHERE id = ? AND owner_id = ?`,
		req.Name, req.Description, time.Now(), groupID, userID,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			http.Error(w, "A group with that name already exists", http.StatusConflict)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to update group: %v", err), http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Group updated successfully"})
}

// DeleteGroup deletes a group
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	groupID := r.PathValue("groupID")

	result, err := h.db.Exec(
		`DELETE FROM user_groups WHERE id = ? AND owner_id = ?`,
		groupID, userID,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete group: %v", err), http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Group deleted successfully"})
}

// AddMember adds a user to a group
func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	groupID := r.PathValue("groupID")

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	if req.Role == "" {
		req.Role = "member"
	}

	// Verify group ownership
	var ownerID string
	err := h.db.QueryRow(`SELECT owner_id FROM user_groups WHERE id = ?`, groupID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}
	if err != nil || ownerID != userID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	// Get the user by email
	var targetUserID string
	err = h.db.QueryRow(`SELECT id FROM users WHERE email = ?`, req.Email).Scan(&targetUserID)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find user: %v", err), http.StatusInternalServerError)
		return
	}

	memberID := uuid.New().String()
	_, err = h.db.Exec(
		`INSERT INTO user_group_members (id, group_id, user_id, role, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		memberID, groupID, targetUserID, req.Role, time.Now(),
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			http.Error(w, "User is already a member of this group", http.StatusConflict)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to add member: %v", err), http.StatusInternalServerError)
		return
	}

	member := GroupMember{
		ID:        memberID,
		GroupID:   groupID,
		UserID:    targetUserID,
		Email:     req.Email,
		Role:      req.Role,
		CreatedAt: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(member)
}

// ListMembers lists members of a group
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	groupID := r.PathValue("groupID")

	// Verify group ownership
	var ownerID string
	err := h.db.QueryRow(`SELECT owner_id FROM user_groups WHERE id = ?`, groupID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}
	if err != nil || ownerID != userID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	rows, err := h.db.Query(
		`SELECT m.id, m.group_id, m.user_id, u.email, m.role, m.created_at
		 FROM user_group_members m
		 JOIN users u ON m.user_id = u.id
		 WHERE m.group_id = ?
		 ORDER BY m.created_at DESC`,
		groupID,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list members: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	members := []GroupMember{}
	for rows.Next() {
		var m GroupMember
		if err := rows.Scan(&m.ID, &m.GroupID, &m.UserID, &m.Email, &m.Role, &m.CreatedAt); err != nil {
			http.Error(w, fmt.Sprintf("Failed to scan member: %v", err), http.StatusInternalServerError)
			return
		}
		members = append(members, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

// RemoveMember removes a user from a group
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	groupID := r.PathValue("groupID")
	memberID := r.PathValue("memberID")

	// Verify group ownership
	var ownerID string
	err := h.db.QueryRow(`SELECT owner_id FROM user_groups WHERE id = ?`, groupID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}
	if err != nil || ownerID != userID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	result, err := h.db.Exec(
		`DELETE FROM user_group_members WHERE id = ? AND group_id = ?`,
		memberID, groupID,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to remove member: %v", err), http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Member not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Member removed successfully"})
}

// GetGroupAnalytics retrieves token usage analytics for a group
func (h *Handler) GetGroupAnalytics(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	groupID := r.PathValue("groupID")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	// Verify group ownership
	var ownerID, groupName string
	err := h.db.QueryRow(`SELECT owner_id, name FROM user_groups WHERE id = ?`, groupID).Scan(&ownerID, &groupName)
	if err == sql.ErrNoRows {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}
	if err != nil || ownerID != userID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	// Calculate date range
	var dateStr string
	switch period {
	case "today":
		dateStr = "CURDATE()"
	case "7d":
		dateStr = "DATE_SUB(CURDATE(), INTERVAL 7 DAY)"
	case "30d":
		dateStr = "DATE_SUB(CURDATE(), INTERVAL 30 DAY)"
	default:
		dateStr = "DATE_SUB(CURDATE(), INTERVAL 30 DAY)"
	}

	query := fmt.Sprintf(
		`SELECT 
		 	COALESCE(COUNT(r.id), 0) as total_requests,
		 	COALESCE(SUM(r.total_tokens), 0) as total_tokens,
		 	COALESCE(SUM(r.cost_usd), 0) as total_cost_usd,
		 	COALESCE(AVG(r.latency_ms), 0) as avg_latency_ms
		 FROM requests r
		 JOIN user_group_members m ON r.user_id = m.user_id
		 WHERE m.group_id = ? AND DATE(r.created_at) >= %s`,
		dateStr,
	)

	var stats struct {
		TotalRequests  int
		TotalTokens    int64
		TotalCostUSD   float64
		AvgLatencyMs   int
	}

	err = h.db.QueryRow(query, groupID).Scan(
		&stats.TotalRequests, &stats.TotalTokens, &stats.TotalCostUSD, &stats.AvgLatencyMs,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get analytics: %v", err), http.StatusInternalServerError)
		return
	}

	// Get member count
	var memberCount int
	h.db.QueryRow(`SELECT COUNT(*) FROM user_group_members WHERE group_id = ?`, groupID).Scan(&memberCount)

	// Get top model
	var topModel string
	h.db.QueryRow(
		fmt.Sprintf(
			`SELECT COALESCE(r.model, 'N/A') FROM requests r
			 WHERE r.user_group_id = ? AND DATE(r.created_at) >= %s
			 GROUP BY r.model ORDER BY COUNT(*) DESC LIMIT 1`,
			dateStr,
		),
		groupID,
	).Scan(&topModel)

	// Get top provider
	var topProvider string
	h.db.QueryRow(
		fmt.Sprintf(
			`SELECT COALESCE(p.name, 'N/A') FROM requests r
			 LEFT JOIN providers p ON r.provider_id = p.id
			 WHERE r.user_group_id = ? AND DATE(r.created_at) >= %s
			 GROUP BY r.provider_id ORDER BY COUNT(*) DESC LIMIT 1`,
			dateStr,
		),
		groupID,
	).Scan(&topProvider)

	analytics := GroupAnalytics{
		GroupID:      groupID,
		GroupName:    groupName,
		TotalRequests: stats.TotalRequests,
		TotalTokens:  stats.TotalTokens,
		TotalCostUSD: stats.TotalCostUSD,
		AvgLatencyMs: stats.AvgLatencyMs,
		MemberCount:  memberCount,
		TopModel:     topModel,
		TopProvider:  topProvider,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

// GetGroupBreakdown retrieves cost breakdown by member within a group
func (h *Handler) GetGroupBreakdown(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	groupID := r.PathValue("groupID")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	// Verify group ownership
	var ownerID string
	err := h.db.QueryRow(`SELECT owner_id FROM user_groups WHERE id = ?`, groupID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}
	if err != nil || ownerID != userID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	// Calculate date range
	var dateStr string
	switch period {
	case "today":
		dateStr = "CURDATE()"
	case "7d":
		dateStr = "DATE_SUB(CURDATE(), INTERVAL 7 DAY)"
	case "30d":
		dateStr = "DATE_SUB(CURDATE(), INTERVAL 30 DAY)"
	default:
		dateStr = "DATE_SUB(CURDATE(), INTERVAL 30 DAY)"
	}

	query := fmt.Sprintf(
		`SELECT
		 	r.user_id,
		 	u.email,
		 	COALESCE(COUNT(r.id), 0) as total_requests,
		 	COALESCE(SUM(r.total_tokens), 0) as total_tokens,
		 	COALESCE(SUM(r.cost_usd), 0) as total_cost_usd
		 FROM requests r
		 JOIN users u ON r.user_id = u.id
		 JOIN user_group_members m ON r.user_id = m.user_id
		 WHERE m.group_id = ? AND DATE(r.created_at) >= %s
		 GROUP BY r.user_id, u.email
		 ORDER BY total_cost_usd DESC`,
		dateStr,
	)

	rows, err := h.db.Query(query, groupID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get breakdown: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type MemberBreakdown struct {
		UserID          string  `json:"user_id"`
		Email           string  `json:"email"`
		TotalRequests   int     `json:"total_requests"`
		TotalTokens     int64   `json:"total_tokens"`
		TotalCostUSD    float64 `json:"total_cost_usd"`
	}

	breakdown := []MemberBreakdown{}
	for rows.Next() {
		var m MemberBreakdown
		if err := rows.Scan(&m.UserID, &m.Email, &m.TotalRequests, &m.TotalTokens, &m.TotalCostUSD); err != nil {
			http.Error(w, fmt.Sprintf("Failed to scan member: %v", err), http.StatusInternalServerError)
			return
		}
		breakdown = append(breakdown, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(breakdown)
}

// Helper function to check for unique constraint errors
func isUniqueConstraintError(err error) bool {
	return err != nil && (err.Error() == "Error 1062: Duplicate entry" || 
		err.Error()[0:6] == "Error " && err.Error()[10:14] == "1062")
}
