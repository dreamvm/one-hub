package middleware

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"one-api/common/logger"
	"one-api/metrics"
)

func SetUpLogger(server *gin.Engine) {
	// Gin redirects before invoking middleware. Its debug redirect message also
	// needs protection without changing redirect behavior or callback parameters.
	gin.DebugPrintFunc = func(format string, values ...any) {
		if strings.HasPrefix(format, "redirecting request ") {
			logger.Logger.Debug("Gin request redirect (URLs omitted)")
			return
		}
		logger.Logger.Debug(fmt.Sprintf(format, values...))
	}
	server.Use(safeRequestRecovery(), GinzapWithConfig())
}

func safeRequestRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if value := recover(); value != nil {
				logRequestPanic(c)
				metrics.RecordPanic("http")
				var networkError *net.OpError
				var syscallError *os.SyscallError
				if err, ok := value.(error); ok && errors.As(err, &networkError) && errors.As(networkError, &syscallError) &&
					(errors.Is(syscallError, syscall.EPIPE) || errors.Is(syscallError, syscall.ECONNRESET)) {
					// Preserve Gin's no-write behavior for a disconnected client,
					// but retain the safe diagnostic above and never attach the raw error.
					c.Abort()
					return
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func logRequestPanic(c *gin.Context) {
	// SysError preserves the root dashboard's log history as well as file/stderr
	// diagnostics. Stack frames are retained; panic values and request dumps are not.
	logger.SysError(fmt.Sprintf("HTTP handler panic: method=%s route=%s request_id=%s\n%s",
		c.Request.Method, requestLogPath(c), c.GetString(logger.RequestIdKey), zap.Stack("stack").String))
}

func requestLogPath(c *gin.Context) string {
	if path := c.FullPath(); path != "" {
		return path
	}
	// Disabled integrations and wrong-method requests can still contain tokens.
	return "[unmatched]"
}

func GinzapWithConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := requestLogPath(c)
		// Do not copy query values: OAuth, verification and payment parameters
		// can contain secrets in addition to the model API's key parameter.
		hasQuery := c.Request.URL.RawQuery != ""
		c.Next()
		end := time.Now()
		latency := end.Sub(start)
		requestID := c.GetString(logger.RequestIdKey)
		userID := c.GetInt("id")

		fields := []zapcore.Field{
			zap.Int("status", c.Writer.Status()),
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Bool("has_query", hasQuery),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.Duration("latency", latency),
			zap.Int("user_id", userID),
			zap.String("original_model", c.GetString("original_model")),
			zap.String("new_model", c.GetString("new_model")),
			zap.Int("token_id", c.GetInt("token_id")),
			zap.String("token_name", c.GetString("token_name")),
			zap.Int("channel_id", c.GetInt("channel_id")),
		}

		if len(c.Errors) > 0 {
			// Error strings may contain an upstream URL or a copy of the request.
			fields = append(fields, zap.Int("error_count", len(c.Errors)))
			logger.Logger.Error("GIN request failed", fields...)
		} else {
			logger.Logger.Info("GIN request", fields...)
		}
		metrics.RecordHttp(c, latency)
	}
}
