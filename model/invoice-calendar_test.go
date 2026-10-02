package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"one-api/common"
	"one-api/model"
)

func TestQuotaTransactionInvoicePreviousMonth(t *testing.T) {
	for _, tc := range []struct{ today, want string }{
		{"2026-10-01", "2026-09-01"},
		{"2026-01-15", "2025-12-01"},
		{"2026-03-28", "2026-02-01"},
		{"2024-03-29", "2024-02-01"},
		{"2026-03-29", "2026-02-01"},
		{"2026-03-30", "2026-02-01"},
		{"2026-03-31", "2026-02-01"},
		{"2024-03-30", "2024-02-01"},
		{"2024-03-31", "2024-02-01"},
		{"2026-05-31", "2026-04-01"},
		{"2026-10-31", "2026-09-01"},
		{"2026-12-31", "2026-11-01"},
	} {
		t.Run(tc.today, func(t *testing.T) {
			db, _ := quotaTransactionFixture(t, false, false)
			oldSQLite, oldPostgres := common.UsingSQLite, common.UsingPostgreSQL
			common.UsingSQLite, common.UsingPostgreSQL = db.Dialector.Name() == "sqlite", db.Dialector.Name() == "postgres"
			t.Cleanup(func() { common.UsingSQLite, common.UsingPostgreSQL = oldSQLite, oldPostgres })
			for _, name := range []string{"statistics", "statistics_months"} {
				require.False(t, db.Migrator().HasTable(name))
			}
			require.NoError(t, db.Table("statistics").AutoMigrate(&model.Statistics{}))
			require.NoError(t, db.Table("statistics_months").AutoMigrate(&model.StatisticsMonth{}))
			require.NoError(t, db.AutoMigrate(&model.StatisticsMonthGeneratedHistory{}))
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&model.StatisticsMonthGeneratedHistory{}, "statistics_months", "statistics"))
			})
			now, err := time.ParseInLocation("2006-01-02", tc.today, time.Local)
			require.NoError(t, err)
			want, err := time.ParseInLocation("2006-01-02", tc.want, time.Local)
			require.NoError(t, err)
			for _, row := range []map[string]any{
				{"date": tc.want, "user_id": 1, "channel_id": 1, "model_name": "last-month", "quota": 100, "request_count": 1},
				{"date": tc.today, "user_id": 1, "channel_id": 1, "model_name": "current-month", "quota": 900, "request_count": 1},
			} {
				require.NoError(t, db.Table("statistics").Create(row).Error)
			}
			for attempt := 0; attempt < 2; attempt++ {
				require.NoError(t, model.InsertStatisticsMonthAtForTest(now))
				assert.True(t, model.IsStatisticsMonthGenerated(want), "expected previous month %s", tc.want)
				var rows []model.StatisticsMonth
				require.NoError(t, db.Table("statistics_months").Find(&rows).Error)
				require.Len(t, rows, 1)
				assert.Equal(t, tc.want, rows[0].Date.Format("2006-01-02"))
				assert.Equal(t, "last-month", rows[0].ModelName)
				assert.Equal(t, 100, rows[0].Quota)
				var histories int64
				require.NoError(t, db.Model(&model.StatisticsMonthGeneratedHistory{}).Count(&histories).Error)
				assert.EqualValues(t, 1, histories, "repeat should use existing generated history")
			}
		})
	}
}
