package gemini

import (
	"encoding/json"
	"fmt"
	"io"

	"one-api/common/utils"
	"one-api/types"
)

type geminiStreamState struct {
	id       string
	created  int64
	tools    map[int64][]GeminiPart
	finished map[int64]bool
}

func validateFunctionParts(parts []GeminiPart) error {
	for _, part := range parts {
		if f := part.FunctionCall; f != nil {
			if f.WillContinue != nil || len(f.PartialArgs) > 0 {
				return fmt.Errorf("incremental partialArgs function calls require a platform-specific adapter; no completed call was emitted")
			}
			if f.Name == "" {
				return fmt.Errorf("upstream function name missing")
			}
		}
	}
	return nil
}

func (h *GeminiStreamHandler) EndError() error {
	if h.state == nil || len(h.state.finished) == 0 {
		return io.ErrUnexpectedEOF
	}
	for _, finished := range h.state.finished {
		if !finished {
			return io.ErrUnexpectedEOF
		}
	}
	return nil
}

func (h *GeminiStreamHandler) convertToOpenaiStream(response *GeminiChatResponse, data chan string) error {
	if h.state == nil {
		h.state = &geminiStreamState{id: "chatcmpl-" + utils.GetUUID(), created: utils.GetTimestamp(), tools: make(map[int64][]GeminiPart), finished: make(map[int64]bool)}
	}
	s := h.state
	if response.ResponseId != "" {
		s.id = response.ResponseId
	}
	if h.Usage == nil {
		h.Usage = &types.Usage{}
	}
	if response.UsageMetadata != nil {
		usage := ConvertOpenAIUsage(response.UsageMetadata)
		usage.TextBuilder = h.Usage.TextBuilder
		*h.Usage = usage
	}
	emit := func(choice types.ChatCompletionStreamChoice) error {
		h.Usage.TextBuilder.WriteString(choice.Delta.Content)
		out := types.ChatCompletionStreamResponse{ID: s.id, Object: "chat.completion.chunk", Created: s.created, Model: h.Request.Model, Choices: []types.ChatCompletionStreamChoice{choice}}
		if choice.FinishReason != nil {
			out.Usage = h.Usage
		}
		b, err := json.Marshal(out)
		if err != nil {
			return err
		}
		data <- string(b)
		return nil
	}
	for _, candidate := range response.Candidates {
		if err := validateFunctionParts(candidate.Content.Parts); err != nil {
			return err
		}
		if s.finished[candidate.Index] {
			return fmt.Errorf("upstream emitted content after final candidate")
		}
		s.finished[candidate.Index] = false
		var textParts []GeminiPart
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				s.tools[candidate.Index] = append(s.tools[candidate.Index], part)
			} else {
				textParts = append(textParts, part)
			}
		}
		textCandidate := candidate
		textCandidate.FinishReason = nil
		textCandidate.Content.Parts = textParts
		if len(textParts) > 0 {
			choice := textCandidate.ToOpenAIStreamChoice(h.Request)
			choice.FinishReason = nil
			if err := emit(choice); err != nil {
				return err
			}
		}
		if candidate.FinishReason == nil {
			continue
		}
		s.finished[candidate.Index] = true
		calls := s.tools[candidate.Index]
		if len(calls) > 0 {
			if *candidate.FinishReason != "STOP" {
				return fmt.Errorf("upstream stopped before successful tool completion")
			}
			if h.Request.Functions != nil && len(calls) > 1 {
				return fmt.Errorf("legacy function_call cannot represent parallel tools")
			}
			candidate.Content.Parts = calls
			choice := candidate.ToOpenAIStreamChoice(h.Request)
			for _, delta := range choice.ConvertOpenaiStream() {
				if err := emit(delta); err != nil {
					return err
				}
			}
			delete(s.tools, candidate.Index)
		} else {
			choice := types.ChatCompletionStreamChoice{Index: int(candidate.Index), FinishReason: ConvertFinishReason(*candidate.FinishReason)}
			if err := emit(choice); err != nil {
				return err
			}
		}
	}
	return nil
}
