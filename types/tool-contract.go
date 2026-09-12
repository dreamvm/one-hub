package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CheckedToolChoice accepts modern tool_choice and legacy function_call without
// unchecked assertions. Unsupported controls must never silently become auto.
func (r *ChatCompletionRequest) CheckedToolChoice() (mode, name string, err error) {
	value := r.ToolChoice
	legacy := false
	if value == nil {
		value = r.FunctionCall
		legacy = true
	}
	if value == nil {
		return ToolChoiceTypeAuto, "", nil
	}
	if s, ok := value.(string); ok {
		switch s {
		case ToolChoiceTypeAuto, ToolChoiceTypeNone, ToolChoiceTypeRequired:
			return s, "", nil
		}
		return "", "", fmt.Errorf("unsupported tool choice")
	}
	b, e := json.Marshal(value)
	if e != nil {
		return "", "", fmt.Errorf("invalid tool choice")
	}
	var choice struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function *struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if json.Unmarshal(b, &choice) != nil {
		return "", "", fmt.Errorf("invalid tool choice")
	}
	if legacy {
		name = choice.Name
	} else if choice.Type == "function" && choice.Function != nil {
		name = choice.Function.Name
	}
	if strings.TrimSpace(name) == "" {
		return "", "", fmt.Errorf("tool choice must name a function")
	}
	return ToolChoiceTypeFunction, name, nil
}

// CheckedFunctions is intentionally provider opt-in; unrelated relays retain
// their existing behavior and native tool types are not mistaken for functions.
func (r *ChatCompletionRequest) CheckedFunctions() ([]*ChatCompletionFunction, error) {
	if r.Tools != nil {
		for _, tool := range r.Tools {
			if tool == nil || tool.Type != "function" {
				return nil, fmt.Errorf("unsupported tool type in chat compatibility request")
			}
		}
	}
	functions := r.GetFunctions()
	names := make(map[string]bool)
	for _, f := range functions {
		if f == nil || strings.TrimSpace(f.Name) == "" {
			return nil, fmt.Errorf("tool name is missing")
		}
		if names[f.Name] {
			return nil, fmt.Errorf("duplicate tool name")
		}
		names[f.Name] = true
	}
	return functions, nil
}
