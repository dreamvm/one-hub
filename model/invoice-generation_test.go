package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"one-api/common"
	"one-api/model"
)

func TestQuotaTransactionInvoiceMonthlyGeneration(t *testing.T) {
	db, _ := quotaTransactionFixture(t, false, false)
	oldSQLite, oldPostgres := common.UsingSQLite, common.UsingPostgreSQL
	common.UsingSQLite = db.Dialector.Name() == "sqlite"
	common.UsingPostgreSQL = db.Dialector.Name() == "postgres"
	t.Cleanup(func() { common.UsingSQLite, common.UsingPostgreSQL = oldSQLite, oldPostgres })
	// Raw aggregation names these tables explicitly; never replace existing data.
	for _, table := range []string{"statistics", "statistics_months"} {
		require.False(t, db.Migrator().HasTable(table))
	}
	require.NoError(t, db.Table("statistics").AutoMigrate(&model.Statistics{}))
	t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable("statistics")) })
	if !common.UsingPostgreSQL {
		t.Run("legacy datetime data survives migration", func(t *testing.T) {
			// Match the old production model on the engines that support datetime.
			type legacyMonth struct {
				Date             time.Time `gorm:"primary_key;type:datetime"`
				UserId           int       `gorm:"primary_key"`
				ModelName        string    `gorm:"primary_key;type:varchar(255)"`
				RequestCount     int
				Quota            int
				PromptTokens     int
				CompletionTokens int
				RequestTime      int
			}
			require.NoError(t, db.Table("statistics_months").AutoMigrate(&legacyMonth{}))
			defer func() { require.NoError(t, db.Migrator().DropTable("statistics_months")) }()
			before := model.StatisticsMonth{Date: time.Date(2026, time.September, 1, 13, 14, 15, 0, time.UTC), UserId: 1, ModelName: "legacy-synthetic", RequestCount: 2, Quota: 100, PromptTokens: 20, CompletionTokens: 4, RequestTime: 200}
			require.NoError(t, db.Table("statistics_months").Create(&before).Error)
			require.NoError(t, db.Table("statistics_months").AutoMigrate(&model.StatisticsMonth{}))
			var after model.StatisticsMonth
			require.NoError(t, db.Table("statistics_months").First(&after).Error)
			require.True(t, before.Date.Equal(after.Date))
			after.Date = before.Date
			require.Equal(t, before, after)
		})
	}
	t.Run("production monthly model migration", func(t *testing.T) {
		require.NoError(t, db.Table("statistics_months").AutoMigrate(&model.StatisticsMonth{}))
	})
	if t.Failed() {
		return
	}
	t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable("statistics_months")) })
	require.NoError(t, db.AutoMigrate(&model.StatisticsMonthGeneratedHistory{}))
	t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(&model.StatisticsMonthGeneratedHistory{})) })
	for _, row := range []map[string]any{
		{"date": "2026-09-01", "user_id": 1, "channel_id": 1, "model_name": "synthetic-a", "request_count": 2, "quota": 100, "prompt_tokens": 20, "completion_tokens": 4, "request_time": 200},
		{"date": "2026-09-30", "user_id": 1, "channel_id": 2, "model_name": "synthetic-a", "request_count": 3, "quota": 150, "prompt_tokens": 30, "completion_tokens": 6, "request_time": 300},
		{"date": "2026-09-15", "user_id": 1, "channel_id": 1, "model_name": "synthetic-b", "request_count": 1, "quota": 50, "prompt_tokens": 10, "completion_tokens": 2, "request_time": 100},
		{"date": "2026-09-15", "user_id": 2, "channel_id": 1, "model_name": "other-user", "request_count": 7, "quota": 700},
		{"date": "2026-08-31", "user_id": 1, "channel_id": 1, "model_name": "previous-month", "request_count": 8, "quota": 800},
		{"date": "2026-10-01", "user_id": 1, "channel_id": 1, "model_name": "next-month", "request_count": 9, "quota": 900},
	} {
		require.NoError(t, db.Table("statistics").Create(row).Error)
	}
	month := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.Local)
	for _, name := range []string{"normal monthly generation", "repeat preserves totals"} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, model.InsertStatisticsMonthForDate(month))
			require.True(t, model.IsStatisticsMonthGenerated(month))
			rows, err := model.GetUserInvoiceDetail(&model.StatisticsMonthDetailSearchParams{UserId: 1, Date: "2026-09-01"})
			require.NoError(t, err)
			require.Len(t, rows, 2)
			quota, count, prompt, completion, elapsed := 0, 0, 0, 0, 0
			for _, row := range rows {
				require.Contains(t, []string{"synthetic-a", "synthetic-b"}, row.ModelName)
				quota += row.Quota
				count += row.RequestCount
				prompt += row.PromptTokens
				completion += row.CompletionTokens
				elapsed += row.RequestTime
			}
			require.Equal(t, []int{300, 6, 60, 12, 600}, []int{quota, count, prompt, completion, elapsed})
		})
	}
}
