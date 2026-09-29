package relay_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/common/requester"
	"one-api/model"
	"one-api/providers/base"
	"one-api/providers/openai"
	"one-api/relay"
	"one-api/types"
)

type queryProvider struct {
	base.BaseProvider
	calls int
	send  func() (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode)
}

func (p *queryProvider) GetRequestHeaders() map[string]string { return nil }

func (p *queryProvider) CreateChatCompletion(*types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
	p.calls++
	return p.send()
}

func (p *queryProvider) CreateChatCompletionStream(*types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	panic("unexpected stream in query fixture")
}

func searchQuotaFixture(t *testing.T, unlimited bool) (*gorm.DB, *gin.Context, *queryProvider, *types.ChatCompletionRequest) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "search.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB, oldLogger, oldPricing := model.DB, logger.Logger, model.PricingInstance
	oldRedis, oldBatch, oldLogs := config.RedisEnabled, config.BatchUpdateEnabled, config.LogConsumeEnabled
	oldPre, oldThreshold, oldEncoders := config.PreConsumedQuota, config.QuotaRemindThreshold, config.DisableTokenEncoders
	model.DB, logger.Logger = db, zap.NewNop()
	config.RedisEnabled, config.BatchUpdateEnabled, config.LogConsumeEnabled = false, false, true
	config.PreConsumedQuota, config.QuotaRemindThreshold, config.DisableTokenEncoders = 20, 0, true
	model.PricingInstance = &model.Pricing{Prices: map[string]*model.Price{"query-fixture": {Type: model.TokensPriceType, Input: 1, Output: 1}}}
	t.Cleanup(func() {
		model.DB, logger.Logger, model.PricingInstance = oldDB, oldLogger, oldPricing
		config.RedisEnabled, config.BatchUpdateEnabled, config.LogConsumeEnabled = oldRedis, oldBatch, oldLogs
		config.PreConsumedQuota, config.QuotaRemindThreshold, config.DisableTokenEncoders = oldPre, oldThreshold, oldEncoders
		_ = sqlDB.Close()
	})
	viper.Set("user_token_secret", "search-quota-fixture-only")
	require.NoError(t, common.InitUserToken())
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}, &model.QuotaReservation{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "search-fixture", Quota: 1000}).Error)
	token := &model.Token{UserId: 1, RemainQuota: 1000, UnlimitedQuota: unlimited}
	require.NoError(t, db.Create(token).Error)
	channel := &model.Channel{Id: 1}
	require.NoError(t, db.Create(channel).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/fixture", nil)
	c.Set("id", 1)
	c.Set("token_id", token.Id)
	c.Set("token_unlimited_quota", unlimited)
	c.Set("channel_id", 1)
	c.Set("group_ratio", float64(1))
	c.Set("requestStartTime", time.Now())
	p := &queryProvider{BaseProvider: base.BaseProvider{Channel: channel}}
	r := &types.ChatCompletionRequest{Model: "query-fixture", Messages: []types.ChatCompletionMessage{{Role: "user", Content: strings.Repeat("query ", 20)}}}
	return db, c, p, r
}

func searchQuotaBalances(t *testing.T, db *gorm.DB, unlimited bool, spent, records int) {
	t.Helper()
	var user model.User
	var token model.Token
	require.NoError(t, db.First(&user, 1).Error)
	require.NoError(t, db.First(&token, 1).Error)
	require.Equal(t, 1000-spent, user.Quota)
	require.Equal(t, records, user.RequestCount)
	tokenSpent := spent
	if unlimited {
		tokenSpent = 0
	}
	require.Equal(t, 1000-tokenSpent, token.RemainQuota)
	require.Equal(t, tokenSpent, token.UsedQuota)
	var count int64
	require.NoError(t, db.Model(&model.Log{}).Count(&count).Error)
	require.EqualValues(t, records, count)
}

func TestSearchQuotaRejectsBeforeUpstream(t *testing.T) {
	for _, balance := range []string{"user", "token", "prompt estimate"} {
		t.Run(balance, func(t *testing.T) {
			db, c, p, r := searchQuotaFixture(t, false)
			initial, expectedCode := 19, http.StatusForbidden
			if balance == "user" {
				expectedCode = http.StatusPaymentRequired
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", initial).Error)
			} else {
				if balance == "prompt estimate" {
					initial = 21
				}
				require.NoError(t, db.Model(&model.Token{}).Where("id = 1").Update("remain_quota", initial).Error)
			}
			p.send = func() (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
				return &types.ChatCompletionResponse{}, nil
			}
			_, err := relay.ExecuteQueryForTest(c, p, r, r.Model)
			require.Zero(t, p.calls, "insufficient budget must not spend upstream")
			require.Error(t, err)
			var apiErr *types.OpenAIErrorWithStatusCode
			require.ErrorAs(t, err, &apiErr)
			require.Equal(t, expectedCode, apiErr.StatusCode)
			if balance == "user" {
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 1000).Error)
			} else {
				require.NoError(t, db.Model(&model.Token{}).Where("id = 1").Update("remain_quota", 1000).Error)
			}
			searchQuotaBalances(t, db, false, 0, 0)
		})
	}
}

func TestSearchQuotaReservationAndTerminal(t *testing.T) {
	for _, unlimited := range []bool{false, true} {
		for _, outcome := range []string{"keyword", "provider error", "no tool", "no choices", "bad arguments"} {
			t.Run(fmt.Sprintf("unlimited=%t/%s", unlimited, outcome), func(t *testing.T) {
				db, c, p, r := searchQuotaFixture(t, unlimited)
				prompt := common.CountTokenMessages(r.Messages, r.Model, p.Channel.PreCost)
				p.send = func() (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
					searchQuotaBalances(t, db, unlimited, prompt+20, 0)
					if outcome == "provider error" {
						return nil, &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusBadGateway}
					}
					*p.Usage = types.Usage{PromptTokens: 5, CompletionTokens: 2}
					if outcome == "no choices" {
						return &types.ChatCompletionResponse{}, nil
					}
					message := types.ChatCompletionMessage{Role: "assistant"}
					if outcome != "no tool" {
						args := `{"query":"fixture keyword"}`
						if outcome == "bad arguments" {
							args = "invalid"
						}
						message.ToolCalls = []*types.ChatCompletionToolCalls{{Function: &types.ChatCompletionToolCallsFunction{Name: "search", Arguments: args}}}
					}
					return &types.ChatCompletionResponse{Choices: []types.ChatCompletionChoice{{Message: message}}}, nil
				}
				keyword, err := relay.ExecuteQueryForTest(c, p, r, r.Model)
				require.Equal(t, 1, p.calls)
				if outcome == "keyword" || outcome == "no tool" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
				if outcome == "keyword" {
					require.Equal(t, "fixture keyword", keyword)
				}
				if outcome == "provider error" {
					searchQuotaBalances(t, db, unlimited, 0, 0)
				} else {
					searchQuotaBalances(t, db, unlimited, 7, 1)
				}
			})
		}
	}
}

func TestSearchQuotaMissingUsageControl(t *testing.T) {
	db, c, p, r := searchQuotaFixture(t, false)
	oldClient := requester.HTTPClient
	requester.HTTPClient = &http.Client{}
	t.Cleanup(func() { requester.HTTPClient = oldClient })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"fixture answer"}}]}`))
	}))
	defer server.Close()
	proxy := ""
	p.Channel.Proxy = &proxy
	provider := openai.CreateOpenAIProvider(p.Channel, server.URL)
	_, err := relay.ExecuteQueryForTest(c, provider, r, r.Model)
	require.NoError(t, err)
	input := common.CountTokenMessages(r.Messages, r.Model, p.Channel.PreCost)
	output := common.CountTokenText("fixture answer", r.Model)
	require.Equal(t, input, provider.GetUsage().PromptTokens)
	searchQuotaBalances(t, db, false, input+output, 1)
}

// These controls retain the existing final fee and opt-out/free behavior.
func TestSearchQuotaExistingBehavior(t *testing.T) {
	for _, mode := range []string{"normal", "no estimate", "free"} {
		t.Run(mode, func(t *testing.T) {
			db, c, p, r := searchQuotaFixture(t, false)
			spent := 7
			if mode == "no estimate" {
				p.Channel.PreCost = config.PreContNotAll
			}
			if mode == "free" {
				model.PricingInstance.Prices[r.Model] = &model.Price{Type: model.TokensPriceType}
				spent = 0
			}
			p.send = func() (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
				*p.Usage = types.Usage{PromptTokens: 5, CompletionTokens: 2}
				return &types.ChatCompletionResponse{Choices: []types.ChatCompletionChoice{{Message: types.ChatCompletionMessage{Content: "0"}}}}, nil
			}
			keyword, err := relay.ExecuteQueryForTest(c, p, r, r.Model)
			require.NoError(t, err)
			require.Empty(t, keyword)
			require.Equal(t, 1, p.calls)
			searchQuotaBalances(t, db, false, spent, 1)
		})
	}
}

func TestSearchQuotaZeroEstimateRejectsBeforeUpstream(t *testing.T) {
	for _, mode := range []string{"zero precharge", "negative precharge", "tiny times", "output only"} {
		t.Run(mode, func(t *testing.T) {
			db, c, p, r := searchQuotaFixture(t, false)
			config.PreConsumedQuota = 0
			p.Channel.PreCost = config.PreContNotAll
			price := model.PricingInstance.Prices[r.Model]
			switch mode {
			case "negative precharge":
				config.PreConsumedQuota = -1
			case "tiny times":
				price.Type = model.TimesPriceType
				price.Input = .00001
			case "output only":
				price.Input = 0
			}
			require.NoError(t, db.Model(&model.Token{}).Where("id=1").Update("remain_quota", 0).Error)
			p.send = func() (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
				return &types.ChatCompletionResponse{}, nil
			}
			_, err := relay.ExecuteQueryForTest(c, p, r, r.Model)
			require.Error(t, err)
			require.Zero(t, p.calls)
		})
	}
}

func TestSearchQuotaMissingLedgerRejectsBeforeUpstream(t *testing.T) {
	db, c, p, r := searchQuotaFixture(t, false)
	require.NoError(t, db.Migrator().DropTable(&model.QuotaReservation{}))
	p.send = func() (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
		return &types.ChatCompletionResponse{}, nil
	}
	_, err := relay.ExecuteQueryForTest(c, p, r, r.Model)
	require.Error(t, err)
	require.Zero(t, p.calls)
	searchQuotaBalances(t, db, false, 0, 0)
}
