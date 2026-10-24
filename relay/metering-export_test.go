package relay

import (
	"github.com/gin-gonic/gin"
	"one-api/providers/base"
	"one-api/types"
)

// Test-only access to the ordinary Responses reservation/settlement path.
func ExecuteResponsesForMeteringTest(c *gin.Context, provider base.ProviderInterface, request *types.OpenAIResponsesRequest) *types.OpenAIErrorWithStatusCode {
	r := &relayResponses{relayBase: relayBase{c: c, provider: provider, originalModel: request.Model, modelName: request.Model}, responsesRequest: *request}
	err, _ := RelayHandler(r)
	return err
}
