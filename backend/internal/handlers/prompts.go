package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"llm-gatway/internal/middleware"
	"llm-gatway/internal/models"
)

func (h *Handler) ListPromptTemplates(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, user_id, name, description, COALESCE(active_version_id,''), created_at, updated_at
		 FROM prompt_templates
		 WHERE user_id = ?
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	items := make([]models.PromptTemplate, 0)
	for rows.Next() {
		var t models.PromptTemplate
		if scanErr := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Description, &t.ActiveVersionID, &t.CreatedAt, &t.UpdatedAt); scanErr != nil {
			continue
		}
		items = append(items, t)
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) CreatePromptTemplate(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Content     string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	body.Description = strings.TrimSpace(body.Description)
	body.Content = strings.TrimSpace(body.Content)
	if body.Name == "" || body.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and content are required"})
		return
	}

	templateID := uuid.NewString()
	versionID := uuid.NewString()
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO prompt_templates (id, user_id, name, description, active_version_id)
		 VALUES (?, ?, ?, ?, ?)`,
		templateID, userID, body.Name, body.Description, versionID,
	)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "template name already exists"})
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO prompt_versions (id, template_id, user_id, version, content, activated_at)
		 VALUES (?, ?, ?, 1, ?, ?)`,
		versionID, templateID, userID, body.Content, time.Now().UTC(),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": templateID, "active_version_id": versionID})
}

func (h *Handler) ListPromptVersions(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	templateID := chi.URLParam(r, "id")

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT pv.id, pv.template_id, pv.user_id, pv.version, pv.content,
		        COALESCE(pv.test_input,''), COALESCE(pv.test_output,''), pv.test_status,
		        pv.created_at, COALESCE(pv.activated_at, '1970-01-01 00:00:00'),
		        CASE WHEN pt.active_version_id = pv.id THEN 1 ELSE 0 END AS is_active
		 FROM prompt_versions pv
		 JOIN prompt_templates pt ON pt.id = pv.template_id
		 WHERE pv.user_id = ? AND pv.template_id = ?
		 ORDER BY pv.version DESC`,
		userID, templateID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	items := make([]models.PromptVersion, 0)
	for rows.Next() {
		var v models.PromptVersion
		var activeInt int
		if scanErr := rows.Scan(&v.ID, &v.TemplateID, &v.UserID, &v.Version, &v.Content, &v.TestInput, &v.TestOutput, &v.TestStatus, &v.CreatedAt, &v.ActivatedAt, &activeInt); scanErr != nil {
			continue
		}
		v.IsActive = activeInt == 1
		items = append(items, v)
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) CreatePromptVersion(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	templateID := chi.URLParam(r, "id")
	var body struct {
		Content   string `json:"content"`
		SetActive bool   `json:"set_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.Content = strings.TrimSpace(body.Content)
	if body.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}

	var currentVersion int
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(MAX(version),0) FROM prompt_versions WHERE user_id = ? AND template_id = ?`,
		userID, templateID,
	).Scan(&currentVersion); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	newID := uuid.NewString()
	nextVersion := currentVersion + 1
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO prompt_versions (id, template_id, user_id, version, content)
		 VALUES (?, ?, ?, ?, ?)`,
		newID, templateID, userID, nextVersion, body.Content,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	if body.SetActive {
		_, _ = h.db.ExecContext(r.Context(),
			`UPDATE prompt_templates SET active_version_id = ? WHERE id = ? AND user_id = ?`,
			newID, templateID, userID,
		)
		_, _ = h.db.ExecContext(r.Context(),
			`UPDATE prompt_versions SET activated_at = ? WHERE id = ?`,
			time.Now().UTC(), newID,
		)
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": newID, "version": nextVersion})
}

func (h *Handler) ActivatePromptVersion(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	templateID := chi.URLParam(r, "id")
	var body struct {
		VersionID string `json:"version_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.VersionID = strings.TrimSpace(body.VersionID)
	if body.VersionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "version_id is required"})
		return
	}

	var exists int
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT 1 FROM prompt_versions WHERE id = ? AND template_id = ? AND user_id = ? LIMIT 1`,
		body.VersionID, templateID, userID,
	).Scan(&exists); err != nil || exists != 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "version not found"})
		return
	}

	_, _ = h.db.ExecContext(r.Context(),
		`UPDATE prompt_templates SET active_version_id = ? WHERE id = ? AND user_id = ?`,
		body.VersionID, templateID, userID,
	)
	_, _ = h.db.ExecContext(r.Context(),
		`UPDATE prompt_versions SET activated_at = ? WHERE id = ?`,
		time.Now().UTC(), body.VersionID,
	)
	writeJSON(w, http.StatusOK, map[string]string{"template_id": templateID, "active_version_id": body.VersionID})
}

func (h *Handler) TestPromptVersion(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	requestID := chiMiddleware.GetReqID(r.Context())
	var body struct {
		TemplateID string `json:"template_id"`
		VersionID  string `json:"version_id"`
		Input      string `json:"input"`
		Model      string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.TemplateID = strings.TrimSpace(body.TemplateID)
	body.VersionID = strings.TrimSpace(body.VersionID)
	body.Input = strings.TrimSpace(body.Input)
	body.Model = strings.TrimSpace(body.Model)
	if body.TemplateID == "" || body.Input == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "template_id and input are required"})
		return
	}

	content := ""
	if body.VersionID != "" {
		if raw, ok := h.getPromptVersionContent(r.Context(), userID, body.VersionID); ok {
			content = raw
		}
	}
	if content == "" {
		var activeVersionID string
		if err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(active_version_id,'') FROM prompt_templates WHERE id = ? AND user_id = ?`, body.TemplateID, userID).Scan(&activeVersionID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "template not found"})
			return
		}
		if raw, ok := h.getPromptVersionContent(r.Context(), userID, activeVersionID); ok {
			content = raw
			body.VersionID = activeVersionID
		}
	}
	if strings.TrimSpace(content) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt content not found"})
		return
	}

	if body.Model == "" {
		body.Model = "prompt-test"
	}

	payload := map[string]any{
		"model": body.Model,
		"messages": []map[string]string{
			{"role": "system", "content": content},
			{"role": "user", "content": body.Input},
		},
		"stream": false,
	}
	bodyBytes, _ := json.Marshal(payload)

	candidates, err := h.listCandidateProviders(r.Context(), false, "", "", "")
	if err != nil || len(candidates) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no providers configured"})
		return
	}

	providerPtr, resp, latency, err := h.executeProviderRequest(r.Context(), userID, nil, requestID, "prompt-test", body.Model, candidates, "/chat/completions", bodyBytes, 90*time.Second, false)
	if err != nil || providerPtr == nil || resp == nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "provider unreachable"})
		return
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	testStatus := resp.StatusCode
	testOutput := string(respBytes)
	_, _ = h.db.ExecContext(r.Context(),
		`UPDATE prompt_versions SET test_input = ?, test_output = ?, test_status = ? WHERE id = ? AND user_id = ?`,
		body.Input, truncateAuditPayload(respBytes, 65536), testStatus, body.VersionID, userID,
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"provider_id": providerPtr.ID,
		"latency_ms":  latency,
		"status":      testStatus,
		"output":      testOutput,
	})
}

func (h *Handler) getPromptVersionContent(ctx context.Context, userID, versionID string) (string, bool) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return "", false
	}
	var content string
	err := h.db.QueryRowContext(ctx,
		`SELECT content FROM prompt_versions WHERE id = ? AND user_id = ?`,
		versionID, userID,
	).Scan(&content)
	if err != nil {
		return "", false
	}
	return content, true
}
