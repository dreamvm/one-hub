package model_test

import (
	"errors"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"one-api/common/config"
	"one-api/model"
	"strings"
	"testing"
)

func TestQuotaTransactionOIDCWinnerBetweenLookupStages(t *testing.T) {
	db, _ := quotaTransactionFixture(t, false, false)
	old := config.QuotaForNewUser
	config.QuotaForNewUser = 0
	t.Cleanup(func() { config.QuotaForNewUser = old })
	const issuer = "https://identity.fixture.invalid"
	winner := model.User{Username: "winner"}
	require.NoError(t, winner.InsertOIDC(issuer, "same-subject"))
	require.NoError(t, db.Unscoped().Delete(&winner).Error)
	winner.Id = 0
	inserted := false
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("oidc_interleaving", func(tx *gorm.DB) {
		if !inserted && errors.Is(tx.Error, gorm.ErrRecordNotFound) && strings.Contains(tx.Statement.SQL.String(), "oidc_identity_key =") {
			inserted = true
			require.NoError(t, db.Create(&winner).Error)
		}
	}))
	var lookup model.User
	require.NoError(t, lookup.FillUserByOIDCIdentity(issuer, "same-subject"), "completed concurrent registration must not be classified as unresolved historical ownership")
	require.Equal(t, winner.Id, lookup.Id)
}
