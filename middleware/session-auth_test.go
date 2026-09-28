package middleware_test

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
	gormlogger "gorm.io/gorm/logger"

	"one-api/common/config"
	"one-api/middleware"
	"one-api/model"
)

func sessionFixture(t *testing.T, changeCookie func(map[string]any)) (*gorm.DB, *gin.Engine, *http.Cookie) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sessions.db")), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "admin", Role: config.RoleAdminUser, Status: config.UserStatusEnabled, Group: "private", AccessToken: "session-bearer-fixture"}).Error)
	values := map[string]any{"id": 1, "username": "admin", "role": config.RoleAdminUser, "status": config.UserStatusEnabled}
	if changeCookie != nil {
		changeCookie(values)
	}
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("session-auth-test-signing-key"))))
	r.GET("/fixture-login", func(c *gin.Context) {
		s := sessions.Default(c)
		for key, value := range values {
			s.Set(key, value)
		}
		require.NoError(t, s.Save())
		c.Status(http.StatusNoContent)
	})
	identity := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"allowed": true, "id": c.GetInt("id"), "username": c.GetString("username"), "role": c.GetInt("role"), "group": c.GetString("group")})
	}
	r.GET("/admin", middleware.AdminAuth(), identity)
	r.GET("/user", middleware.UserAuth(), identity)
	r.GET("/mcp/:accessToken", middleware.AdminAuth(), identity)
	r.GET("/optional", middleware.TrySetUserBySession(), identity)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/fixture-login", nil))
	require.Len(t, w.Result().Cookies(), 1)
	return db, r, w.Result().Cookies()[0]
}

func sessionRequest(t *testing.T, r http.Handler, path string, cookie *http.Cookie, bearer string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	require.NotPanics(t, func() { r.ServeHTTP(w, req) })
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	return result
}

func TestSessionAuthorizationUsesCurrentAccount(t *testing.T) {
	for _, action := range []string{"disable", "demote", "delete", "unknown_status", "database_failure"} {
		t.Run(action, func(t *testing.T) {
			db, r, cookie := sessionFixture(t, nil)
			require.Equal(t, true, sessionRequest(t, r, "/admin", cookie, "")["allowed"])
			switch action {
			case "disable":
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("status", config.UserStatusDisabled).Error)
			case "demote":
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("role", config.RoleCommonUser).Error)
			case "delete":
				require.NoError(t, db.Delete(&model.User{}, 1).Error)
			case "unknown_status":
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("status", 99).Error)
			case "database_failure":
				sqlDB, err := db.DB()
				require.NoError(t, err)
				require.NoError(t, sqlDB.Close())
			}
			require.NotEqual(t, true, sessionRequest(t, r, "/admin", cookie, "session-bearer-fixture")["allowed"])
			if action == "demote" {
				current := sessionRequest(t, r, "/user", cookie, "")
				require.Equal(t, true, current["allowed"])
				require.Equal(t, float64(config.RoleCommonUser), current["role"])
			} else {
				require.NotEqual(t, true, sessionRequest(t, r, "/user", cookie, "")["allowed"])
				require.Equal(t, float64(0), sessionRequest(t, r, "/optional", cookie, "")["id"])
			}
		})
	}
}

func TestSessionIdentityAndBearerCompatibility(t *testing.T) {
	db, r, cookie := sessionFixture(t, nil)
	require.NoError(t, db.Model(&model.User{}).Where("id = 1").Updates(map[string]any{"username": "renamed", "group": "new-private"}).Error)
	require.Equal(t, "renamed", sessionRequest(t, r, "/user", cookie, "")["username"])
	require.Equal(t, "new-private", sessionRequest(t, r, "/optional", cookie, "")["group"])
	require.Equal(t, true, sessionRequest(t, r, "/admin", nil, "session-bearer-fixture")["allowed"])
	require.Equal(t, true, sessionRequest(t, r, "/mcp/session-bearer-fixture", nil, "")["allowed"])
	require.NotEqual(t, true, sessionRequest(t, r, "/admin", nil, "invalid-fixture")["allowed"])
	require.Equal(t, float64(0), sessionRequest(t, r, "/optional", nil, "")["id"])
}

func TestMalformedSessionCannotAuthorize(t *testing.T) {
	for _, invalid := range []any{"1", 0, -1, nil} {
		t.Run("invalid_id", func(t *testing.T) {
			_, r, cookie := sessionFixture(t, func(values map[string]any) {
				if invalid == nil {
					delete(values, "id")
				} else {
					values["id"] = invalid
				}
			})
			require.NotEqual(t, true, sessionRequest(t, r, "/admin", cookie, "")["allowed"])
			require.Equal(t, float64(0), sessionRequest(t, r, "/optional", cookie, "")["id"])
		})
	}
}
