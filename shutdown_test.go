package main

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"one-api/common/config"
)

func TestShutdownUnsupportedBackgroundProfilesFailClosed(t *testing.T) {
	oldBatch, oldMCP, oldTG := config.BatchUpdateEnabled, config.MCP_ENABLE, viper.GetString("tg.bot_api_key")
	defer func() {
		config.BatchUpdateEnabled = oldBatch
		config.MCP_ENABLE = oldMCP
		viper.Set("tg.bot_api_key", oldTG)
	}()
	for _, profile := range []string{"batch", "mcp", "telegram"} {
		t.Run(profile, func(t *testing.T) {
			config.BatchUpdateEnabled = profile == "batch"
			config.MCP_ENABLE = profile == "mcp"
			viper.Set("tg.bot_api_key", "")
			if profile == "telegram" {
				viper.Set("tg.bot_api_key", "synthetic-only")
			}
			err := stopBackgroundWriters(context.Background())
			if err == nil || err.Error() != "shutdown_unsupported_background_profile" {
				t.Fatalf("unsupported profile accepted: %v", err)
			}
		})
	}
}
