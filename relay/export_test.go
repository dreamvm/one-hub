package relay

import (
	"github.com/gin-gonic/gin"
	"one-api/providers/base"
	"one-api/types"
)

var ExecuteQueryForTest = executeQuery

// Exercise the ordinary chat reservation/terminal path with an isolated provider.
func ExecuteChatForTest(c *gin.Context, provider base.ProviderInterface, request *types.ChatCompletionRequest) *types.OpenAIErrorWithStatusCode {
	r := &relayChat{relayBase: relayBase{c: c, provider: provider, originalModel: request.Model, modelName: request.Model}, chatRequest: *request}
	err, _ := RelayHandler(r)
	return err
}
