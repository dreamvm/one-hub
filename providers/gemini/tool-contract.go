package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"one-api/types"
)

func convertTools(request *types.ChatCompletionRequest, out *GeminiChatRequest) error {
	functions, err := request.CheckedFunctions()
	if err != nil {
		return err
	}
	mode, name, err := request.CheckedToolChoice()
	if err != nil {
		return err
	}
	if request.ParallelToolCalls != nil && !*request.ParallelToolCalls && mode != types.ToolChoiceTypeNone {
		return fmt.Errorf("Gemini cannot guarantee parallel_tool_calls=false through this adapter")
	}
	var declarations []GeminiFunctionDeclaration
	strict := false
	found := false
	for _, f := range functions {
		var builtin GeminiChatTools
		switch f.Name {
		case "googleSearch":
			builtin.GoogleSearch = &GeminiCodeExecution{}
		case "urlContext":
			builtin.UrlContext = &GeminiCodeExecution{}
		case "codeExecution":
			builtin.CodeExecution = &GeminiCodeExecution{}
		default:
			if strings.TrimSpace(f.Name) != f.Name {
				return fmt.Errorf("tool name contains surrounding whitespace")
			}
			var schema map[string]any
			if f.Parameters != nil {
				b, e := json.Marshal(f.Parameters)
				if e != nil || json.Unmarshal(b, &schema) != nil || schema == nil {
					return fmt.Errorf("tool parameters must be a JSON schema object")
				}
				if schema["type"] != "object" {
					return fmt.Errorf("tool parameters must have type object")
				}
			} else {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			declarations = append(declarations, GeminiFunctionDeclaration{Name: f.Name, Description: f.Description, ParametersJsonSchema: schema})
			if f.Strict != nil && *f.Strict {
				strict = true
			}
			if f.Name == name {
				found = true
			}
			continue
		}
		// NONE applies to all tools, including the legacy built-in aliases.
		if mode != types.ToolChoiceTypeNone {
			out.Tools = append(out.Tools, builtin)
		}
	}
	if len(declarations) > 0 {
		out.Tools = append(out.Tools, GeminiChatTools{FunctionDeclarations: declarations})
	}
	control := &GeminiFunctionCallingConfig{Mode: "AUTO"}
	switch mode {
	case types.ToolChoiceTypeNone:
		control.Mode = "NONE"
	case types.ToolChoiceTypeRequired:
		if len(declarations) == 0 {
			return fmt.Errorf("required tool choice needs function declarations")
		}
		control.Mode = "ANY"
	case types.ToolChoiceTypeFunction:
		if !found {
			return fmt.Errorf("selected function is not declared")
		}
		control.Mode = "ANY"
		control.AllowedFunctionNames = []string{name}
	}
	// ANY provides schema-constrained function calls. Do not silently force a
	// call when the caller requested auto merely to emulate OpenAI strict mode.
	if strict && control.Mode == "AUTO" {
		return fmt.Errorf("Gemini strict tools require required or named tool_choice; auto strict is not supported by this adapter")
	}
	if len(functions) > 0 || request.ToolChoice != nil || request.FunctionCall != nil {
		out.ToolConfig = &GeminiToolConfig{FunctionCallingConfig: control}
	}
	return nil
}
