package router

import (
	"embed"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"net/http"
	"one-api/controller"
	"one-api/middleware"
	"strings"
)

func SetWebRouter(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.Use(limitStaticRanges)
	// 特别处理favicon.ico请求
	router.GET("/favicon.ico", controller.Favicon(buildFS))

	embedFS, err := static.EmbedFolder(buildFS, "web/build")
	if err != nil {
		panic("加载嵌入式资源失败")
	}
	router.Use(static.Serve("/", embedFS))

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
	})
}

// Bound Range parsing before the embedded file server. API routes registered
// before SetWebRouter keep their own protocol semantics.
func limitStaticRanges(c *gin.Context) {
	values := c.Request.Header.Values("Range")
	if len(values) > 1 || (len(values) == 1 && (len(values[0]) > 1024 || strings.Count(values[0], ",") >= 16)) {
		c.AbortWithStatus(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	c.Next()
}
