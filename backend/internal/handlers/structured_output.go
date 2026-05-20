package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
)

func validateStructuredOutput(responseBody []byte, schemaJSON string) error {
	content := extractAssistantContent(responseBody)
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("assistant content is empty")
	}

	var payload any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return fmt.Errorf("assistant content is not valid JSON")
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return fmt.Errorf("configured schema is invalid JSON")
	}

	return validateJSONSchemaSimple(payload, schema)
}

func extractAssistantContent(responseBody []byte) string {
	var normalized struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &normalized); err != nil {
		return ""
	}
	if len(normalized.Choices) == 0 {
		return ""
	}
	return strings.TrimSpace(normalized.Choices[0].Message.Content)
}

func validateJSONSchemaSimple(payload any, schema map[string]any) error {
	typeName := strings.TrimSpace(stringFromAny(schema["type"]))
	if typeName == "" {
		typeName = "object"
	}
	if typeName != "object" {
		return fmt.Errorf("only object schema type is supported")
	}

	obj, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("response JSON must be an object")
	}

	required := map[string]bool{}
	if req, ok := schema["required"].([]any); ok {
		for _, item := range req {
			name := strings.TrimSpace(stringFromAny(item))
			if name != "" {
				required[name] = true
			}
		}
	}
	for key := range required {
		if _, exists := obj[key]; !exists {
			return fmt.Errorf("missing required field %q", key)
		}
	}

	props, _ := schema["properties"].(map[string]any)
	for key, rawRule := range props {
		value, exists := obj[key]
		if !exists {
			continue
		}
		rule, _ := rawRule.(map[string]any)
		expectedType := strings.TrimSpace(stringFromAny(rule["type"]))
		if expectedType == "" {
			continue
		}
		if !matchesExpectedType(value, expectedType) {
			return fmt.Errorf("field %q must be %s", key, expectedType)
		}
	}

	return nil
}

func matchesExpectedType(value any, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		switch value.(type) {
		case float64, float32, int, int32, int64, json.Number:
			return true
		default:
			return false
		}
	case "integer":
		switch value.(type) {
		case int, int32, int64:
			return true
		case float64:
			f := value.(float64)
			return float64(int64(f)) == f
		default:
			return false
		}
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		return true
	}
}
