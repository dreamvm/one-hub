package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"

	"gorm.io/gorm"
)

var ErrOIDCIdentityUnresolved = errors.New("OIDC身份归属待核实，无法自动关联")

// The length prefix separates issuer from subject without normalization. A
// fixed hexadecimal key avoids database collation and index-length semantics.
func oidcIdentityKey(issuer, subject string) string {
	sum := sha256.Sum256([]byte(strconv.Itoa(len(issuer)) + ":" + issuer + subject))
	return fmt.Sprintf("%x", sum)
}

func (user *User) validateOIDCIdentity() error {
	if user.OidcId == "" && user.OidcIssuer == "" && user.OidcIdentityKey == nil {
		return nil
	}
	if user.OidcId == "" || user.OidcIssuer == "" || user.OidcIdentityKey == nil ||
		*user.OidcIdentityKey != oidcIdentityKey(user.OidcIssuer, user.OidcId) {
		return ErrOIDCIdentityUnresolved
	}
	return nil
}

func (user *User) FillUserByOIDCIdentity(issuer, subject string) error {
	if issuer == "" || subject == "" {
		return ErrOIDCIdentityUnresolved
	}
	// A follower or interrupted migration must not admit new identities before
	// the database uniqueness guard exists. Do not fall back to subject lookup.
	statement := &gorm.Statement{DB: DB}
	if err := statement.Parse(&User{}); err != nil {
		return err
	}
	// Pass the resolved table/index names: HasIndex(model, field) reparses and
	// mutates GORM's cached index metadata, which is unsafe in concurrent logins.
	indexName := DB.NamingStrategy.IndexName(statement.Table, "OidcIdentityKey")
	if !DB.Migrator().HasIndex(statement.Table, indexName) {
		return ErrOIDCIdentityUnresolved
	}
	var found User
	err := DB.Unscoped().Where("oidc_identity_key = ?", oidcIdentityKey(issuer, subject)).First(&found).Error
	if err == nil {
		if found.OidcIssuer != issuer || found.OidcId != subject || found.DeletedAt.Valid {
			return ErrOIDCIdentityUnresolved
		}
		*user = found
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	// Old rows have no trustworthy issuer. Retain them, including deleted rows,
	// and fail closed instead of assigning the current provider on first login.
	// Also reject an inconsistent row for this pair rather than select a winner.
	rows, err := DB.Unscoped().Model(&User{}).Where("oidc_id = ? AND (oidc_issuer IS NULL OR oidc_issuer = '' OR oidc_issuer = ?)", subject, issuer).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var unresolved User
		if err := DB.ScanRows(rows, &unresolved); err != nil {
			return err
		}
		// The SQL predicate may use a case-insensitive/trailing-space collation.
		if unresolved.OidcId == subject && (unresolved.OidcIssuer == "" || unresolved.OidcIssuer == issuer) {
			// A concurrent INSERT may finish after the first key lookup. Accept
			// that complete, exact pair instead of classifying it as legacy data.
			if unresolved.OidcIssuer == issuer && unresolved.OidcIdentityKey != nil &&
				*unresolved.OidcIdentityKey == oidcIdentityKey(issuer, subject) && !unresolved.DeletedAt.Valid {
				*user = unresolved
				return nil
			}
			return ErrOIDCIdentityUnresolved
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return gorm.ErrRecordNotFound
}

// InsertOIDC creates the user and unique identity in the same INSERT. A losing
// concurrent registration may log in only the exact, non-deleted winning pair.
// Initial quota and registration logs are emitted only by the successful Insert.
func (user *User) InsertOIDC(issuer, subject string) error {
	var existing User
	if err := existing.FillUserByOIDCIdentity(issuer, subject); err == nil {
		*user = existing
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	key := oidcIdentityKey(issuer, subject)
	user.OidcId, user.OidcIssuer, user.OidcIdentityKey = subject, issuer, &key
	if err := user.Insert(0); err != nil {
		if lookupErr := existing.FillUserByOIDCIdentity(issuer, subject); lookupErr == nil {
			*user = existing
			return nil
		}
		return err
	}
	return nil
}
