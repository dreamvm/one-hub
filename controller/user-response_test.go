package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"one-api/common/config"
	"one-api/controller"
	"one-api/middleware"
	"one-api/model"
)

func userResponseFixture(t *testing.T) (*gin.Engine, []model.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "users.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.User{}))
	users := []model.User{
		{Id: 1, Username: "root", Password: "root-password-fixture", AccessToken: "root-management-fixture", AffCode: "root", Role: config.RoleRootUser, Status: config.UserStatusEnabled, Group: "default"},
		{Id: 2, Username: "admin", Password: "admin-password-fixture", AccessToken: "admin-management-fixture", AffCode: "admin", Role: config.RoleAdminUser, Status: config.UserStatusEnabled, Group: "default"},
		{Id: 3, Username: "alice", Password: "alice-password-fixture", AccessToken: "alice-management-fixture", AffCode: "alice", Role: config.RoleCommonUser, Status: config.UserStatusEnabled, Group: "default", DisplayName: "Alice", Email: "alice@example.invalid", Quota: 12345},
	}
	require.NoError(t, db.Create(&users).Error)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("user-response-test-signing-key"))))
	r.GET("/api/user/", middleware.AdminAuth(), controller.GetUsersList)
	r.GET("/api/user/self", middleware.UserAuth(), controller.GetSelf)
	r.GET("/api/user/token", middleware.UserAuth(), controller.GenerateAccessToken)
	r.GET("/api/user/:id", middleware.AdminAuth(), controller.GetUser)
	return r, users
}

func userResponseRequest(t *testing.T, r http.Handler, path, token string) (map[string]any, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	return response, w.Body.String()
}

func TestAdminUserResponsesExcludeCredentials(t *testing.T) {
	r, users := userResponseFixture(t)
	for _, path := range []string{"/api/user/?page=1&size=10&order=id", "/api/user/?keyword=root", "/api/user/3"} {
		t.Run(path, func(t *testing.T) {
			response, body := userResponseRequest(t, r, path, users[1].AccessToken)
			require.Equal(t, true, response["success"])
			for _, user := range users {
				require.NotContains(t, body, user.Password)
				require.NotContains(t, body, user.AccessToken)
			}
			require.NotContains(t, body, `"access_token"`)
			require.NotContains(t, body, `"verification_code"`)
		})
	}
	response, body := userResponseRequest(t, r, "/api/user/1", users[0].AccessToken)
	require.Equal(t, true, response["success"])
	require.NotContains(t, body, users[0].AccessToken, "admin detail is not the owner credential endpoint")
	response, _ = userResponseRequest(t, r, "/api/user/1", users[1].AccessToken)
	require.Equal(t, false, response["success"], "detail role boundary must remain")
}

func TestAdminUserResponsePreservesPagingAndEditableFields(t *testing.T) {
	r, users := userResponseFixture(t)
	response, _ := userResponseRequest(t, r, "/api/user/?page=2&size=1&order=id", users[1].AccessToken)
	page := response["data"].(map[string]any)
	require.Equal(t, float64(3), page["total_count"])
	require.Equal(t, float64(2), page["page"])
	require.Equal(t, float64(1), page["size"])
	rows := page["data"].([]any)
	require.Len(t, rows, 1)
	require.Equal(t, "admin", rows[0].(map[string]any)["username"])
	response, _ = userResponseRequest(t, r, "/api/user/3", users[1].AccessToken)
	detail := response["data"].(map[string]any)
	for key, value := range map[string]any{"id": float64(3), "username": "alice", "display_name": "Alice", "password": "", "group": "default", "email": "alice@example.invalid", "quota": float64(12345)} {
		require.Equal(t, value, detail[key], key)
	}
}

func TestOwnerManagementTokenStillAvailableAndCanRotate(t *testing.T) {
	r, users := userResponseFixture(t)
	response, _ := userResponseRequest(t, r, "/api/user/self", users[2].AccessToken)
	require.Equal(t, users[2].AccessToken, response["data"].(map[string]any)["access_token"])
	response, _ = userResponseRequest(t, r, "/api/user/token", users[2].AccessToken)
	require.Equal(t, true, response["success"])
	newToken := response["data"].(string)
	require.NotEmpty(t, newToken)
	require.NotEqual(t, users[2].AccessToken, newToken)
	require.Nil(t, model.ValidateAccessToken("Bearer "+users[2].AccessToken))
	require.Equal(t, users[2].Id, model.ValidateAccessToken("Bearer "+newToken).Id)
	response, _ = userResponseRequest(t, r, "/api/user/self", newToken)
	require.Equal(t, newToken, response["data"].(map[string]any)["access_token"])
	root, err := model.GetUserById(1, true)
	require.NoError(t, err)
	require.Equal(t, users[0].AccessToken, root.AccessToken, "admin reads must not mutate stored credentials")
}
