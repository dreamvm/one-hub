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
	"one-api/model"
)

func TestOIDCClaimsAndIdentity(t *testing.T) {
	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", "oidc-fixture"))
	require.NoError(t, err)
	for _, stage := range []string{"bound login", "bound subject collation", "subject case variant", "subject space variant", "bound registration disabled", "bound profile collision", "bound optional missing", "new registration", "new optional missing", "new optional null", "username collision", "collision registration disabled", "bound collision", "disabled", "registration disabled", "missing token", "object token", "numeric username", "missing username", "null username", "object username", "array username", "boolean username", "empty username", "blank username", "bad email", "bad display", "bad avatar", "numeric state", "wrong signature", "wrong issuer", "wrong audience", "expired"} {
		t.Run(stage, func(t *testing.T) {
			r, _ := userResponseFixture(t)
			oldEnabled, oldRegister := config.OIDCAuthEnabled, config.RegisterEnabled
			oldIssuer, oldID, oldSecret, oldClaim, oldScopes := config.OIDCIssuer, config.OIDCClientId, config.OIDCClientSecret, config.OIDCUsernameClaims, config.OIDCScopes
			oldQuota, oldRedis := config.QuotaForNewUser, config.RedisEnabled
			config.OIDCAuthEnabled, config.RegisterEnabled = true, true
			config.OIDCIssuer, config.OIDCClientId, config.OIDCClientSecret, config.OIDCUsernameClaims, config.OIDCScopes = "https://oidc.fixture.invalid", "fixture-client", "fixture-client-secret", "preferred_username", "openid,profile,email"
			config.QuotaForNewUser, config.RedisEnabled = 0, false
			oldTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = oldTransport
				config.OIDCAuthEnabled, config.RegisterEnabled = oldEnabled, oldRegister
				config.OIDCIssuer, config.OIDCClientId, config.OIDCClientSecret, config.OIDCUsernameClaims, config.OIDCScopes = oldIssuer, oldID, oldSecret, oldClaim, oldScopes
				config.QuotaForNewUser, config.RedisEnabled = oldQuota, oldRedis
			})
			claims := map[string]any{"iss": config.OIDCIssuer, "aud": config.OIDCClientId, "sub": "new-subject", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "preferred_username": "newuser", "email": "new@example.invalid", "displayName": "New User", "avatar": "https://fixture.invalid/avatar"}
			switch stage {
			case "bound login", "bound subject collation", "subject case variant", "subject space variant", "bound registration disabled", "bound profile collision", "bound optional missing", "disabled":
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = 3").Update("oidc_id", "bound-subject").Error)
				claims["sub"] = "bound-subject"
				claims["preferred_username"] = "renamed"
				if stage == "bound registration disabled" {
					config.RegisterEnabled = false
				}
				if stage == "bound subject collation" || stage == "subject case variant" || stage == "subject space variant" {
					var ddl string
					require.NoError(t, model.DB.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'users'").Scan(&ddl).Error)
					require.Contains(t, ddl, "`oidc_id` text")
					collation := "NOCASE"
					if stage == "subject space variant" {
						collation = "RTRIM"
					}
					require.NoError(t, model.DB.Exec("ALTER TABLE users RENAME TO oidc_original_users").Error)
					ddl = strings.Replace(ddl, "`oidc_id` text", "`oidc_id` text COLLATE "+collation, 1)
					require.NoError(t, model.DB.Exec(ddl).Error)
					require.NoError(t, model.DB.Exec("INSERT INTO users SELECT * FROM oidc_original_users").Error)
					if stage == "subject case variant" {
						claims["sub"] = "BOUND-SUBJECT"
					}
					if stage == "subject space variant" {
						claims["sub"] = "bound-subject "
					}
				}

				if stage == "bound profile collision" {
					claims["preferred_username"] = "root"
				}
				if stage == "bound optional missing" {
					delete(claims, "email")
					delete(claims, "displayName")
					delete(claims, "avatar")
				}
				if stage == "disabled" {
					require.NoError(t, model.DB.Model(&model.User{}).Where("id = 3").Update("status", config.UserStatusDisabled).Error)
				}
			case "username collision", "collision registration disabled":
				claims["preferred_username"] = "root"
				if stage == "collision registration disabled" {
					config.RegisterEnabled = false
				}
			case "bound collision":
				claims["preferred_username"] = "alice"
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = 3").Update("oidc_id", "original-subject").Error)
			case "registration disabled":
				config.RegisterEnabled = false
			case "new optional missing":
				delete(claims, "email")
				delete(claims, "displayName")
				delete(claims, "avatar")
			case "new optional null":
				claims["email"] = nil
				claims["displayName"] = nil
				claims["avatar"] = nil
			case "missing username":
				delete(claims, "preferred_username")
			case "null username":
				claims["preferred_username"] = nil
			case "object username":
				claims["preferred_username"] = map[string]string{"name": "root"}
			case "array username":
				claims["preferred_username"] = []string{"root"}
			case "boolean username":
				claims["preferred_username"] = true
			case "empty username":
				claims["preferred_username"] = ""
			case "wrong issuer":
				claims["iss"] = "https://other.fixture.invalid"
			case "wrong audience":
				claims["aud"] = "other-client"
			case "expired":
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			case "numeric username":
				claims["preferred_username"] = 17
			case "blank username":
				claims["preferred_username"] = "  "
			case "bad email":
				claims["email"] = []string{"invalid"}
			case "bad display":
				claims["displayName"] = true
			case "bad avatar":
				claims["avatar"] = map[string]string{"url": "invalid"}
			}
			payload, err := json.Marshal(claims)
			require.NoError(t, err)
			signed, err := signer.Sign(payload)
			require.NoError(t, err)
			token, err := signed.CompactSerialize()
			require.NoError(t, err)
			if stage == "wrong signature" {
				token = token[:len(token)-8] + "AAAAAAAA"
			}
			exchangeCalls := 0
			http.DefaultTransport = oauthFixtureTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "oidc.fixture.invalid" {
					return nil, fmt.Errorf("unexpected OIDC fixture host")
				}
				var body any
				switch req.URL.Path {
				case "/.well-known/openid-configuration":
					body = map[string]any{"issuer": config.OIDCIssuer, "authorization_endpoint": config.OIDCIssuer + "/auth", "token_endpoint": config.OIDCIssuer + "/token", "jwks_uri": config.OIDCIssuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}}
				case "/keys":
					body = jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "oidc-fixture", Algorithm: "RS256", Use: "sig"}}}
				case "/token":
					exchangeCalls++
					data := map[string]any{"access_token": "fixture-access", "token_type": "Bearer", "id_token": token}
					if stage == "missing token" {
						delete(data, "id_token")
					}
					if stage == "object token" {
						data["id_token"] = map[string]string{"bad": "type"}
					}
					body = data
				default:
					return nil, fmt.Errorf("unexpected OIDC fixture path")
				}
				data, err := json.Marshal(body)
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(data))), Request: req}, nil
			})
			require.NoError(t, oidcconfig.InitOIDCConfig())
			r.GET("/oidc-state", func(c *gin.Context) {
				s := sessions.Default(c)
				var state any = "state-fixture"
				if stage == "numeric state" {
					state = 17
				}
				s.Set("oauth_state", state)
				require.NoError(t, s.Save())
				c.Status(204)
			})
			r.GET("/oidc-callback", controller.OIDCAuth)
			start := httptest.NewRecorder()
			r.ServeHTTP(start, httptest.NewRequest("GET", "/oidc-state", nil))
			request := httptest.NewRequest("GET", "/oidc-callback?state=state-fixture&code=fixture-code", nil)
			request.AddCookie(start.Result().Cookies()[0])
			response := httptest.NewRecorder()
			require.NotPanics(t, func() { r.ServeHTTP(response, request) })
			good := strings.HasPrefix(stage, "bound ") && stage != "bound collision" || stage == "new registration" || strings.HasPrefix(stage, "new optional")
			var result map[string]any
			_ = json.Unmarshal(response.Body.Bytes(), &result)
			if good {
				require.Equal(t, true, result["success"])
				if strings.HasPrefix(stage, "bound ") {
					require.EqualValues(t, 3, result["data"].(map[string]any)["id"])
					sessionRequest := httptest.NewRequest("GET", "/api/user/self", nil)
					for _, c := range response.Result().Cookies() {
						sessionRequest.AddCookie(c)
					}
					sessionResponse := httptest.NewRecorder()
					r.ServeHTTP(sessionResponse, sessionRequest)
					var identity map[string]any
					require.NoError(t, json.Unmarshal(sessionResponse.Body.Bytes(), &identity))
					require.Equal(t, true, identity["success"])
					require.EqualValues(t, 3, identity["data"].(map[string]any)["id"])
				}
			} else {
				require.NotEqual(t, true, result["success"])
			}
			var root, alice model.User
			require.NoError(t, model.DB.First(&root, 1).Error)
			require.NoError(t, model.DB.First(&alice, 3).Error)
			require.Empty(t, root.OidcId)
			if stage == "bound collision" {
				require.Equal(t, "original-subject", alice.OidcId)
			}
			var count int64
			require.NoError(t, model.DB.Model(&model.User{}).Count(&count).Error)
			expected := int64(3)
			if stage == "new registration" || strings.HasPrefix(stage, "new optional") {
				expected = 4
			}
			require.Equal(t, expected, count)
			if stage == "numeric state" {
				require.Zero(t, exchangeCalls)
			}
		})
	}
}
