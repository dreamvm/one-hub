package controller_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"one-api/common/config"
	"one-api/controller"
	"one-api/model"
)

type oauthFixtureTransport func(*http.Request) (*http.Response, error)

func (f oauthFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOAuthBindingRejectsDisabledSession(t *testing.T) {
	for _, provider := range []string{"github", "lark"} {
		for _, enabled := range []bool{false, true} {
			t.Run(provider+"/enabled="+map[bool]string{false: "false", true: "true"}[enabled], func(t *testing.T) {
				r, _ := userResponseFixture(t)
				oldGitHub, oldLark, oldProxy := config.GitHubOAuthEnabled, config.LarkAuthEnabled, config.GitHubProxy
				config.GitHubOAuthEnabled, config.LarkAuthEnabled, config.GitHubProxy = true, true, ""
				t.Cleanup(func() {
					config.GitHubOAuthEnabled, config.LarkAuthEnabled, config.GitHubProxy = oldGitHub, oldLark, oldProxy
				})
				calls := 0
				oldTransport := http.DefaultTransport
				http.DefaultTransport = oauthFixtureTransport(func(req *http.Request) (*http.Response, error) {
					calls++
					responses := map[string]string{
						"github.com/login/oauth/access_token":                         `{"access_token":"provider-fixture","scope":""}`,
						"api.github.com/user":                                         `{"id":123,"login":"linked-fixture"}`,
						"open.feishu.cn/open-apis/auth/v3/app_access_token/internal/": `{"code":0,"app_access_token":"app-fixture"}`,
						"open.feishu.cn/open-apis/authen/v1/oidc/access_token":        `{"code":0,"data":{"access_token":"user-fixture"}}`,
						"open.feishu.cn/open-apis/authen/v1/user_info":                `{"code":0,"data":{"open_id":"linked-fixture","name":"Alice"}}`,
					}
					body, ok := responses[req.URL.Host+req.URL.Path]
					require.True(t, ok, "unexpected upstream fixture request")
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
				})
				t.Cleanup(func() { http.DefaultTransport = oldTransport })
				r.GET("/oauth-session-fixture", func(c *gin.Context) {
					s := sessions.Default(c)
					s.Set("id", 3)
					s.Set("username", "alice")
					s.Set("role", config.RoleCommonUser)
					s.Set("status", config.UserStatusEnabled)
					s.Set("oauth_state", "state-fixture")
					require.NoError(t, s.Save())
					c.Status(http.StatusNoContent)
				})
				r.GET("/github", controller.GitHubOAuth)
				r.GET("/lark", controller.LarkOAuth)
				login := httptest.NewRecorder()
				r.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/oauth-session-fixture", nil))
				if !enabled {
					require.NoError(t, model.DB.Model(&model.User{}).Where("id = 3").Update("status", config.UserStatusDisabled).Error)
				}
				req := httptest.NewRequest(http.MethodGet, "/"+provider+"?state=state-fixture&code=code-fixture", nil)
				req.AddCookie(login.Result().Cookies()[0])
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				var result map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
				require.Equal(t, enabled, result["success"])
				user, err := model.GetUserById(3, true)
				require.NoError(t, err)
				if enabled {
					require.Positive(t, calls)
					if provider == "github" {
						require.Equal(t, "linked-fixture", user.GitHubId)
					} else {
						require.Equal(t, "linked-fixture", user.LarkId)
					}
				} else {
					require.Zero(t, calls, "invalid sessions must fail before exchanging the OAuth code")
					require.Empty(t, user.GitHubId)
					require.Empty(t, user.LarkId)
				}
			})
		}
	}
}
