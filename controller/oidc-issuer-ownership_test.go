package controller_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"one-api/common/config"
	"one-api/common/logger"
	oidcconfig "one-api/common/oidc"
	"one-api/controller"
	"one-api/middleware"
	"one-api/model"
)

func TestOIDCIssuerOwnership(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", "ownership-fixture"))
	require.NoError(t, err)
	for _, stage := range []string{"new registration", "same issuer login", "different issuer same subject", "legacy unknown", "legacy duplicates", "deleted ownership", "cached verifier issuer", "Google issuer alias", "explicit unbind"} {
		t.Run(stage, func(t *testing.T) {
			r, _ := userResponseFixture(t)
			oldLogger, oldTransport := logger.Logger, http.DefaultTransport
			oldEnabled, oldRegister, oldRedis := config.OIDCAuthEnabled, config.RegisterEnabled, config.RedisEnabled
			oldIssuer, oldClient, oldSecret, oldClaim, oldScopes := config.OIDCIssuer, config.OIDCClientId, config.OIDCClientSecret, config.OIDCUsernameClaims, config.OIDCScopes
			oldQuota := config.QuotaForNewUser
			logger.Logger = zap.NewNop()
			config.OIDCAuthEnabled, config.RegisterEnabled, config.RedisEnabled = true, true, false
			config.OIDCClientId, config.OIDCClientSecret, config.OIDCUsernameClaims, config.OIDCScopes = "ownership-client", "synthetic-secret", "preferred_username", "openid"
			config.QuotaForNewUser = 0
			t.Cleanup(func() {
				logger.Logger, http.DefaultTransport = oldLogger, oldTransport
				config.OIDCAuthEnabled, config.RegisterEnabled, config.RedisEnabled = oldEnabled, oldRegister, oldRedis
				config.OIDCIssuer, config.OIDCClientId, config.OIDCClientSecret, config.OIDCUsernameClaims, config.OIDCScopes = oldIssuer, oldClient, oldSecret, oldClaim, oldScopes
				config.QuotaForNewUser = oldQuota
			})
			var fixtureIssuer, fixtureToken string
			googleAlias := false
			http.DefaultTransport = oauthFixtureTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "issuer-a.fixture.invalid" && req.URL.Host != "issuer-b.fixture.invalid" && req.URL.Host != "accounts.google.com" {
					return nil, fmt.Errorf("unexpected fixture host")
				}
				var body any
				switch req.URL.Path {
				case "/.well-known/openid-configuration":
					body = map[string]any{"issuer": fixtureIssuer, "authorization_endpoint": fixtureIssuer + "/auth", "token_endpoint": fixtureIssuer + "/token", "jwks_uri": fixtureIssuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}}
				case "/keys":
					body = jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "ownership-fixture", Algorithm: "RS256", Use: "sig"}}}
				case "/token":
					body = map[string]any{"access_token": "synthetic-access", "token_type": "Bearer", "id_token": fixtureToken}
				default:
					return nil, fmt.Errorf("unexpected fixture path")
				}
				data, err := json.Marshal(body)
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(data))), Request: req}, nil
			})
			r.GET("/ownership-state", func(c *gin.Context) {
				s := sessions.Default(c)
				s.Set("oauth_state", "synthetic-state")
				require.NoError(t, s.Save())
				c.Status(204)
			})
			r.GET("/ownership-callback", controller.OIDCAuth)
			callback := func(issuer, username string, reinitialize bool) map[string]any {
				fixtureIssuer = issuer
				tokenIssuer := issuer
				if googleAlias {
					tokenIssuer = "accounts.google.com"
				}
				payload, err := json.Marshal(map[string]any{"iss": tokenIssuer, "aud": config.OIDCClientId, "sub": "shared-subject", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "preferred_username": username})
				require.NoError(t, err)
				signed, err := signer.Sign(payload)
				require.NoError(t, err)
				fixtureToken, err = signed.CompactSerialize()
				require.NoError(t, err)
				if reinitialize {
					config.OIDCIssuer = issuer
					require.NoError(t, oidcconfig.InitOIDCConfig())
				}
				start := httptest.NewRecorder()
				r.ServeHTTP(start, httptest.NewRequest("GET", "/ownership-state", nil))
				req := httptest.NewRequest("GET", "/ownership-callback?state=synthetic-state&code=synthetic-code", nil)
				req.AddCookie(start.Result().Cookies()[0])
				response := httptest.NewRecorder()
				r.ServeHTTP(response, req)
				var result map[string]any
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
				return result
			}
			issuerA := "https://issuer-a.fixture.invalid"
			if stage == "Google issuer alias" {
				issuerA = "https://accounts.google.com"
			}
			const issuerB = "https://issuer-b.fixture.invalid"
			if strings.HasPrefix(stage, "legacy") {
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 3).Update("oidc_id", "shared-subject").Error)
				if stage == "legacy duplicates" {
					require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 2).Update("oidc_id", "shared-subject").Error)
				}
				result := callback(issuerA, "newowner", true)
				require.NotEqual(t, true, result["success"], "unknown historical issuer must not be assigned by first login")
				var count int64
				require.NoError(t, model.DB.Model(&model.User{}).Count(&count).Error)
				require.EqualValues(t, 3, count)
				return
			}
			first := callback(issuerA, "issueruser", true)
			require.Equal(t, true, first["success"])
			firstID := first["data"].(map[string]any)["id"]
			if stage == "new registration" {
				return
			}
			if stage == "deleted ownership" {
				require.NoError(t, model.DB.Where("id = ?", firstID).Delete(&model.User{}).Error)
				result := callback(issuerA, "replacement", true)
				require.NotEqual(t, true, result["success"], "soft deletion must retain identity ownership pending explicit recovery")
				return
			}
			if stage == "explicit unbind" {
				existing, err := model.GetUserById(int(firstID.(float64)), true)
				require.NoError(t, err)
				r.POST("/ownership-unbind", middleware.UserAuth(), controller.Unbind)
				request := httptest.NewRequest("POST", "/ownership-unbind", strings.NewReader(`{"type":"oidc"}`))
				request.Header.Set("Authorization", "Bearer "+existing.AccessToken)
				response := httptest.NewRecorder()
				r.ServeHTTP(response, request)
				require.Contains(t, response.Body.String(), `"success":true`)
				saved, err := model.GetUserById(existing.Id, true)
				require.NoError(t, err)
				require.Empty(t, saved.OidcId)
				require.Empty(t, saved.OidcIssuer)
				require.Nil(t, saved.OidcIdentityKey)
				result := callback(issuerA, "afterunbind", true)
				require.Equal(t, true, result["success"])
				require.NotEqual(t, firstID, result["data"].(map[string]any)["id"])
				return
			}
			if stage == "Google issuer alias" {
				googleAlias = true
			}
			nextIssuer := issuerA
			reinitialize := true
			if stage == "different issuer same subject" {
				nextIssuer = issuerB
			}
			if stage == "cached verifier issuer" {
				config.OIDCIssuer = issuerB
				reinitialize = false
			}
			result := callback(nextIssuer, "anothername", reinitialize)
			require.Equal(t, true, result["success"])
			nextID := result["data"].(map[string]any)["id"]
			if stage == "different issuer same subject" {
				require.NotEqual(t, firstID, nextID, "verified issuers must not share local ownership by subject")
			}
			if stage == "same issuer login" || stage == "cached verifier issuer" || stage == "Google issuer alias" {
				require.Equal(t, firstID, nextID)
			}
		})
	}
}
