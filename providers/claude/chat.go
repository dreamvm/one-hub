package claude

import (
	"encoding/json"
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/common/config"
	"one-api/common/image"
	"one-api/common/requester"
	"one-api/common/utils"
	"one-api/providers/base"
	"one-api/types"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
)

const (
	StreamTollsNone = 0
	StreamTollsUse  = 1
	StreamTollsArg  = 2
)

type ClaudeStreamHandler struct {
	Usage       *types.Usage
	Request     *types.ChatCompletionRequest
	StreamTolls int
	Prefix      string
	state       *streamState
}

func (p *ClaudeProvider) CreateChatCompletion(request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
	request.OneOtherArg = p.GetOtherArg()
	claudeRequest, errWithCode := ConvertFromChatOpenai(request)
	if errWithCode != nil {
		return nil, errWithCode
	}

	req, errWithCode := p.getChatRequest(claudeRequest)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	claudeResponse := &ClaudeResponse{}
	// 发送请求
	_, errWithCode = p.Requester.SendRequest(req, claudeResponse, false)
	if errWithCode != nil {
		return nil, errWithCode
	}

	return ConvertToChatOpenai(p, claudeResponse, request)
}

func (p *ClaudeProvider) CreateChatCompletionStream(request *types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	request.OneOtherArg = p.GetOtherArg()
	claudeRequest, errWithCode := ConvertFromChatOpenai(request)
	if errWithCode != nil {
		return nil, errWithCode
	}

	req, errWithCode := p.getChatRequest(claudeRequest)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	// 发送请求
	resp, errWithCode := p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}

	chatHandler := &ClaudeStreamHandler{
		Usage:   p.Usage,
		Request: request,
		Prefix:  `data: {"type"`,
	}

	eventstream.NewDecoder()

	return requester.RequestStream(p.Requester, resp, chatHandler.HandlerStream)
}

func (p *ClaudeProvider) getChatRequest(claudeRequest *ClaudeRequest) (*http.Request, *types.OpenAIErrorWithStatusCode) {
	url, errWithCode := p.GetSupportedAPIUri(config.RelayModeChatCompletions)
	if errWithCode != nil {
		return nil, errWithCode
	}

	// 获取请求地址
	fullRequestURL := p.GetFullRequestURL(url)
	if fullRequestURL == "" {
		return nil, common.ErrorWrapperLocal(nil, "invalid_claude_config", http.StatusInternalServerError)
	}

	headers := p.GetRequestHeaders()
	if claudeRequest.Stream {
		headers["Accept"] = "text/event-stream"
	}

	if headers["anthropic-beta"] == "" && strings.HasPrefix(claudeRequest.Model, "claude-3-5-sonnet") {
		headers["anthropic-beta"] = "max-tokens-3-5-sonnet-2024-07-15"
	}

	if headers["anthropic-beta"] == "" && strings.HasPrefix(claudeRequest.Model, "claude-3-7-sonnet") {
		headers["anthropic-beta"] = "output-128k-2025-02-19"
	}

	// 创建请求
	req, err := p.Requester.NewRequest(http.MethodPost, fullRequestURL, p.Requester.WithBody(claudeRequest), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, common.ErrorWrapperLocal(err, "new_request_failed", http.StatusInternalServerError)
	}

	return req, nil
}

func ConvertFromChatOpenai(request *types.ChatCompletionRequest) (*ClaudeRequest, *types.OpenAIErrorWithStatusCode) {
	claudeRequest := ClaudeRequest{
		Model:         request.Model,
		Messages:      make([]Message, 0),
		MaxTokens:     request.MaxTokens,
		StopSequences: nil,
		Temperature:   request.Temperature,
		TopP:          request.TopP,
		Stream:        request.Stream,
	}

	if request.Stop != nil {
		stopBytes, err := json.Marshal(request.Stop)
		if err == nil {
			var stopSequences []string
			if err := json.Unmarshal(stopBytes, &stopSequences); err == nil {
				claudeRequest.StopSequences = stopSequences
			} else if stop, ok := request.Stop.(string); ok {
				claudeRequest.StopSequences = []string{stop}
			}
		}
	}

	// 处理 system 字段（支持 cache_control）
	systemMessage := ""

	// 如果请求中已经有 system 字段（如数组格式带 cache_control），直接使用
	if request.System != nil {
		// 检查是否为空字符串，避免传递空 system 字段
		if str, ok := request.System.(string); ok && str == "" {
			// 空字符串不设置，保持为 nil
		} else {
			claudeRequest.System = request.System
		}
	}

	// 处理 messages
	for _, msg := range request.Messages {
		if msg.IsSystemRole() {
			// 如果没有预设的 system 字段，从 messages 中提取
			if request.System == nil {
				systemMessage += msg.StringContent()
			}
			continue
		}
		messageContent, err := convertMessageContent(&msg)
		if err != nil {
			return nil, common.ErrorWrapper(err, "conversion_error", http.StatusBadRequest)
		}
		if messageContent != nil {
			claudeRequest.Messages = append(claudeRequest.Messages, *messageContent)
		}
	}

	// 如果没有预设的 system 字段，且从 messages 中提取到了 system message
	if request.System == nil && systemMessage != "" {
		claudeRequest.System = systemMessage
	}

	functions, controlErr := request.CheckedFunctions()
	if controlErr != nil {
		return nil, common.ErrorWrapperLocal(controlErr, "invalid_tools", 400)
	}
	for _, f := range functions {
		tool := Tools{
			Name:        f.Name,
			Description: f.Description,
			InputSchema: f.Parameters,
			Strict:      f.Strict,
		}
		if tool.InputSchema == nil {
			tool.InputSchema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		claudeRequest.Tools = append(claudeRequest.Tools, tool)
	}

	toolType, toolFunc, controlErr := request.CheckedToolChoice()
	if controlErr != nil {
		return nil, common.ErrorWrapperLocal(controlErr, "invalid_tool_choice", 400)
	}
	if toolType == types.ToolChoiceTypeRequired && len(functions) == 0 {
		return nil, common.StringErrorWrapperLocal("required tool choice needs tools", "invalid_tool_choice", 400)
	}
	if toolType == types.ToolChoiceTypeFunction {
		found := false
		for _, f := range functions {
			if f.Name == toolFunc {
				found = true
			}
		}
		if !found {
			return nil, common.StringErrorWrapperLocal("selected function is not declared", "invalid_tool_choice", 400)
		}
	}
	if request.ToolChoice != nil || request.FunctionCall != nil || request.ParallelToolCalls != nil {
		claudeRequest.ToolChoice = ConvertToolChoice(toolType, toolFunc)
		if request.ParallelToolCalls != nil {
			claudeRequest.ToolChoice.DisableParallelToolUse = !*request.ParallelToolCalls
		}
	}

	if claudeRequest.MaxTokens == 0 {
		claudeRequest.MaxTokens = config.ClaudeSettingsInstance.GetDefaultMaxTokens(request.Model)
	}

	// 如果是3-7 默认开启thinking
	if request.OneOtherArg == "thinking" || request.Reasoning != nil {
		var opErr *types.OpenAIErrorWithStatusCode
		claudeRequest.MaxTokens, claudeRequest.Thinking, opErr = getThinking(claudeRequest.MaxTokens, request.Reasoning)

		if opErr != nil {
			return nil, opErr
		}

		claudeRequest.TopP = nil
		if toolType == types.ToolChoiceTypeRequired || toolType == types.ToolChoiceTypeFunction {
			return nil, common.StringErrorWrapperLocal("manual thinking does not support forced tool choice", "invalid_tool_choice", 400)
		}
	}
	if err := normalizeToolHistory(&claudeRequest); err != nil {
		return nil, common.ErrorWrapperLocal(err, "invalid_tool_history", 400)
	}

	return &claudeRequest, nil
}

func getThinking(maxTokens int, reasoning *types.ChatReasoning) (newMaxtokens int, thinking *Thinking, err *types.OpenAIErrorWithStatusCode) {
	newMaxtokens = maxTokens
	thinking = &Thinking{
		Type: "enabled",
	}

	if reasoning == nil || (reasoning.MaxTokens == 0 && reasoning.Effort == "") {
		thinking.BudgetTokens = int(float64(maxTokens) * config.ClaudeSettingsInstance.BudgetTokensPercentage)
	} else if reasoning.MaxTokens > 0 {
		if reasoning.MaxTokens < 1024 {
			err = common.StringErrorWrapper("budget_token must be greater than 1024", "budget_tokens_too_small", http.StatusBadRequest)
			return
		}

		if reasoning.MaxTokens > maxTokens {
			err = common.StringErrorWrapper(fmt.Sprintf("budget_token cannot be greater than the max_token, max_token: %d, budget_token: %d", maxTokens, reasoning.MaxTokens), "budget_tokens_too_large", http.StatusBadRequest)
			return
		}
		thinking.BudgetTokens = reasoning.MaxTokens
	} else {
		switch reasoning.Effort {
		case "low":
			thinking.BudgetTokens = int(float64(maxTokens) * 0.2)
		case "medium":
			thinking.BudgetTokens = int(float64(maxTokens) * 0.5)
		default:
			thinking.BudgetTokens = int(float64(maxTokens) * 0.8)
		}
	}

	// 如果低于1024,则设置为1024
	if thinking.BudgetTokens < 1024 {
		thinking.BudgetTokens = 1024
	}

	if newMaxtokens <= thinking.BudgetTokens {
		newMaxtokens = 1280
	}

	return
}

func ConvertToolChoice(toolType, toolFunc string) *ToolChoice {
	choice := &ToolChoice{Type: "auto"}

	switch toolType {
	case types.ToolChoiceTypeFunction:
		choice.Type = "tool"
		choice.Name = toolFunc
	case types.ToolChoiceTypeRequired:
		choice.Type = "any"
	case types.ToolChoiceTypeNone:
		choice.Type = "none"
	}

	return choice
}

func convertMessageContent(msg *types.ChatCompletionMessage) (*Message, error) {
	msg.FuncToToolCalls()
	message := Message{
		Role: convertRole(msg.Role),
	}

	content := make([]MessageContent, 0)
	if replay, err := replayAssistant(msg); err != nil {
		return nil, err
	} else if replay != nil {
		message.Content = replay
		return &message, nil
	}

	if msg.ToolCalls != nil {
		if text := msg.StringContent(); text != "" {
			content = append(content, MessageContent{Type: "text", Text: text})
		}
		for _, toolCall := range msg.ToolCalls {
			if toolCall == nil || toolCall.Function == nil || toolCall.Id == "" || toolCall.Function.Name == "" {
				return nil, fmt.Errorf("invalid tool call")
			}
			inputParam := make(map[string]any)
			args := toolCall.Function.Arguments
			if args == "" {
				args = "{}"
			}
			if err := json.Unmarshal([]byte(args), &inputParam); err != nil {
				return nil, err
			}
			if inputParam == nil {
				return nil, fmt.Errorf("tool arguments must be an object")
			}
			content = append(content, MessageContent{
				Type:  ContentTypeToolUes,
				Id:    toolCall.Id,
				Name:  toolCall.Function.Name,
				Input: inputParam,
			})
		}

		message.Content = content
		return &message, nil
	}

	if msg.Role == types.ChatMessageRoleTool || msg.Role == types.ChatMessageRoleFunction {
		id := msg.ToolCallID
		if id == "" && msg.Role == types.ChatMessageRoleFunction && msg.Name != nil {
			id = *msg.Name
		}
		result, err := toolResultContent(msg.Content)
		if err != nil {
			return nil, err
		}
		content = append(content, MessageContent{
			Type:      ContentTypeToolResult,
			Content:   result,
			ToolUseId: id,
			IsError:   msg.IsError,
		})

		message.Content = content
		return &message, nil
	}

	openaiContent := msg.ParseContent()
	for _, part := range openaiContent {
		if part.Type == types.ContentTypeText {
			msgContent := MessageContent{
				Type: "text",
				Text: part.Text,
			}
			// 传递 cache_control 字段
			if msg.CacheControl != nil {
				msgContent.CacheControl = msg.CacheControl
			}
			content = append(content, msgContent)
			continue
		}
		if part.Type == types.ContentTypeImageURL {
			mimeType, data, err := image.GetImageFromUrl(part.ImageURL.URL)
			if err != nil {
				return nil, common.ErrorWrapper(err, "image_url_invalid", http.StatusBadRequest)
			}
			claudeType := "image"

			if mimeType == "application/pdf" {
				claudeType = "document"
			}
			content = append(content, MessageContent{
				Type: claudeType,
				Source: &ContentSource{
					Type:      "base64",
					MediaType: mimeType,
					Data:      data,
				},
			})
		}
	}

	message.Content = content

	return &message, nil
}

func ConvertToChatOpenai(provider base.ProviderInterface, response *ClaudeResponse, request *types.ChatCompletionRequest) (openaiResponse *types.ChatCompletionResponse, errWithCode *types.OpenAIErrorWithStatusCode) {
	aiError := errorHandle(response.Error)
	if aiError != nil {
		errWithCode = &types.OpenAIErrorWithStatusCode{
			OpenAIError: *aiError,
			StatusCode:  http.StatusBadRequest,
		}
		return
	}

	choice := types.ChatCompletionChoice{Index: 0, Message: types.ChatCompletionMessage{Role: response.Role}, FinishReason: stopReasonClaude2OpenAI(response.StopReason)}
	var text, thinking strings.Builder
	for _, content := range response.Content {
		switch content.Type {
		case ContentTypeToolUes:
			choice.Message.ToolCalls = append(choice.Message.ToolCalls, content.ToOpenAITool())
		case ContentTypeThinking:
			thinking.WriteString(content.Thinking)
		case ContentTypeRedactedThinking:
		case ContentTypeText:
			text.WriteString(content.Text)
		default:
			return nil, common.StringErrorWrapperLocal("unsupported Claude content block; use native Messages API", "unsupported_content", 502)
		}
	}
	choice.Message.Content = text.String()
	choice.Message.ReasoningContent = thinking.String()
	choice.Message.ExtraContent = packAssistant(response.Content)
	if len(choice.Message.ToolCalls) > 0 {
		choice.Message.ToolCalls[0].ExtraContent = choice.Message.ExtraContent
	}
	if request.Functions != nil {
		for _, content := range response.Content {
			if content.Type == ContentTypeThinking || content.Type == ContentTypeRedactedThinking {
				return nil, common.StringErrorWrapperLocal("signed thinking requires modern tools/tool_calls", "unsupported_legacy_thinking", 400)
			}
		}
		if len(choice.Message.ToolCalls) > 1 {
			return nil, common.StringErrorWrapperLocal("legacy function_call cannot represent parallel tools", "unsupported_legacy_tools", 400)
		}
		choice.Message.ExtraContent = nil
	}
	choice.CheckChoice(request)
	choices := []types.ChatCompletionChoice{choice}

	openaiResponse = &types.ChatCompletionResponse{
		ID:      response.Id,
		Object:  "chat.completion",
		Created: utils.GetTimestamp(),
		Choices: choices,
		Model:   request.Model,
		Usage: &types.Usage{
			CompletionTokens: 0,
			PromptTokens:     0,
			TotalTokens:      0,
		},
	}

	usage := provider.GetUsage()
	isOk := ClaudeUsageToOpenaiUsage(&response.Usage, usage)
	if !isOk {
		usage.CompletionTokens = ClaudeOutputUsage(response)
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	openaiResponse.Usage = usage

	return openaiResponse, nil
}
