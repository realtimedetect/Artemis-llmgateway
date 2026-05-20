package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"llm-gatway/internal/middleware"
)

type agentRunRequest struct {
	Model        string `json:"model"`
	Input        string `json:"input"`
	Instructions string `json:"instructions,omitempty"`
	MaxSteps     int    `json:"max_steps,omitempty"`
	UseTools     bool   `json:"use_tools,omitempty"`
}

type agentPlan struct {
	Steps []agentPlanStep `json:"steps"`
}

type agentPlanStep struct {
	Title     string `json:"title"`
	Objective string `json:"objective"`
}

type agentExecutionStep struct {
	Index     int              `json:"index"`
	Title     string           `json:"title"`
	Objective string           `json:"objective"`
	Output    string           `json:"output"`
	ToolCalls []agentToolEvent `json:"tool_calls,omitempty"`
}

type agentToolEvent struct {
	Tool   string          `json:"tool"`
	Args   json.RawMessage `json:"args,omitempty"`
	Result string          `json:"result"`
}

// ── gateway tool definitions (OpenAI function-calling schema) ────────────────

// gatewayToolDefinitions is the static set of tools the agent may call.
// All tools query internal gateway data; none expose API keys or secrets.
var gatewayToolDefinitions = []map[string]any{
	{
		"type": "function",
		"function": map[string]any{
			"name":        "query_usage_analytics",
			"description": "Returns the current user's monthly token usage statistics: used tokens, remaining tokens, plan name, and monthly limit.",
			"parameters": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]any{
			"name":        "list_routes",
			"description": "Returns all LLM routes configured in the gateway for the current user: name, slug, model, provider name, and enabled status.",
			"parameters": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]any{
			"name":        "list_providers",
			"description": "Returns all LLM providers configured in the gateway: name, adapter type, and enabled status. API keys are never included.",
			"parameters": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
	},
}

// ── internal message types for the tool loop ─────────────────────────────────

type agentChatMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  []agentToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type agentToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function agentToolFunction `json:"function"`
}

type agentToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ── gateway tool dispatch ─────────────────────────────────────────────────────

// dispatchGatewayTool executes a named gateway tool and returns a JSON string
// result safe to include in the conversation history.
// API keys and secrets are never included in any tool result.
func (h *Handler) dispatchGatewayTool(ctx context.Context, userID, toolName string, _ json.RawMessage) (string, error) {
	switch toolName {

	case "query_usage_analytics":
		usage, err := h.getMonthlyPlanUsage(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("usage query failed: %w", err)
		}
		result := map[string]any{
			"plan_id":     usage.PlanID,
			"plan_name":   usage.PlanName,
			"used_tokens": usage.UsedTokens,
		}
		if usage.MonthlyTokenLimit.Valid {
			result["monthly_limit"] = usage.MonthlyTokenLimit.Int64
			remaining := usage.MonthlyTokenLimit.Int64 - usage.UsedTokens
			if remaining < 0 {
				remaining = 0
			}
			result["remaining_tokens"] = remaining
		} else {
			result["monthly_limit"] = nil
			result["remaining_tokens"] = "unlimited"
		}
		b, _ := json.Marshal(result)
		return string(b), nil

	case "list_routes":
		rows, err := h.db.QueryContext(ctx,
			`SELECT rt.name, rt.slug, rt.model, COALESCE(p.name,'') AS provider_name, rt.enabled
			 FROM llm_routes rt
			 LEFT JOIN providers p ON p.id = rt.provider_id
			 WHERE rt.user_id = ?
			 ORDER BY rt.created_at DESC`, userID,
		)
		if err != nil {
			return "", fmt.Errorf("routes query failed: %w", err)
		}
		defer rows.Close()
		type routeRow struct {
			Name     string `json:"name"`
			Slug     string `json:"slug"`
			Model    string `json:"model"`
			Provider string `json:"provider"`
			Enabled  bool   `json:"enabled"`
		}
		result := make([]routeRow, 0)
		for rows.Next() {
			var rr routeRow
			if scanErr := rows.Scan(&rr.Name, &rr.Slug, &rr.Model, &rr.Provider, &rr.Enabled); scanErr != nil {
				continue
			}
			result = append(result, rr)
		}
		b, _ := json.Marshal(result)
		return string(b), nil

	case "list_providers":
		rows, err := h.db.QueryContext(ctx,
			`SELECT name, adapter, enabled FROM providers ORDER BY name ASC`,
		)
		if err != nil {
			return "", fmt.Errorf("providers query failed: %w", err)
		}
		defer rows.Close()
		type provRow struct {
			Name    string `json:"name"`
			Adapter string `json:"adapter"`
			Enabled bool   `json:"enabled"`
		}
		result := make([]provRow, 0)
		for rows.Next() {
			var pr provRow
			if scanErr := rows.Scan(&pr.Name, &pr.Adapter, &pr.Enabled); scanErr != nil {
				continue
			}
			result = append(result, pr)
		}
		b, _ := json.Marshal(result)
		return string(b), nil

	default:
		return "", fmt.Errorf("unknown gateway tool: %s", toolName)
	}
}

// ── tool-enabled step executor ────────────────────────────────────────────────

// executeStepWithTools runs a single agent step. It sends the step prompt to
// the model along with gateway tool definitions, dispatches any tool calls
// the model makes (up to maxToolRounds), feeds results back, and returns the
// final content output together with a log of tool events.
func (h *Handler) executeStepWithTools(r *http.Request, userID, model, systemPrompt, stepPrompt string) (string, []agentToolEvent, error) {
	messages := []agentChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: stepPrompt},
	}
	toolEvents := make([]agentToolEvent, 0)
	const maxToolRounds = 5

	for round := 0; round < maxToolRounds; round++ {
		payload := map[string]any{
			"model":       model,
			"stream":      false,
			"messages":    messages,
			"tools":       gatewayToolDefinitions,
			"tool_choice": "auto",
		}
		body, _ := json.Marshal(payload)

		proxyReq := httptest.NewRequest(http.MethodPost, "/api/chat/completions", bytes.NewReader(body))
		proxyReq = proxyReq.WithContext(r.Context())
		proxyReq.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		h.ChatCompletions(rec, proxyReq)
		resp := rec.Result()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return "", toolEvents, fmt.Errorf("model execution failed (status %d)", resp.StatusCode)
		}

		var chatResp struct {
			Choices []struct {
				Message      agentChatMessage `json:"message"`
				FinishReason string           `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
			resp.Body.Close()
			return "", toolEvents, fmt.Errorf("failed to parse model response")
		}
		resp.Body.Close()

		if len(chatResp.Choices) == 0 {
			return "", toolEvents, fmt.Errorf("empty model response")
		}

		msg := chatResp.Choices[0].Message

		// No tool calls → content is the final step output.
		if len(msg.ToolCalls) == 0 {
			return strings.TrimSpace(msg.Content), toolEvents, nil
		}

		// Append assistant message (with tool calls) to conversation history.
		messages = append(messages, msg)

		// Dispatch each tool call and feed results back.
		for _, tc := range msg.ToolCalls {
			var args json.RawMessage
			if tc.Function.Arguments != "" {
				args = json.RawMessage(tc.Function.Arguments)
			}
			result, dispErr := h.dispatchGatewayTool(r.Context(), userID, tc.Function.Name, args)
			if dispErr != nil {
				result = fmt.Sprintf(`{"error":%q}`, dispErr.Error())
			}
			toolEvents = append(toolEvents, agentToolEvent{
				Tool:   tc.Function.Name,
				Args:   args,
				Result: result,
			})
			messages = append(messages, agentChatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    result,
			})
		}
	}
	return "", toolEvents, fmt.Errorf("agent tool loop exceeded maximum rounds (%d)", maxToolRounds)
}

// AgentRun executes an agentic workflow (plan → execute steps → synthesize)
// entirely through the gateway's existing chat/completions path.
//
// When use_tools is true, each execution step may call gateway-native tools
// (query_usage_analytics, list_routes, list_providers) before producing output.
// Tool results are injected back into the conversation automatically.
func (h *Handler) AgentRun(w http.ResponseWriter, r *http.Request) {
	var req agentRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	req.Input = strings.TrimSpace(req.Input)
	req.Instructions = strings.TrimSpace(req.Instructions)
	if req.Model == "" || req.Input == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model and input are required"})
		return
	}
	if req.MaxSteps <= 0 {
		req.MaxSteps = 3
	}
	if req.MaxSteps > 8 {
		req.MaxSteps = 8
	}

	userID, _ := r.Context().Value(middleware.UserIDKey).(string)

	baseSystem := "You are an autonomous assistant. Keep responses concise, accurate, and actionable."
	if req.Instructions != "" {
		baseSystem = baseSystem + " Additional instructions: " + req.Instructions
	}

	// ── Plan ──────────────────────────────────────────────────────────────────
	plannerSystem := baseSystem + " Create an execution plan in strict JSON only with this schema: {\"steps\":[{\"title\":\"...\",\"objective\":\"...\"}]} and at most " + intToString(req.MaxSteps) + " steps."
	planContent, status, err := h.invokeInternalChat(r, req.Model, plannerSystem, req.Input)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	plan := parseAgentPlan(planContent)
	if len(plan.Steps) == 0 {
		plan.Steps = []agentPlanStep{{Title: "Solve request", Objective: req.Input}}
	}
	if len(plan.Steps) > req.MaxSteps {
		plan.Steps = plan.Steps[:req.MaxSteps]
	}

	// ── Execute steps ──────────────────────────────────────────────────────────
	executed := make([]agentExecutionStep, 0, len(plan.Steps))
	contextBlock := ""
	for i, step := range plan.Steps {
		stepTitle := strings.TrimSpace(step.Title)
		if stepTitle == "" {
			stepTitle = "Step " + intToString(i+1)
		}
		stepObjective := strings.TrimSpace(step.Objective)
		if stepObjective == "" {
			stepObjective = req.Input
		}

		stepPrompt := "Original task: " + req.Input + "\n\nCurrent step: " + stepTitle + "\nObjective: " + stepObjective
		if contextBlock != "" {
			stepPrompt += "\n\nPrevious findings:\n" + contextBlock
		}
		stepSystem := baseSystem + " Execute ONLY the current step and return practical output for this step."

		var stepOutput string
		var toolEvents []agentToolEvent

		if req.UseTools && strings.TrimSpace(userID) != "" {
			stepOutput, toolEvents, err = h.executeStepWithTools(r, userID, req.Model, stepSystem, stepPrompt)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		} else {
			var stepStatus int
			stepOutput, stepStatus, err = h.invokeInternalChat(r, req.Model, stepSystem, stepPrompt)
			if err != nil {
				writeJSON(w, stepStatus, map[string]string{"error": err.Error()})
				return
			}
		}

		executed = append(executed, agentExecutionStep{
			Index:     i + 1,
			Title:     stepTitle,
			Objective: stepObjective,
			Output:    stepOutput,
			ToolCalls: toolEvents,
		})
		contextBlock += "- " + stepTitle + ": " + stepOutput + "\n"
	}

	// ── Synthesize ────────────────────────────────────────────────────────────
	summarySystem := baseSystem + " Synthesize the step outputs into one final answer with clear recommendations."
	summaryPrompt := "Original task: " + req.Input + "\n\nCompleted steps:\n" + contextBlock
	finalAnswer, finalStatus, finalErr := h.invokeInternalChat(r, req.Model, summarySystem, summaryPrompt)
	if finalErr != nil {
		writeJSON(w, finalStatus, map[string]string{"error": finalErr.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"model":        req.Model,
		"input":        req.Input,
		"use_tools":    req.UseTools,
		"total_steps":  len(executed),
		"steps":        executed,
		"final_answer": finalAnswer,
	})
}

func (h *Handler) invokeInternalChat(r *http.Request, model, systemPrompt, userPrompt string) (string, int, error) {
	payload := map[string]any{
		"model":  model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}
	body, _ := json.Marshal(payload)

	proxyReq := httptest.NewRequest(http.MethodPost, "/api/chat/completions", bytes.NewReader(body))
	proxyReq = proxyReq.WithContext(r.Context())
	proxyReq.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, proxyReq)
	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", resp.StatusCode, fmt.Errorf("agent model execution failed")
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", http.StatusBadGateway, fmt.Errorf("failed to parse model response")
	}
	if len(chatResp.Choices) == 0 {
		return "", http.StatusBadGateway, fmt.Errorf("empty model response")
	}
	return strings.TrimSpace(chatResp.Choices[0].Message.Content), http.StatusOK, nil
}

func parseAgentPlan(raw string) agentPlan {
	var p agentPlan
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return p
	}
	if err := json.Unmarshal([]byte(raw), &p); err == nil {
		return p
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		_ = json.Unmarshal([]byte(raw[start:end+1]), &p)
	}
	return p
}

func intToString(v int) string {
	return strconv.Itoa(v)
}
