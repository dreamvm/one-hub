package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"one-api/common"
	"one-api/model"
)

func TestInvoiceMonthlyCommitFailure(t *testing.T) {
	for _, reject := range []bool{false, true} {
		name := "normal commit"
		if reject {
			name = "deferred constraint rejects commit"
		}
		t.Run(name, func(t *testing.T) {
			db, _ := quotaTransactionFixture(t, false, false)
			if db.Dialector.Name() != "sqlite" {
				t.Skip("real deferred-constraint fixture uses SQLite")
			}
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			oldSQLite, oldPostgres := common.UsingSQLite, common.UsingPostgreSQL
			common.UsingSQLite, common.UsingPostgreSQL = true, false
			t.Cleanup(func() { common.UsingSQLite, common.UsingPostgreSQL = oldSQLite, oldPostgres })
			for _, table := range []string{"statistics", "statistics_months", "invoice_commit_parent", "invoice_commit_child"} {
				require.False(t, db.Migrator().HasTable(table))
			}
			require.NoError(t, db.Table("statistics").AutoMigrate(&model.Statistics{}))
			require.NoError(t, db.Table("statistics_months").AutoMigrate(&model.StatisticsMonth{}))
			require.NoError(t, db.AutoMigrate(&model.StatisticsMonthGeneratedHistory{}))
			require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
			require.NoError(t, db.Exec("CREATE TABLE invoice_commit_parent (id INTEGER PRIMARY KEY)").Error)
			require.NoError(t, db.Exec("CREATE TABLE invoice_commit_child (parent_id INTEGER REFERENCES invoice_commit_parent(id) DEFERRABLE INITIALLY DEFERRED)").Error)
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&model.StatisticsMonthGeneratedHistory{}, "statistics_months", "statistics", "invoice_commit_child", "invoice_commit_parent"))
			})
			if reject {
				history := db.NamingStrategy.TableName("StatisticsMonthGeneratedHistory")
				require.NoError(t, db.Exec("CREATE TRIGGER invoice_reject_commit AFTER INSERT ON "+history+" BEGIN INSERT INTO invoice_commit_child(parent_id) VALUES (1); END").Error)
			}
			require.NoError(t, db.Table("statistics").Create(map[string]any{"date": "2026-09-01", "user_id": 1, "channel_id": 1, "model_name": "synthetic", "quota": 100, "request_count": 1}).Error)
			month := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
			err = model.InsertStatisticsMonthForDate(month)
			if reject {
				assert.ErrorContains(t, err, "FOREIGN KEY constraint failed")
			} else {
				require.NoError(t, err)
			}
			var invoices, histories int64
			require.NoError(t, db.Table("statistics_months").Count(&invoices).Error)
			require.NoError(t, db.Model(&model.StatisticsMonthGeneratedHistory{}).Count(&histories).Error)
			if reject {
				require.Zero(t, invoices)
				require.Zero(t, histories)
				require.NoError(t, db.Exec("DROP TRIGGER invoice_reject_commit").Error)
				require.NoError(t, model.InsertStatisticsMonthForDate(month))
				require.True(t, model.IsStatisticsMonthGenerated(month))
			} else {
				require.EqualValues(t, 1, invoices)
				require.EqualValues(t, 1, histories)
			}
		})
	}
}
