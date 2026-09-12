// Package mockserver provides a synthetic upstream, never a real model service.
package mockserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
)

type object = map[string]any

// Signatures are deliberately synthetic opaque values, not provider credentials.
var Signatures = []string{"fixture-signature-A", "fixture-signature-B"}
var Names = []string{"read_doc_metadata", "list_doc_sections"}

type Server struct {
	mu     sync.Mutex
	counts map[string]int
}

func New() *Server { return &Server{counts: make(map[string]int)} }

func (s *Server) count(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[key]++
}

func reply(w http.ResponseWriter, value any, stream bool) {
	data, _ := json.Marshal(value)
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func (s *Server) reject(w http.ResponseWriter, message string) {
	s.count("rejected")
	w.WriteHeader(http.StatusBadRequest)
	reply(w, object{"error": object{"code": 400, "status": "INVALID_ARGUMENT", "message": message}}, false)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/stats" {
		s.mu.Lock()
		defer s.mu.Unlock()
		reply(w, s.counts, false)
		return
	}
	if r.Method != http.MethodPost {
		s.reject(w, "unexpected method")
		return
	}
	var request object
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
		s.reject(w, "invalid JSON")
		return
	}
	if r.URL.Path == "/v1/chat/completions" {
		s.openai(w, r, request)
		return
	}
	if r.URL.Path != "/v1beta/models/gemini-smoke:generateContent" && r.URL.Path != "/v1beta/models/gemini-smoke:streamGenerateContent" {
		s.reject(w, "unexpected upstream path")
		return
	}
	if r.Header.Get("x-goog-api-key") != "fixture-gemini-key" {
		s.reject(w, "missing synthetic Gemini key")
		return
	}
	stream := strings.HasSuffix(r.URL.Path, ":streamGenerateContent")
	contents, _ := request["contents"].([]any)
	var calls, results []object
	for _, content := range contents {
		c, _ := content.(object)
		role, _ := c["role"].(string)
		if role != "user" && role != "model" {
			s.reject(w, "unsupported Gemini content role")
			return
		}
		parts, _ := c["parts"].([]any)
		for _, part := range parts {
			p, _ := part.(object)
			if _, ok := p["functionCall"]; ok {
				if role != "model" {
					s.reject(w, "functionCall requires model role")
					return
				}
				calls = append(calls, p)
			}
			if value, ok := p["functionResponse"].(object); ok {
				if role != "user" {
					s.reject(w, "functionResponse requires user role")
					return
				}
				results = append(results, value)
			}
		}
	}
	parts := []object{}
	stage := "tools"
	if len(calls) > 0 || len(results) > 0 {
		if len(calls) != 2 || len(results) != 2 {
			s.reject(w, "both tool calls and results must survive")
			return
		}
		for i, part := range calls {
			call, _ := part["functionCall"].(object)
			response, _ := results[i]["response"].(object)
			if part["thoughtSignature"] != Signatures[i] || call["name"] != Names[i] ||
				!reflect.DeepEqual(call["args"], object{"file": "中文测试.docx"}) ||
				results[i]["name"] != Names[i] || response["content"] != `{"ok":true}` {
				s.reject(w, "tool signature, arguments, name or result changed")
				return
			}
		}
		stage = "followup"
		parts = append(parts, object{"text": "工具签名往返成功"})
	} else {
		tools, _ := request["tools"].([]any)
		if len(contents) == 0 || len(tools) != 1 {
			s.reject(w, "missing messages or tool declarations")
			return
		}
		tool, _ := tools[0].(object)
		declarations, _ := tool["functionDeclarations"].([]any)
		if len(declarations) != 2 {
			s.reject(w, "expected two declarations")
			return
		}
		for i, name := range Names {
			parts = append(parts, object{"functionCall": object{"name": name, "args": object{"file": "中文测试.docx"}}, "thoughtSignature": Signatures[i]})
		}
	}
	s.count(fmt.Sprintf("gemini_%s_%t", stage, stream))
	reply(w, object{
		"responseId": "fixture-gemini-response", "modelVersion": "gemini-smoke",
		"candidates":    []object{{"index": 0, "content": object{"role": "model", "parts": parts}, "finishReason": "STOP"}},
		"usageMetadata": object{"promptTokenCount": 20, "candidatesTokenCount": 8, "totalTokenCount": 28},
	}, stream)
}

func (s *Server) openai(w http.ResponseWriter, r *http.Request, request object) {
	if r.Header.Get("Authorization") != "Bearer fixture-openai-key" {
		s.reject(w, "missing synthetic OpenAI key")
		return
	}
	model, _ := request["model"].(string)
	if model != "openai-smoke" && model != "openai-https-smoke" {
		s.reject(w, "unexpected model")
		return
	}
	stream, _ := request["stream"].(bool)
	s.count(fmt.Sprintf("%s_%t", model, stream))
	usage := object{"prompt_tokens": 10, "completion_tokens": 4, "total_tokens": 14}
	base := object{"id": "fixture-openai-response", "created": 1, "model": model}
	if stream {
		base["object"] = "chat.completion.chunk"
		base["choices"] = []object{{"index": 0, "delta": object{"role": "assistant", "content": "中文对话成功"}, "finish_reason": nil}}
		reply(w, base, true)
		base["choices"] = []object{{"index": 0, "delta": object{}, "finish_reason": "stop"}}
		base["usage"] = usage
		reply(w, base, true)
		fmt.Fprint(w, "data: [DONE]\n\n")
		return
	}
	base["object"] = "chat.completion"
	base["choices"] = []object{{"index": 0, "message": object{"role": "assistant", "content": "中文对话成功"}, "finish_reason": "stop"}}
	base["usage"] = usage
	reply(w, base, false)
}
