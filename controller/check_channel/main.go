package check_channel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"one-api/common/config"
	"one-api/model"
	"one-api/providers"
	providers_base "one-api/providers/base"
	"one-api/types"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	CheckStatusFailed  = 0
	CheckStatusSuccess = 1
	CheckStatusUnknown = 2
)

type CheckChannel struct {
	Models        []string       `json:"models"`
	Channel       *model.Channel `json:"-"`
	ChatInterface providers_base.ChatInterface
}

type ModelResult struct {
	Model   string                `json:"model"`
	Process []*CheckProcessResult `json:"process"`
}

type CheckProcessResult struct {
	Name     string                        `json:"name"`
	Results  []*CheckResult                `json:"results"`
	Response *types.ChatCompletionResponse `json:"response"`
}

type CheckResult struct {
	Name   string `json:"name"`
	Status int    `json:"status"`
	Remark string `json:"remark"`
}

type CheckProcess interface {
	GetName() string
	GetRequest() *types.ChatCompletionRequest
	Check(req *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult
}

func CreateCheckChannel(ctx context.Context, channelId int, models string) (*CheckChannel, error) {
	modelsList := strings.Split(models, ",")
	if len(modelsList) == 0 {
		return nil, errors.New("models is empty")
	}

	channel, err := model.GetChannelById(channelId)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	provider := providers.GetProvider(channel, c)
	if provider == nil {
		return nil, errors.New("channel not implemented")
	}
	if requester := provider.GetRequester(); requester != nil {
		requester.Context = ctx
	}

	usage := &types.Usage{
		PromptTokens:     0,
		CompletionTokens: 0,
	}
	provider.SetUsage(usage)

	chatInterface, ok := provider.(providers_base.ChatInterface)
	if !ok {
		return nil, errors.New("channel not implemented")
	}

	return &CheckChannel{
		Models:        modelsList,
		Channel:       channel,
		ChatInterface: chatInterface,
	}, nil
}

func (c *CheckChannel) Run() ([]*ModelResult, error) {
	results := make([]*ModelResult, 0)

	for _, model := range c.Models {
		process := getProcess(model)
		modelResult := &ModelResult{
			Model:   model,
			Process: make([]*CheckProcessResult, 0),
		}
		for _, p := range process {
			processResult := &CheckProcessResult{
				Name:     p.GetName(),
				Results:  make([]*CheckResult, 0),
				Response: nil,
			}
			req := p.GetRequest()
			resp, err := c.ChatInterface.CreateChatCompletion(req)
			var openaiErr *types.OpenAIError
			if err != nil {
				openaiErr = &err.OpenAIError
			}
			processResult.Results = p.Check(req, resp, openaiErr)
			processResult.Response = resp

			modelResult.Process = append(modelResult.Process, processResult)
		}
		results = append(results, modelResult)
	}
	return results, nil
}

func getProcess(modelName string) []CheckProcess {
	return []CheckProcess{
		CreateCheckBaseProcess(modelName),
		CreateCheckErrorProcess(modelName),
		CreateCheckImgProcess(modelName),
		CreateCheckJsonFormatProcess(modelName),
		CreateCheckToolProcess(modelName),
	}
}

// 仅支持OpenAI、Anthropic、Gemini
func getChannelTypeByModelName(modelName string) int {
	if strings.HasPrefix(modelName, "gpt-") || strings.HasPrefix(modelName, "chatgpt-") || strings.HasPrefix(modelName, "o1-") {
		if strings.Contains(modelName, "-all") || strings.Contains(modelName, "-realtime") || strings.Contains(modelName, "-instruct") {
			return 0
		}
		return config.ChannelTypeOpenAI
	}

	if strings.HasPrefix(modelName, "claude-") {
		return config.ChannelTypeAnthropic
	}

	if strings.HasPrefix(modelName, "gemini-") {
		return config.ChannelTypeGemini
	}

	return 0
}

func (c *CheckChannel) RunStream(ctx context.Context, resultChan chan<- *ModelResult) {
	defer close(resultChan)

	for _, model := range c.Models {
		if ctx.Err() != nil {
			return
		}
		process := getProcess(model)
		modelResult := &ModelResult{
			Model:   model,
			Process: make([]*CheckProcessResult, 0),
		}

		for _, p := range process {
			if ctx.Err() != nil {
				return
			}
			processResult := &CheckProcessResult{
				Name:     p.GetName(),
				Results:  make([]*CheckResult, 0),
				Response: nil,
			}
			req := p.GetRequest()
			resp, err := c.ChatInterface.CreateChatCompletion(req)
			var openaiErr *types.OpenAIError
			if err != nil {
				openaiErr = &err.OpenAIError
			}
			processResult.Results = p.Check(req, resp, openaiErr)
			processResult.Response = resp

			modelResult.Process = append(modelResult.Process, processResult)
		}

		select {
		case resultChan <- modelResult:
		case <-ctx.Done():
			return
		}
	}
}
