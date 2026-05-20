package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"llm-gatway/internal/models"
)

var errProviderUnsupported = fmt.Errorf("provider endpoint unsupported")

func normalizeProviderAdapter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "openai", "openai-compatible", "openai_compatible":
		return "openai"
	case "anthropic":
		return "anthropic"
	default:
		return ""
	}
}

func providerAdapterOrDefault(provider models.Provider) string {
	adapter := normalizeProviderAdapter(provider.Adapter)
	if adapter == "" {
		return "openai"
	}
	return adapter
}

func providerSupportsStreaming(provider models.Provider) bool {
	switch providerAdapterOrDefault(provider) {
	case "openai", "anthropic":
		return true
	default:
		return false
	}
}

func buildProviderRequest(provider models.Provider, endpoint string, bodyBytes []byte) (string, http.Header, []byte, error) {
	switch providerAdapterOrDefault(provider) {
	case "openai":
		return buildOpenAIProviderRequest(provider, endpoint, bodyBytes)
	case "anthropic":
		return buildAnthropicProviderRequest(provider, endpoint, bodyBytes)
	default:
		return "", nil, nil, fmt.Errorf("unsupported provider adapter %q", provider.Adapter)
	}
}

func normalizeProviderResponse(provider models.Provider, endpoint string, statusCode int, body []byte) ([]byte, error) {
	switch providerAdapterOrDefault(provider) {
	case "openai":
		return body, nil
	case "anthropic":
		if endpoint != "/chat/completions" || statusCode < 200 || statusCode >= 300 {
			return body, nil
		}
		return normalizeAnthropicChatResponse(body)
	default:
		return body, nil
	}
}

func buildOpenAIProviderRequest(provider models.Provider, endpoint string, bodyBytes []byte) (string, http.Header, []byte, error) {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", "Bearer "+provider.APIKey)
	return strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/") + endpoint, headers, bodyBytes, nil
}

func buildAnthropicProviderRequest(provider models.Provider, endpoint string, bodyBytes []byte) (string, http.Header, []byte, error) {
	if endpoint != "/chat/completions" {
		return "", nil, nil, errProviderUnsupported
	}

	convertedBody, err := convertUnifiedChatToAnthropic(bodyBytes)
	if err != nil {
		return "", nil, nil, err
	}

	version := strings.TrimSpace(provider.APIVersion)
	if version == "" {
		version = "2023-06-01"
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("X-API-Key", provider.APIKey)
	headers.Set("Anthropic-Version", version)
	return strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/") + "/messages", headers, convertedBody, nil
}

func convertUnifiedChatToAnthropic(bodyBytes []byte) ([]byte, error) {
	var raw map[string]any
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, err
	}

	messageItems, _ := raw["messages"].([]any)
	anthropicMessages := make([]map[string]any, 0, len(messageItems))
	systemParts := make([]string, 0)

	for _, item := range messageItems {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.TrimSpace(stringFromAny(msg["role"]))
		content := stringifyUnifiedMessageContent(msg["content"])
		if content == "" {
			continue
		}
		if role == "system" {
			systemParts = append(systemParts, content)
			continue
		}
		if role != "assistant" {
			role = "user"
		}
		anthropicMessages = append(anthropicMessages, map[string]any{
			"role":    role,
			"content": content,
		})
	}

	payload := map[string]any{
		"model":    stringFromAny(raw["model"]),
		"messages": anthropicMessages,
		"stream":   boolFromAny(raw["stream"]),
	}

	if len(systemParts) > 0 {
		payload["system"] = strings.Join(systemParts, "\n\n")
	}

	maxTokens := intFromAny(raw["max_tokens"])
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	payload["max_tokens"] = maxTokens

	if temperature, ok := raw["temperature"]; ok {
		payload["temperature"] = temperature
	}
	if topP, ok := raw["top_p"]; ok {
		payload["top_p"] = topP
	}
	if stop, ok := raw["stop"]; ok {
		switch typed := stop.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				payload["stop_sequences"] = []string{typed}
			}
		case []any:
			sequences := make([]string, 0, len(typed))
			for _, item := range typed {
				if seq := strings.TrimSpace(stringFromAny(item)); seq != "" {
					sequences = append(sequences, seq)
				}
			}
			if len(sequences) > 0 {
				payload["stop_sequences"] = sequences
			}
		}
	}

	return json.Marshal(payload)
}

func normalizeAnthropicChatResponse(body []byte) ([]byte, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	usage, _ := raw["usage"].(map[string]any)
	promptTokens := intFromAny(usage["input_tokens"])
	completionTokens := intFromAny(usage["output_tokens"])
	finishReason := normalizeAnthropicStopReason(stringFromAny(raw["stop_reason"]))
	if finishReason == "" {
		finishReason = "stop"
	}

	normalized := map[string]any{
		"id":      stringFromAny(raw["id"]),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   stringFromAny(raw["model"]),
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": extractAnthropicText(raw["content"]),
				},
				"finish_reason": finishReason,
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}

	return json.Marshal(normalized)
}

func streamProviderResponse(w http.ResponseWriter, resp *http.Response, provider models.Provider) (int, int, int, int, int) {
	if providerAdapterOrDefault(provider) == "anthropic" {
		return streamAnthropicChatResponse(w, resp)
	}

	copyProviderHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(resp.Body)
	start := time.Now()
	promptTokens, completionTokens, totalTokens := 0, 0, 0
	ttftMs := 0
	firstByteObserved := false

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if !firstByteObserved {
				firstByteObserved = true
				ttftMs = int(time.Since(start).Milliseconds())
			}
			_, _ = w.Write(line)
			if flusher != nil {
				flusher.Flush()
			}

			trimmed := strings.TrimSpace(string(line))
			if strings.HasPrefix(trimmed, "data:") {
				payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if payload != "" && payload != "[DONE]" {
					var chunk struct {
						Usage struct {
							PromptTokens     int `json:"prompt_tokens"`
							CompletionTokens int `json:"completion_tokens"`
							TotalTokens      int `json:"total_tokens"`
						} `json:"usage"`
					}
					if json.Unmarshal([]byte(payload), &chunk) == nil {
						if chunk.Usage.TotalTokens > 0 || chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
							promptTokens = chunk.Usage.PromptTokens
							completionTokens = chunk.Usage.CompletionTokens
							totalTokens = chunk.Usage.TotalTokens
						}
					}
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
	}

	return promptTokens, completionTokens, totalTokens, int(time.Since(start).Milliseconds()), ttftMs
}

func streamAnthropicChatResponse(w http.ResponseWriter, resp *http.Response) (int, int, int, int, int) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		copyProviderHeaders(w, resp)
		w.WriteHeader(resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		_, _ = w.Write(body)
		return 0, 0, 0, 0, 0
	}

	copyProviderHeaders(w, resp)
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(resp.Body)
	start := time.Now()
	created := time.Now().Unix()
	promptTokens, completionTokens := 0, 0
	ttftMs := 0
	responseID := uuid.NewString()
	modelName := ""
	didSendDone := false

	emitData := func(payload any) {
		if ttftMs == 0 {
			ttftMs = int(time.Since(start).Milliseconds())
		}
		encoded, _ := json.Marshal(payload)
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(encoded)
		_, _ = w.Write([]byte("\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}
	emitDone := func() {
		if didSendDone {
			return
		}
		didSendDone = true
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}

	processBlock := func(eventName string, dataLines []string) {
		if len(dataLines) == 0 {
			return
		}
		payloadText := strings.Join(dataLines, "\n")
		var payload map[string]any
		if err := json.Unmarshal([]byte(payloadText), &payload); err != nil {
			return
		}

		switch eventName {
		case "message_start":
			message, _ := payload["message"].(map[string]any)
			if id := strings.TrimSpace(stringFromAny(message["id"])); id != "" {
				responseID = id
			}
			if model := strings.TrimSpace(stringFromAny(message["model"])); model != "" {
				modelName = model
			}
			usage, _ := message["usage"].(map[string]any)
			promptTokens = intFromAny(usage["input_tokens"])
			emitData(map[string]any{
				"id":      responseID,
				"object":  "chat.completion.chunk",
				"created": created,
				"model":   modelName,
				"choices": []map[string]any{{
					"index": 0,
					"delta": map[string]any{"role": "assistant", "content": ""},
				}},
			})
		case "content_block_delta":
			delta, _ := payload["delta"].(map[string]any)
			text := stringFromAny(delta["text"])
			if text == "" {
				return
			}
			emitData(map[string]any{
				"id":      responseID,
				"object":  "chat.completion.chunk",
				"created": created,
				"model":   modelName,
				"choices": []map[string]any{{
					"index": 0,
					"delta": map[string]any{"content": text},
				}},
			})
		case "message_delta":
			usage, _ := payload["usage"].(map[string]any)
			if tokens := intFromAny(usage["output_tokens"]); tokens > 0 {
				completionTokens = tokens
			}
			delta, _ := payload["delta"].(map[string]any)
			finishReason := normalizeAnthropicStopReason(stringFromAny(delta["stop_reason"]))
			if finishReason == "" {
				return
			}
			emitData(map[string]any{
				"id":      responseID,
				"object":  "chat.completion.chunk",
				"created": created,
				"model":   modelName,
				"choices": []map[string]any{{
					"index":         0,
					"delta":         map[string]any{},
					"finish_reason": finishReason,
				}},
			})
		case "message_stop":
			emitDone()
		}
	}

	currentEvent := ""
	dataLines := []string{}
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimRight(line, "\r\n")
			if trimmed == "" {
				processBlock(currentEvent, dataLines)
				currentEvent = ""
				dataLines = nil
			} else if strings.HasPrefix(trimmed, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
			} else if strings.HasPrefix(trimmed, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(trimmed, "data:")))
			}
		}
		if err == io.EOF {
			processBlock(currentEvent, dataLines)
			break
		}
		if err != nil {
			break
		}
	}

	emitDone()
	totalTokens := promptTokens + completionTokens
	return promptTokens, completionTokens, totalTokens, int(time.Since(start).Milliseconds()), ttftMs
}

func extractAnthropicText(content any) string {
	items, ok := content.([]any)
	if !ok {
		return strings.TrimSpace(stringFromAny(content))
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		part, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if partType := strings.TrimSpace(stringFromAny(part["type"])); partType != "" && partType != "text" {
			continue
		}
		if text := strings.TrimSpace(stringFromAny(part["text"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func stringifyUnifiedMessageContent(content any) string {
	switch typed := content.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			switch piece := item.(type) {
			case string:
				if trimmed := strings.TrimSpace(piece); trimmed != "" {
					parts = append(parts, trimmed)
				}
			case map[string]any:
				if text := strings.TrimSpace(stringFromAny(piece["text"])); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return strings.TrimSpace(stringFromAny(content))
	}
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		if typed == nil {
			return ""
		}
		return fmt.Sprint(typed)
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		var parsed json.Number = json.Number(strings.TrimSpace(typed))
		value, _ := parsed.Int64()
		return int(value)
	default:
		return 0
	}
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func normalizeAnthropicStopReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return ""
	}
}

func filterStreamingProviders(providers []models.Provider) []models.Provider {
	streamingProviders := make([]models.Provider, 0, len(providers))
	for _, provider := range providers {
		if providerSupportsStreaming(provider) {
			streamingProviders = append(streamingProviders, provider)
		}
	}
	return streamingProviders
}

func cloneHTTPResponse(resp *http.Response, body []byte) *http.Response {
	cloned := new(http.Response)
	*cloned = *resp
	cloned.Body = io.NopCloser(bytes.NewReader(body))
	cloned.ContentLength = int64(len(body))
	if cloned.Header == nil {
		cloned.Header = make(http.Header)
	}
	cloned.Header.Del("Content-Length")
	return cloned
}