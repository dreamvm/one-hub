package middleware

import (
	"errors"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"one-api/common/config"
	"one-api/model"
)

// CurrentSessionUser resolves a signed session against current database state.
// Cookie role/status snapshots and Redis caches are not authorization sources.
func CurrentSessionUser(c *gin.Context) (*model.User, error) {
	session := sessions.Default(c)
	id, ok := session.Get("id").(int)
	username, hasUsername := session.Get("username").(string)
	if !ok || id <= 0 || !hasUsername || username == "" {
		return nil, errors.New("登录状态已失效，请重新登录")
	}
	user, err := model.GetUserById(id, false)
	if err != nil || user.Username == "" || user.Status != config.UserStatusEnabled || user.Role < config.RoleCommonUser {
		return nil, errors.New("登录状态已失效，请重新登录")
	}
	return user, nil
}
