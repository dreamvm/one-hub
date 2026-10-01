package mockserver

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteFixtureRequest selects fixed accounting facts from the owned fixture.
// It accepts neither SQL text nor a database path from the command input.
type SQLiteFixtureRequest struct {
	Action      string
	UserID      int64
	FiniteID    int64
	UnlimitedID int64
}

func ReadSQLiteFixture(path string, input SQLiteFixtureRequest) ([]int64, error) {
	if input.UserID <= 0 || (input.Action != "tokens" && input.Action != "snapshot") ||
		(input.Action == "snapshot" && (input.FiniteID <= 0 || input.UnlimitedID <= 0 || input.FiniteID == input.UnlimitedID)) {
		return nil, fmt.Errorf("invalid synthetic accounting request")
	}
	uri := &url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&_query_only=1&_busy_timeout=1000"}
	db, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var values []int64
	var query string
	if input.Action == "tokens" {
		values = make([]int64, 2)
		query = fmt.Sprintf("SELECT (SELECT id FROM tokens WHERE user_id=%d AND name='smoke_load_finite' AND NOT unlimited_quota), (SELECT id FROM tokens WHERE user_id=%d AND name='sys_playground' AND unlimited_quota)", input.UserID, input.UserID)
	} else {
		values = make([]int64, 14)
		// All substitutions are validated integer IDs; SQL and columns are fixed.
		query = fmt.Sprintf(`SELECT quota,used_quota,request_count,
(SELECT used_quota FROM tokens WHERE id=%[2]d AND user_id=%[1]d AND NOT unlimited_quota),
(SELECT remain_quota FROM tokens WHERE id=%[2]d AND user_id=%[1]d AND NOT unlimited_quota),
(SELECT used_quota FROM tokens WHERE id=%[3]d AND user_id=%[1]d AND unlimited_quota),
(SELECT remain_quota FROM tokens WHERE id=%[3]d AND user_id=%[1]d AND unlimited_quota),
(SELECT COUNT(*) FROM quota_reservations WHERE user_id=%[1]d),
(SELECT COUNT(*) FROM quota_reservations WHERE user_id=%[1]d AND state='consumed' AND outcome='consume'),
(SELECT COALESCE(SUM(final_quota),0) FROM quota_reservations WHERE user_id=%[1]d AND state='consumed'),
(SELECT COUNT(*) FROM logs WHERE user_id=%[1]d AND type=2),
(SELECT COALESCE(SUM(quota),0) FROM logs WHERE user_id=%[1]d AND type=2),
(SELECT used_quota FROM channels WHERE name='Mock OpenAI'),
(SELECT COUNT(*) FROM quota_reservations WHERE user_id=%[1]d AND state IN ('reserved','pending','reconcile'))
FROM users WHERE id=%[1]d`, input.UserID, input.FiniteID, input.UnlimitedID)
	}
	targets := make([]any, len(values))
	for i := range values {
		targets[i] = &values[i]
	}
	if err := db.QueryRowContext(ctx, query).Scan(targets...); err != nil {
		return nil, err
	}
	return values, nil
}
