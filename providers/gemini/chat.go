package gemini

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"one-api/common"
	"one-api/common/config"
	"one-api/common/requester"
	"one-api/common/utils"
	"one-api/providers/base"
	"one-api/types"
	"strings"
)

const (
	GeminiVisionMaxImageNum = 16
)

type GeminiStreamHandler struct {
	Usage   *types.Usage
	Request *types.ChatCompletionRequest

	key   string
	state *geminiStreamState
}

type OpenAIStreamHandler struct {
	Usage     *types.Usage
	ModelName string
}

func (p *GeminiProvider) CreateChatCompletion(request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
	if p.UseOpenaiAPI {
		return p.OpenAIProvider.CreateChatCompletion(request)
	}

	geminiRequest, errWithCode := ConvertFromChatOpenai(request)
	if errWithCode != nil {
		return nil, errWithCode
	}

	req, errWithCode := p.getChatRequest(geminiRequest, false)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	geminiChatResponse := &GeminiChatResponse{}
	// 发送请求
	_, errWithCode = p.Requester.SendRequest(req, geminiChatResponse, false)
	if errWithCode != nil {
		return nil, errWithCode
	}

	return ConvertToChatOpenai(p, geminiChatResponse, request)
}

func (p *GeminiProvider) CreateChatCompletionStream(request *types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {

	channel := p.GetChannel()
	if p.UseOpenaiAPI {
		return p.OpenAIProvider.CreateChatCompletionStream(request)
	}

	geminiRequest, errWithCode := ConvertFromChatOpenai(request)
	if errWithCode != nil {
		return nil, errWithCode
	}

	req, errWithCode := p.getChatRequest(geminiRequest, false)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	// 发送请求
	resp, errWithCode := p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}

	chatHandler := &GeminiStreamHandler{
		Usage:   p.Usage,
		Request: request,

		key: channel.Key,
	}

	return requester.RequestStream(p.Requester, resp, chatHandler.HandlerStream, chatHandler.EndError)
}

func (p *GeminiProvider) getChatRequest(geminiRequest *GeminiChatRequest, isRelay bool) (*http.Request, *types.OpenAIErrorWithStatusCode) {
	url := "generateContent"
	if geminiRequest.Stream {
		url = "streamGenerateContent?alt=sse"
	}
	// 获取请求地址
	fullRequestURL := p.GetFullRequestURL(url, geminiRequest.Model)

	// 获取请求头
	headers := p.GetRequestHeaders()
	if geminiRequest.Stream {
		headers["Accept"] = "text/event-stream"
	}

	var body any
	if isRelay {
		var exists bool
		body, exists = p.GetRawBody()
		if !exists {
			return nil, common.StringErrorWrapperLocal("request body not found", "request_body_not_found", http.StatusInternalServerError)
		}
	} else {
		p.pluginHandle(geminiRequest)
		body = geminiRequest
	}

	// 创建请求
	req, err := p.Requester.NewRequest(http.MethodPost, fullRequestURL, p.Requester.WithBody(body), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, common.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}

	return req, nil
}

func ConvertFromChatOpenai(request *types.ChatCompletionRequest) (*GeminiChatRequest, *types.OpenAIErrorWithStatusCode) {

	threshold := "BLOCK_NONE"

	// if strings.HasPrefix(request.Model, "gemini-2.0") && !strings.Contains(request.Model, "thinking") {
	// 	threshold = "OFF"
	// }

	geminiRequest := GeminiChatRequest{
		Contents: make([]GeminiChatContent, 0, len(request.Messages)),
		SafetySettings: []GeminiChatSafetySettings{
			{
				Category:  "HARM_CATEGORY_HARASSMENT",
				Threshold: threshold,
			},
			{
				Category:  "HARM_CATEGORY_HATE_SPEECH",
				Threshold: threshold,
			},
			{
				Category:  "HARM_CATEGORY_SEXUALLY_EXPLICIT",
				Threshold: threshold,
			},
			{
				Category:  "HARM_CATEGORY_DANGEROUS_CONTENT",
				Threshold: threshold,
			},
			{
				Category:  "HARM_CATEGORY_CIVIC_INTEGRITY",
				Threshold: threshold,
			},
		},
		GenerationConfig: GeminiChatGenerationConfig{
			Temperature:        request.Temperature,
			TopP:               request.TopP,
			MaxOutputTokens:    request.MaxTokens,
			ResponseModalities: request.Modalities,
		},
	}

	if strings.HasPrefix(request.Model, "gemini-2.0-flash-exp") || strings.HasPrefix(request.Model, "gemini-2.5-flash-image-preview") {
		geminiRequest.GenerationConfig.ResponseModalities = []string{"Text", "Image"}
	}

	if strings.HasSuffix(request.Model, "-tts") {
		geminiRequest.GenerationConfig.ResponseModalities = []string{"AUDIO"}
	}

	if request.Reasoning != nil {
		thinkingConfig := &ThinkingConfig{}

		// Set ThinkingBudget when MaxTokens >= 0
		if request.Reasoning.MaxTokens >= 0 {
			thinkingConfig.ThinkingBudget = &request.Reasoning.MaxTokens
		}

		// Convert effort to thinkingLevel
		if request.Reasoning.Effort != "" {
			effortToLevelMap := map[string]string{
				"minimal": "MINIMAL",
				"low":     "LOW",
				"medium":  "MEDIUM",
				"high":    "HIGH",
			}
			if level, ok := effortToLevelMap[request.Reasoning.Effort]; ok {
				thinkingConfig.ThinkingLevel = level
			}
		}

		// Only set ThinkingConfig if at least one parameter is set
		if thinkingConfig.ThinkingBudget != nil || thinkingConfig.ThinkingLevel != "" {
			geminiRequest.GenerationConfig.ThinkingConfig = thinkingConfig
		}
	} else if request.ReasoningEffort != nil {
		// Preserve the existing nested reasoning contract when both are set.
		// Standard OpenAI clients send reasoning_effort instead; do not ignore it
		// or invent a zero thinking budget while converting an effort-only request.
		switch *request.ReasoningEffort {
		case "minimal", "low", "medium", "high":
			geminiRequest.GenerationConfig.ThinkingConfig = &ThinkingConfig{
				ThinkingLevel: strings.ToUpper(*request.ReasoningEffort),
			}
		default:
			return nil, common.ErrorWrapperLocal(errors.New("Gemini reasoning_effort must be minimal, low, medium, or high"), "invalid_reasoning_effort", http.StatusBadRequest)
		}
	}

	if config.GeminiSettingsInstance.GetOpenThink(request.Model) {
		if geminiRequest.GenerationConfig.ThinkingConfig == nil {
			geminiRequest.GenerationConfig.ThinkingConfig = &ThinkingConfig{}
		}
		geminiRequest.GenerationConfig.ThinkingConfig.IncludeThoughts = true
	}

	if err := convertTools(request, &geminiRequest); err != nil {
		return nil, common.ErrorWrapperLocal(err, "invalid_tools", http.StatusBadRequest)
	}

	geminiContent, systemContent, err := OpenAIToGeminiChatContent(request.Messages)
	if err != nil {
		return nil, err
	}

	if systemContent != "" {
		geminiRequest.SystemInstruction = &GeminiChatContent{
			Parts: []GeminiPart{
				{Text: systemContent},
			},
		}
	}

	geminiRequest.Contents = geminiContent
	geminiRequest.Stream = request.Stream
	geminiRequest.Model = request.Model

	if request.ResponseFormat != nil && (request.ResponseFormat.Type == "json_schema" || request.ResponseFormat.Type == "json_object") {
		geminiRequest.GenerationConfig.ResponseMimeType = "application/json"

		if request.ResponseFormat.JsonSchema != nil && request.ResponseFormat.JsonSchema.Schema != nil {
			cleanedSchema := removeAdditionalPropertiesWithDepth(request.ResponseFormat.JsonSchema.Schema, 0)
			geminiRequest.GenerationConfig.ResponseSchema = cleanedSchema
		}
	}

	return &geminiRequest, nil
}

func removeAdditionalPropertiesWithDepth(schema interface{}, depth int) interface{} {
	if depth >= 5 {
		return schema
	}

	v, ok := schema.(map[string]interface{})
	if !ok || len(v) == 0 {
		return schema
	}

	// 如果type不为object和array，则直接返回
	if typeVal, exists := v["type"]; !exists || (typeVal != "object" && typeVal != "array") {
		return schema
	}

	delete(v, "title")

	switch v["type"] {
	case "object":
		delete(v, "additionalProperties")
		// 处理 properties
		if properties, ok := v["properties"].(map[string]interface{}); ok {
			for key, value := range properties {
				properties[key] = removeAdditionalPropertiesWithDepth(value, depth+1)
			}
		}
		for _, field := range []string{"allOf", "anyOf", "oneOf"} {
			if nested, ok := v[field].([]interface{}); ok {
				for i, item := range nested {
					nested[i] = removeAdditionalPropertiesWithDepth(item, depth+1)
				}
			}
		}
	case "array":
		if items, ok := v["items"].(map[string]interface{}); ok {
			v["items"] = removeAdditionalPropertiesWithDepth(items, depth+1)
		}
	}

	return v
}

func ConvertToChatOpenai(provider base.ProviderInterface, response *GeminiChatResponse, request *types.ChatCompletionRequest) (openaiResponse *types.ChatCompletionResponse, errWithCode *types.OpenAIErrorWithStatusCode) {
	openaiResponse = &types.ChatCompletionResponse{
		ID:      response.ResponseId,
		Object:  "chat.completion",
		Created: utils.GetTimestamp(),
		Model:   request.Model,
		Choices: make([]types.ChatCompletionChoice, 0, len(response.Candidates)),
	}

	if len(response.Candidates) == 0 {
		errWithCode = common.StringErrorWrapper("no candidates", "no_candidates", http.StatusInternalServerError)
		return
	}

	for _, candidate := range response.Candidates {
		if err := validateFunctionParts(candidate.Content.Parts); err != nil {
			return nil, common.ErrorWrapperLocal(err, "invalid_tool_response", 502)
		}
		openaiResponse.Choices = append(openaiResponse.Choices, candidate.ToOpenAIChoice(request))
	}

	usage := provider.GetUsage()
	*usage = ConvertOpenAIUsage(response.UsageMetadata)
	openaiResponse.Usage = usage

	return
}

// 转换为OpenAI聊天流式请求体
func (h *GeminiStreamHandler) HandlerStream(rawLine *[]byte, dataChan chan string, errChan chan error) {
	// 如果rawLine 前缀不为data:，则直接返回
	if !bytes.HasPrefix(*rawLine, []byte("data:")) {
		*rawLine = nil
		return
	}

	// 去除前缀
	*rawLine = bytes.TrimSpace((*rawLine)[5:])

	var geminiResponse GeminiChatResponse
	err := json.Unmarshal(*rawLine, &geminiResponse)
	if err != nil {
		errChan <- &requester.ProtocolError{Message: "Gemini stream: invalid event JSON", Cause: err}
		*rawLine = requester.StreamClosed
		return
	}

	aiError := errorHandle(&geminiResponse.GeminiErrorResponse, h.key)
	if aiError != nil {
		errChan <- aiError
		*rawLine = requester.StreamClosed
		return
	}

	if err := h.convertToOpenaiStream(&geminiResponse, dataChan); err != nil {
		errChan <- &requester.ProtocolError{Message: "Gemini stream: " + err.Error(), Cause: err}
		*rawLine = requester.StreamClosed
	}

}

func ConvertOpenAIUsage(geminiUsage *GeminiUsageMetadata) types.Usage {
	if geminiUsage == nil {
		return types.Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		}
	}

	usage := types.Usage{
		PromptTokens:     geminiUsage.PromptTokenCount,
		CompletionTokens: geminiUsage.CandidatesTokenCount + geminiUsage.ThoughtsTokenCount,
		TotalTokens:      geminiUsage.TotalTokenCount,

		CompletionTokensDetails: types.CompletionTokensDetails{
			ReasoningTokens: geminiUsage.ThoughtsTokenCount,
		},
	}

	for _, p := range geminiUsage.PromptTokensDetails {
		switch p.Modality {
		case "TEXT":
			usage.PromptTokensDetails.TextTokens = p.TokenCount
		case "AUDIO":
			usage.PromptTokensDetails.AudioTokens = p.TokenCount
		}
	}

	for _, c := range geminiUsage.CandidatesTokensDetails {
		switch c.Modality {
		case "TEXT":
			usage.CompletionTokensDetails.TextTokens = c.TokenCount
		case "AUDIO":
			usage.CompletionTokensDetails.AudioTokens = c.TokenCount
		case "IMAGE":
			usage.CompletionTokensDetails.ImageTokens = c.TokenCount
		}
	}

	return usage
}

func (p *GeminiProvider) pluginHandle(request *GeminiChatRequest) {
	if request.ToolConfig != nil && request.ToolConfig.FunctionCallingConfig != nil && request.ToolConfig.FunctionCallingConfig.Mode == "NONE" {
		return
	}
	if !p.UseCodeExecution {
		return
	}

	if len(request.Tools) > 0 {
		return
	}

	if p.Channel.Plugin == nil {
		return
	}

	request.Tools = append(request.Tools, GeminiChatTools{
		CodeExecution: &GeminiCodeExecution{},
	})

}
