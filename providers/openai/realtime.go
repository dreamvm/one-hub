package openai

import (
	"encoding/json"
	"fmt"
	"net/http"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/common/requester"
	"one-api/types"

	"github.com/gorilla/websocket"
)

func (p *OpenAIProvider) CreateChatRealtime(modelName string) (*websocket.Conn, requester.MessageHandler, *types.OpenAIErrorWithStatusCode) {
	url, errWithCode := p.GetSupportedAPIUri(config.RelayModeChatRealtime)
	if errWithCode != nil {
		return nil, nil, errWithCode
	}
	// 获取请求地址
	fullRequestURL := p.GetFullRequestURL(url, modelName)

	// 获取请求头
	httpHeaders := make(http.Header)
	if p.IsAzure {
		httpHeaders.Set("api-key", p.Channel.Key)
	} else {
		httpHeaders.Set("Authorization", fmt.Sprintf("Bearer %s", p.Channel.Key))
	}
	httpHeaders.Set("OpenAI-Beta", "realtime=v1")

	wsRequester := requester.NewWSRequester(*p.Channel.Proxy)

	wsConn, err := wsRequester.NewRequest(fullRequestURL, httpHeaders)
	if err != nil {
		return nil, nil, common.ErrorWrapper(err, "ws_request_failed", http.StatusInternalServerError)
	}

	return wsConn, p.HandleMessage, nil
}

func (p *OpenAIProvider) HandleMessage(source requester.MessageSource, messageType int, message []byte) (bool, *types.UsageEvent, []byte, error) {
	// 处理用户消息
	if source == requester.UserMessage {
		return true, nil, nil, nil
	}

	// 确保消息类型为文本
	if messageType != websocket.TextMessage {
		return true, nil, nil, nil
	}

	// 解析事件
	var progress struct {
		Type       string          `json:"type"`
		ResponseID json.RawMessage `json:"response_id"`
	}
	if err := json.Unmarshal(message, &progress); err != nil {
		return true, nil, nil, types.NewErrorEvent("", "json_unmarshal_failed", "invalid_event", err.Error())
	}
	if isRealtimeResponseProgress(progress.Type) {
		var responseID string
		if err := json.Unmarshal(progress.ResponseID, &responseID); err != nil {
			// Work is observable even when its ID cannot be correlated. Do not
			// discard it as a parsing failure or guess an ID from another field.
			responseID = ""
		}
		return true, &types.UsageEvent{ResponseStarted: true, ResponseID: responseID}, nil, nil
	}
	var event types.Event
	if err := json.Unmarshal(message, &event); err != nil {
		return true, nil, nil, types.NewErrorEvent("", "json_unmarshal_failed", "invalid_event", err.Error())
	}

	// 处理错误事件
	if event.IsError() {
		logger.SysError("event error: " + event.Error())
		return false, nil, nil, &event
	}

	// 处理响应完成事件
	if event.Type == types.EventTypeResponseCreated {
		usage := &types.UsageEvent{ResponseStarted: true}
		if event.Response != nil {
			usage.ResponseID = event.Response.ID
		}
		return true, usage, nil, nil
	}
	if event.Type == types.EventTypeResponseDone {
		if event.Response == nil {
			return false, &types.UsageEvent{MissingUsage: true}, nil, types.NewErrorEvent("", "invalid_response", "invalid_event", "realtime response.done is missing response")
		}
		if event.Response.Usage != nil {
			event.Response.Usage.ResponseID = event.Response.ID
		} else {
			// Keep an absent report distinct from an explicit zero usage object.
			return true, &types.UsageEvent{ResponseID: event.Response.ID, MissingUsage: true}, nil, nil
		}
		return true, event.Response.Usage, nil, nil
	}

	// 处理其他事件类型
	return true, nil, nil, nil
}

func isRealtimeResponseProgress(eventType string) bool {
	switch eventType {
	case "response.text.delta", "response.text.done",
		"response.audio.delta", "response.audio.done",
		"response.audio_transcript.delta", "response.audio_transcript.done",
		"response.output_text.delta", "response.output_text.done",
		"response.output_audio.delta", "response.output_audio.done",
		"response.output_audio_transcript.delta", "response.output_audio_transcript.done",
		"response.content_part.added", "response.content_part.done",
		"response.output_item.added", "response.output_item.done",
		"response.function_call_arguments.delta", "response.function_call_arguments.done",
		"response.mcp_call_arguments.delta", "response.mcp_call_arguments.done":
		return true
	default:
		return false
	}
}
