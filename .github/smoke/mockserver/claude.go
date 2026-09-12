package mockserver

import (
	"fmt"
	"net/http"
	"reflect"
)

const ClaudeSignature = "fixture-claude-signature"

func claudeBlocks() []object {
	blocks := []object{{"type": "thinking", "thinking": "synthetic thought", "signature": ClaudeSignature}, {"type": "text", "text": "检查文档"}}
	for i, name := range Names {
		blocks = append(blocks, object{"type": "tool_use", "id": fmt.Sprintf("fixture-tool-%d", i), "name": name, "input": object{"file": "中文测试.docx"}})
	}
	return blocks
}

func (s *Server) claude(w http.ResponseWriter, r *http.Request, request object) {
	if r.Header.Get("x-api-key") != "fixture-claude-key" || request["model"] != "claude-smoke" {
		s.reject(w, "unexpected Claude key or model")
		return
	}
	messages, _ := request["messages"].([]any)
	if len(messages) == 0 {
		s.reject(w, "missing Claude messages")
		return
	}
	stream, _ := request["stream"].(bool)
	stage, reason, blocks := "tools", "tool_use", claudeBlocks()
	if len(messages) > 1 {
		if len(messages) != 3 {
			s.reject(w, "Claude tool results must be grouped")
			return
		}
		assistant, _ := messages[1].(object)
		result, _ := messages[2].(object)
		content, _ := assistant["content"].([]any)
		results, _ := result["content"].([]any)
		if assistant["role"] != "assistant" || result["role"] != "user" || len(content) != len(blocks) || len(results) != 2 {
			s.reject(w, "Claude block count or roles changed")
			return
		}
		for i := range blocks {
			if !reflect.DeepEqual(content[i], blocks[i]) {
				s.reject(w, "Claude signed blocks changed")
				return
			}
		}
		for i, value := range results {
			block, _ := value.(object)
			if block["type"] != "tool_result" || block["tool_use_id"] != fmt.Sprintf("fixture-tool-%d", i) || block["content"] != `{"ok":true}` {
				s.reject(w, "Claude tool result changed")
				return
			}
		}
		stage, reason, blocks = "followup", "end_turn", []object{{"type": "text", "text": "Claude工具往返成功"}}
	} else {
		tools, _ := request["tools"].([]any)
		if len(tools) != 2 {
			s.reject(w, "expected two Claude tools")
			return
		}
		for i, item := range tools {
			tool, _ := item.(object)
			schema, _ := tool["input_schema"].(object)
			if tool["name"] != Names[i] || schema["type"] != "object" {
				s.reject(w, "Claude tool schema changed")
				return
			}
		}
	}
	s.count(fmt.Sprintf("claude_%s_%t", stage, stream))
	usage := object{"input_tokens": 20, "output_tokens": 8}
	message := object{"id": "fixture-claude-response", "type": "message", "role": "assistant", "model": "claude-smoke", "content": blocks, "stop_reason": reason, "stop_sequence": nil, "usage": usage}
	if !stream {
		reply(w, message, false)
		return
	}
	message["content"], message["stop_reason"] = []object{}, nil
	message["usage"] = object{"input_tokens": 20, "output_tokens": 0}
	reply(w, object{"type": "message_start", "message": message}, true)
	for i, block := range blocks {
		start := object{}
		for key, value := range block {
			start[key] = value
		}
		var deltas []object
		switch block["type"] {
		case "thinking":
			start["thinking"], start["signature"] = "", ""
			deltas = []object{{"type": "thinking_delta", "thinking": block["thinking"]}, {"type": "signature_delta", "signature": ClaudeSignature}}
		case "tool_use":
			start["input"] = object{}
			deltas = []object{{"type": "input_json_delta", "partial_json": `{"file":"中文`}, {"type": "input_json_delta", "partial_json": `测试.docx"}`}}
		case "text":
			start["text"] = ""
			deltas = []object{{"type": "text_delta", "text": block["text"]}}
		}
		reply(w, object{"type": "content_block_start", "index": i, "content_block": start}, true)
		for _, delta := range deltas {
			reply(w, object{"type": "content_block_delta", "index": i, "delta": delta}, true)
		}
		reply(w, object{"type": "content_block_stop", "index": i}, true)
	}
	reply(w, object{"type": "message_delta", "delta": object{"stop_reason": reason, "stop_sequence": nil}, "usage": object{"output_tokens": 8}}, true)
	reply(w, object{"type": "message_stop"}, true)
}
