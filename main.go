package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"one-api/cli"
	"one-api/common"
	"one-api/common/cache"
	"one-api/common/config"
	"one-api/common/lifecycle"
	"one-api/common/logger"
	"one-api/common/notify"
	"one-api/common/oidc"
	"one-api/common/redis"
	"one-api/common/requester"
	"one-api/common/scheduler"
	"one-api/common/search"
	"one-api/common/storage"
	"one-api/common/telegram"
	"one-api/common/webauthn"
	"one-api/controller"
	"one-api/cron"
	"one-api/middleware"
	"one-api/model"
	"one-api/relay/task"
	"one-api/router"
	"one-api/safty"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

//go:embed web/build
var buildFS embed.FS

//go:embed web/build/index.html
var indexPage []byte

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		logger.SysError(err.Error())
		// Incomplete work is not joined by os.Exit. Do not defer CloseDB or emit
		// shutdown_complete on this path; the maintenance gate must reject it.
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cli.InitCli()
	config.InitConf()
	if viper.GetString("log_level") == "debug" {
		config.Debug = true
	}

	logger.SetupLogger()
	logger.SysLog("One Hub " + config.Version + " started")

	// Initialize user token
	err := common.InitUserToken()
	if err != nil {
		logger.FatalLog("failed to initialize user token: " + err.Error())
	}

	// Initialize SQL Database
	model.SetupDB()
	// Initialize Redis
	redis.InitRedisClient()
	cache.InitCacheManager()
	// Initialize options
	model.InitOptionMap()
	// Initialize oidc
	oidc.InitOIDCConfig()
	// Initialize wenauthn
	webauthn.InitWebAuthn()
	model.NewPricing()
	model.HandleOldTokenMaxId()
	common.SafeGoroutine(func() { model.RunQuotaReservationRecovery(lifecycle.Background.Context()) })

	initMemoryCache()
	initSync()

	common.InitTokenEncoders()
	requester.InitHttpClient()
	// Initialize Telegram bot
	telegram.InitTelegramBot()

	controller.InitMidjourneyTask()
	task.InitTask()
	notify.InitNotifier()
	cron.InitCron()
	storage.InitStorage()
	search.InitSearcher()
	// 初始化安全检查器
	safty.InitSaftyTools()
	// 初始化账单数据
	if config.UserInvoiceMonth {
		logger.SysLog("Enable User Invoice Monthly Data")
		common.SafeGoroutine(func() {
			if model.InsertStatisticsMonth() != nil {
				lifecycle.Background.Fail()
			}
		})
	}
	return initHttpServer(ctx)
}

func initMemoryCache() {
	if viper.GetBool("memory_cache_enabled") {
		config.MemoryCacheEnabled = true
	}

	if !config.MemoryCacheEnabled {
		return
	}

	syncFrequency := viper.GetInt("sync_frequency")
	model.TokenCacheSeconds = syncFrequency

	logger.SysLog("memory cache enabled")
	logger.SysLog(fmt.Sprintf("sync frequency: %d seconds", syncFrequency))
	common.SafeGoroutine(func() { model.SyncOptions(syncFrequency) })
	common.SafeGoroutine(func() { SyncChannelCache(syncFrequency) })
}

func initSync() {
	// go controller.AutomaticallyUpdateChannels(viper.GetInt("channel.update_frequency"))
	common.SafeGoroutine(func() { controller.AutomaticallyTestChannels(viper.GetInt("channel.test_frequency")) })
}

func initHttpServer(ctx context.Context) error {
	if viper.GetString("gin_mode") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	server := gin.New()
	server.Use(middleware.RequestId())
	middleware.SetUpLogger(server)

	trustedHeader := viper.GetString("trusted_header")
	if trustedHeader != "" {
		server.TrustedPlatform = trustedHeader
	}

	store := cookie.NewStore([]byte(config.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   2592000, // 30 days
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})
	server.Use(sessions.Sessions("session", store))

	router.SetRouter(server, buildFS, indexPage)
	port := viper.GetString("port")

	timeout := viper.GetDuration("shutdown_timeout")
	if timeout <= 0 || timeout > 2*time.Minute {
		return errors.New("shutdown_invalid_timeout")
	}
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return errors.New("http_listen_failed")
	}
	err = lifecycle.Serve(ctx, listener, server, timeout, stopBackgroundWriters,
		model.ShutdownAccounting, model.CloseDB)
	if err == nil {
		logger.SysLog("shutdown_complete: requests_drained workers_joined accounting_settled database_closed")
	}
	return err
}

func stopBackgroundWriters(ctx context.Context) error {
	// These optional profiles have no trustworthy join/error contract yet.
	// Refuse a success marker; do not silently skip buffered or external work.
	if config.BatchUpdateEnabled || config.MCP_ENABLE || viper.GetString("tg.bot_api_key") != "" {
		return errors.New("shutdown_unsupported_background_profile")
	}
	if scheduler.Manager == nil {
		return errors.New("shutdown_scheduler_unavailable")
	}
	if err := scheduler.Manager.Shutdown(ctx); err != nil {
		return err
	}
	return lifecycle.Background.Stop(ctx)
}

func SyncChannelCache(frequency int) {
	// 只有 从 服务器端获取数据的时候才会用到
	if config.IsMasterNode {
		logger.SysLog("master node does't synchronize the channel")
		return
	}
	for lifecycle.Background.Sleep(time.Duration(frequency) * time.Second) {
		logger.SysLog("syncing channels from database")
		model.ChannelGroup.Load()
		model.GlobalUserGroupRatio.Load()
		model.PricingInstance.Init()
		model.ModelOwnedBysInstance.Load()
	}
}
