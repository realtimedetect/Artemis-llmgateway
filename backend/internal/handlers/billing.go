package handlers

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"llm-gatway/internal/middleware"
	"llm-gatway/internal/models"
)

const basicMonthlyTokenLimit int64 = 5_000_000

type monthlyPlanUsage struct {
	PlanID            string
	PlanName          string
	MonthlyTokenLimit sql.NullInt64
	UsedTokens        int64
}

type adminUserRow struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	PlanID    string `json:"plan_id"`
	CreatedAt string `json:"created_at"`
}

func (h *Handler) getMonthlyPlanUsage(ctx context.Context, userID string) (*monthlyPlanUsage, error) {
	usage := &monthlyPlanUsage{}
	err := h.db.QueryRowContext(ctx,
		`SELECT
		    COALESCE(NULLIF(TRIM(u.plan_id),''),'basic') AS plan_id,
		    COALESCE(p.name, 'Basic') AS plan_name,
		    p.monthly_token_limit,
		    COALESCE((
		        SELECT SUM(r.total_tokens)
		        FROM requests r
		        WHERE r.user_id = u.id
		          AND r.created_at >= DATE_FORMAT(UTC_TIMESTAMP(), '%Y-%m-01 00:00:00')
		    ), 0) AS used_tokens
		 FROM users u
		 LEFT JOIN plans p ON p.id = COALESCE(NULLIF(TRIM(u.plan_id),''),'basic')
		 WHERE u.id = ?`,
		userID,
	).Scan(&usage.PlanID, &usage.PlanName, &usage.MonthlyTokenLimit, &usage.UsedTokens)
	if err == sql.ErrNoRows {
		usage.PlanID = "basic"
		usage.PlanName = "Basic"
		usage.MonthlyTokenLimit = sql.NullInt64{Int64: basicMonthlyTokenLimit, Valid: true}
		usage.UsedTokens = 0
		return usage, nil
	}
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(usage.PlanID) == "" {
		usage.PlanID = "basic"
	}
	if strings.EqualFold(usage.PlanID, "basic") && !usage.MonthlyTokenLimit.Valid {
		usage.MonthlyTokenLimit = sql.NullInt64{Int64: basicMonthlyTokenLimit, Valid: true}
	}
	if strings.TrimSpace(usage.PlanName) == "" {
		usage.PlanName = displayPlanName(usage.PlanID)
	}
	return usage, nil
}

func displayPlanName(planID string) string {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return "Basic"
	}
	return strings.ToUpper(planID[:1]) + strings.ToLower(planID[1:])
}

func (h *Handler) enforceMonthlyTokenQuota(ctx context.Context, userID string) (*monthlyPlanUsage, bool, error) {
	usage, err := h.getMonthlyPlanUsage(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	if !usage.MonthlyTokenLimit.Valid || usage.MonthlyTokenLimit.Int64 <= 0 {
		return usage, false, nil
	}
	return usage, usage.UsedTokens >= usage.MonthlyTokenLimit.Int64, nil
}

// ListPlans returns available license plans.
func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name, monthly_token_limit, description FROM plans ORDER BY id ASC`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	plans := make([]models.Plan, 0)
	for rows.Next() {
		var p models.Plan
		var limit sql.NullInt64
		if scanErr := rows.Scan(&p.ID, &p.Name, &limit, &p.Description); scanErr != nil {
			continue
		}
		if limit.Valid {
			v := limit.Int64
			p.MonthlyTokenLimit = &v
		}
		plans = append(plans, p)
	}
	writeJSON(w, http.StatusOK, plans)
}

// ListUsersForAdmin returns basic user records for admin operations.
func (h *Handler) ListUsersForAdmin(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, email, role, COALESCE(NULLIF(TRIM(plan_id),''),'basic') AS plan_id, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s')
		 FROM users
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	users := make([]adminUserRow, 0)
	for rows.Next() {
		var u adminUserRow
		if scanErr := rows.Scan(&u.ID, &u.Email, &u.Role, &u.PlanID, &u.CreatedAt); scanErr != nil {
			continue
		}
		if strings.TrimSpace(u.PlanID) == "" {
			u.PlanID = "basic"
		}
		users = append(users, u)
	}

	writeJSON(w, http.StatusOK, users)
}

// GetAdminLicenseStatus returns current admin plan usage for this month.
func (h *Handler) GetAdminLicenseStatus(w http.ResponseWriter, r *http.Request) {
	adminUserID, _ := r.Context().Value(middleware.UserIDKey).(string)
	if strings.TrimSpace(adminUserID) == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	usage, err := h.getMonthlyPlanUsage(r.Context(), adminUserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	resetAt := time.Now().UTC().AddDate(0, 1, 0)
	resetAt = time.Date(resetAt.Year(), resetAt.Month(), 1, 0, 0, 0, 0, time.UTC)

	resp := map[string]any{
		"user_id":             adminUserID,
		"plan_id":             usage.PlanID,
		"monthly_used_tokens": usage.UsedTokens,
		"next_reset_at":       resetAt.Format(time.RFC3339),
	}
	if usage.MonthlyTokenLimit.Valid && usage.MonthlyTokenLimit.Int64 > 0 {
		remaining := usage.MonthlyTokenLimit.Int64 - usage.UsedTokens
		if remaining < 0 {
			remaining = 0
		}
		resp["monthly_token_limit"] = usage.MonthlyTokenLimit.Int64
		resp["remaining_tokens"] = remaining
	} else {
		resp["monthly_token_limit"] = nil
		resp["remaining_tokens"] = nil
	}

	writeJSON(w, http.StatusOK, resp)
}

// ActivateProfessionalLicense upgrades the current admin user to professional
// after validating the provided license key against PROFESSIONAL_LICENSE_KEY.
func (h *Handler) ActivateProfessionalLicense(w http.ResponseWriter, r *http.Request) {
	adminUserID, _ := r.Context().Value(middleware.UserIDKey).(string)
	if strings.TrimSpace(adminUserID) == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var body struct {
		LicenseKey string `json:"license_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	provided := strings.TrimSpace(body.LicenseKey)
	if provided == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "license_key is required"})
		return
	}

	expected := strings.TrimSpace(os.Getenv("PROFESSIONAL_LICENSE_KEY"))
	if expected == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "professional license key is not configured on server"})
		return
	}

	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid license key"})
		return
	}

	if _, err := h.db.ExecContext(r.Context(), `UPDATE users SET plan_id = 'professional' WHERE id = ?`, adminUserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": adminUserID,
		"plan_id": "professional",
		"status":  "activated",
	})
}

// UpdateUserPlan lets an admin assign basic/professional plan to a user.
func (h *Handler) UpdateUserPlan(w http.ResponseWriter, r *http.Request) {
	actorID, _ := r.Context().Value(middleware.UserIDKey).(string)
	userID := strings.TrimSpace(chi.URLParam(r, "id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user id is required"})
		return
	}

	var body struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.PlanID = strings.ToLower(strings.TrimSpace(body.PlanID))
	if body.PlanID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "plan_id is required"})
		return
	}

	var planExists int
	if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM plans WHERE id = ?`, body.PlanID).Scan(&planExists); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if planExists == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plan_id"})
		return
	}

	if body.PlanID == "professional" {
		licensed, err := h.isUserProfessional(r.Context(), actorID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
			return
		}
		if !licensed {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "activate professional license key first"})
			return
		}
	}

	res, err := h.db.ExecContext(r.Context(), `UPDATE users SET plan_id = ? WHERE id = ?`, body.PlanID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	usage, _ := h.getMonthlyPlanUsage(r.Context(), userID)
	resetAt := time.Now().UTC().AddDate(0, 1, 0)
	resetAt = time.Date(resetAt.Year(), resetAt.Month(), 1, 0, 0, 0, 0, time.UTC)

	response := map[string]any{
		"user_id":             userID,
		"plan_id":             body.PlanID,
		"monthly_used_tokens": int64(0),
		"next_reset_at":       resetAt.Format(time.RFC3339),
	}
	if usage != nil {
		response["monthly_used_tokens"] = usage.UsedTokens
		if usage.MonthlyTokenLimit.Valid {
			response["monthly_token_limit"] = usage.MonthlyTokenLimit.Int64
		} else {
			response["monthly_token_limit"] = nil
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) isUserProfessional(ctx context.Context, userID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return false, nil
	}
	var planID string
	err := h.db.QueryRowContext(ctx, `SELECT COALESCE(NULLIF(TRIM(plan_id),''), 'basic') FROM users WHERE id = ?`, userID).Scan(&planID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(planID), "professional"), nil
}
