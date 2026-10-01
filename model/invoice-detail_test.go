package model_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/common"
	"one-api/model"
)

// Runs only against the existing opt-in, disposable three-database fixtures.
func TestQuotaTransactionInvoiceDetail(t *testing.T) {
	db, _ := quotaTransactionFixture(t, false, false)
	oldSQLite, oldPostgres := common.UsingSQLite, common.UsingPostgreSQL
	common.UsingSQLite = db.Dialector.Name() == "sqlite"
	common.UsingPostgreSQL = db.Dialector.Name() == "postgres"
	t.Cleanup(func() { common.UsingSQLite, common.UsingPostgreSQL = oldSQLite, oldPostgres })
	// The production reader explicitly names this table, so do not apply the
	// shared fixture's per-test table prefix. Refuse to replace existing data.
	require.False(t, db.Migrator().HasTable("statistics_months"))
	// Use SQL DATE supported by all fixture engines. The legacy production
	// model's explicit datetime tag is not a portable PostgreSQL fixture DDL.
	require.NoError(t, db.Exec(`CREATE TABLE statistics_months (
		date DATE NOT NULL, user_id INTEGER NOT NULL, model_name VARCHAR(255) NOT NULL,
		request_count INTEGER, quota INTEGER, prompt_tokens INTEGER,
		completion_tokens INTEGER, request_time INTEGER,
		PRIMARY KEY (date, user_id, model_name)
	)`).Error)
	t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable("statistics_months")) })
	for _, row := range []map[string]any{
		{"date": "2026-09-01", "user_id": 1, "model_name": "synthetic-a", "request_count": 2, "quota": 500000, "prompt_tokens": 120, "completion_tokens": 30, "request_time": 2000},
		{"date": "2026-09-01", "user_id": 1, "model_name": "synthetic-b", "request_count": 1, "quota": 250000, "prompt_tokens": 40, "completion_tokens": 10, "request_time": 1000},
		{"date": "2026-09-01", "user_id": 2, "model_name": "other-user", "quota": 9000000},
		{"date": "2026-08-01", "user_id": 1, "model_name": "other-month", "quota": 8000000},
	} {
		require.NoError(t, db.Table("statistics_months").Create(row).Error)
	}
	params := model.StatisticsMonthDetailSearchParams{UserId: 1, Date: "2026-09-01"}
	t.Run("normal totals and scope", func(t *testing.T) {
		rows, err := model.GetUserInvoiceDetail(&params)
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
		require.Equal(t, []int{750000, 3, 160, 40, 3000}, []int{quota, count, prompt, completion, elapsed})
	})
	t.Run("returned date identifies selected month", func(t *testing.T) {
		rows, err := model.GetUserInvoiceDetail(&params)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		for _, row := range rows {
			// MySQL may expose its datetime suffix; the selected date must remain.
			require.True(t, strings.HasPrefix(row.Date, params.Date), "missing or wrong date: %q", row.Date)
		}
	})
	t.Run("empty month and invalid input", func(t *testing.T) {
		rows, err := model.GetUserInvoiceDetail(&model.StatisticsMonthDetailSearchParams{UserId: 1, Date: "2026-07-01"})
		require.NoError(t, err)
		require.Empty(t, rows)
		for _, invalid := range []model.StatisticsMonthDetailSearchParams{{UserId: 0, Date: params.Date}, {UserId: 1, Date: "2026-13-01"}} {
			_, err := model.GetUserInvoiceDetail(&invalid)
			require.Error(t, err)
		}
	})
}
