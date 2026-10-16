package gemini

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"one-api/common"
	"one-api/common/requester"
	"one-api/common/utils"
	"one-api/types"
)

// Finalize the accepted attempt before handing its terminal status to the
// relay. This also covers interrupted streams, which bypass EndError. Waiting
// on the terminal channel orders usage updates after the stream parser.
type geminiUsageStream struct {
	requester.StreamReaderInterface[string]
	handler *GeminiStreamHandler
}

func (s *geminiUsageStream) Recv() (<-chan string, <-chan error) {
	data, upstreamErrors := s.StreamReaderInterface.Recv()
	terminal := make(chan error, 1)
	go func() {
		err := <-upstreamErrors
		if usageErr := s.handler.finalizeAcceptedUsage(); usageErr != nil {
			err = &requester.ProtocolError{Message: "Gemini stream: invalid combined usage", Cause: usageErr}
		}
		terminal <- err
	}()
	return data, terminal
}

func (h *GeminiStreamHandler) finalizeAcceptedUsage() error {
	if h.acceptedCompletionTokens != 0 || h.Usage.TextBuilder.Len() == 0 {
		return nil
	}
	// The relay normally estimates missing completion usage. Discarded thought
	// tokens must not suppress that fallback for the accepted attempt. Estimate
	// once, after all text, then add the discarded completion exactly once.
	estimate := &types.Usage{PromptTokens: h.Usage.PromptTokens, CompletionTokens: h.priorUsage.CompletionTokens}
	if err := mergeGeminiUsage(estimate, &types.Usage{CompletionTokens: common.CountTokenText(h.Usage.TextBuilder.String(), h.billingModel)}); err != nil {
		return err
	}
	h.Usage.CompletionTokens = estimate.CompletionTokens
	h.Usage.TotalTokens = estimate.PromptTokens + estimate.CompletionTokens
	return nil
}

type geminiStreamState struct {
	id        string
	created   int64
	tools     map[int64][]GeminiPart
	finished  map[int64]bool
	delivered map[int64]bool
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
		h.state = &geminiStreamState{id: "chatcmpl-" + utils.GetUUID(), created: utils.GetTimestamp(), tools: make(map[int64][]GeminiPart), finished: make(map[int64]bool), delivered: make(map[int64]bool)}
	}
	s := h.state
	if response.ResponseId != "" {
		s.id = response.ResponseId
	}
	if h.Usage == nil {
		h.Usage = &types.Usage{}
	}
	if response.UsageMetadata != nil {
		usage, err := geminiStreamUsage(response.UsageMetadata)
		if err != nil {
			return err
		}
		h.acceptedCompletionTokens = usage.CompletionTokens
		if err := mergeGeminiUsage(&usage, h.priorUsage); err != nil {
			return err
		}
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
		if strings.TrimSpace(choice.Delta.Content) != "" || len(choice.Delta.Image) > 0 || len(choice.Delta.ToolCalls) > 0 {
			s.delivered[int64(choice.Index)] = true
		}
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
		if candidate.FinishReason != nil {
			if err := validateCandidateCompletion(candidate, s.delivered[candidate.Index] || hasCandidateDeliverable(textParts), len(s.tools[candidate.Index]) > 0); err != nil {
				return err
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
