package controller_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/common"
	"one-api/common/config"
	"one-api/controller"
	"one-api/middleware"
	"one-api/model"
)

func TestOIDCBindingSurvivesStaleProfileUpdate(t *testing.T) {
	for _, password := range []bool{false, true} {
		for _, binding := range []string{"", "replacement-subject", "original-subject"} {
			t.Run(fmt.Sprintf("password=%v/binding=%s", password, binding), func(t *testing.T) {
				_, users := userResponseFixture(t)
				oldRedis := config.RedisEnabled
				config.RedisEnabled = false
				t.Cleanup(func() { config.RedisEnabled = oldRedis })
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", users[2].Id).Update("oidc_id", "original-subject").Error)
				stale, err := model.GetUserById(users[2].Id, true)
				require.NoError(t, err)
				// Deterministic interleaving: a separate authorized binding operation
				// completes after the profile reader and before its generic write.
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", users[2].Id).Update("oidc_id", binding).Error)
				stale.DisplayName = "Updated profile"
				if password {
					stale.Password = "synthetic-password"
				}
				require.NoError(t, stale.Update(password))
				saved, err := model.GetUserById(users[2].Id, true)
				require.NoError(t, err)
				require.Equal(t, "Updated profile", saved.DisplayName)
				require.Equal(t, binding, saved.OidcId, "generic profile updates must not restore or replace a separately changed binding")
				if password {
					require.True(t, common.ValidatePasswordAndHash("synthetic-password", saved.Password))
				} else {
					require.Equal(t, users[2].Password, saved.Password)
				}
			})
		}
	}
}

func TestOIDCUnbindSurvivesLaterProfileWrite(t *testing.T) {
	r, users := userResponseFixture(t)
	oldRedis := config.RedisEnabled
	config.RedisEnabled = false
	t.Cleanup(func() { config.RedisEnabled = oldRedis })
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", users[2].Id).Update("oidc_id", "original-subject").Error)
	stale, err := model.GetUserById(users[2].Id, true)
	require.NoError(t, err)
	r.POST("/api/user/unbind", middleware.UserAuth(), controller.Unbind)
	req := httptest.NewRequest(http.MethodPost, "/api/user/unbind", strings.NewReader(`{"type":"oidc"}`))
	req.Header.Set("Authorization", "Bearer "+users[2].AccessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"success":true`)
	stale.DisplayName = "After unbind"
	require.NoError(t, stale.Update(false))
	saved, err := model.GetUserById(users[2].Id, true)
	require.NoError(t, err)
	require.Empty(t, saved.OidcId)
	require.Equal(t, "After unbind", saved.DisplayName)
}
