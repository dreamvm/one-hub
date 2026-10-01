package mockserver_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"one-api/.github/smoke/mockserver"
)

func TestSQLiteFixtureFixedReadSnapshotAndOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one-api.db")
	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL;
CREATE TABLE users(id INTEGER, quota INTEGER, used_quota INTEGER, request_count INTEGER);
CREATE TABLE tokens(id INTEGER, user_id INTEGER, name TEXT, unlimited_quota INTEGER, used_quota INTEGER, remain_quota INTEGER);
CREATE TABLE quota_reservations(user_id INTEGER, state TEXT, outcome TEXT, final_quota INTEGER);
CREATE TABLE logs(user_id INTEGER, type INTEGER, quota INTEGER);
CREATE TABLE channels(name TEXT, used_quota INTEGER);
INSERT INTO users VALUES(1,986,14,1);
INSERT INTO tokens VALUES(1,1,'smoke_load_finite',0,14,986),(2,1,'sys_playground',1,0,0),(3,2,'foreign-fixture',0,0,100);
INSERT INTO quota_reservations VALUES(1,'consumed','consume',14);
INSERT INTO logs VALUES(1,2,14);
INSERT INTO channels VALUES('Mock OpenAI',14);`)
	require.NoError(t, err)
	ids, err := mockserver.ReadSQLiteFixture(path, mockserver.SQLiteFixtureRequest{Action: "tokens", UserID: 1})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, ids)
	input := mockserver.SQLiteFixtureRequest{Action: "snapshot", UserID: 1, FiniteID: 1, UnlimitedID: 2}
	values, err := mockserver.ReadSQLiteFixture(path, input)
	require.NoError(t, err)
	require.Equal(t, []int64{986, 14, 1, 14, 986, 0, 0, 1, 1, 14, 1, 14, 14, 0}, values)
	// Reads leave the writer and its original facts usable; incomplete ownership
	// or account data must error rather than silently report a zero snapshot.
	input.FiniteID = 3
	_, err = mockserver.ReadSQLiteFixture(path, input)
	require.Error(t, err)
	input.FiniteID, input.UserID = 1, 99
	_, err = mockserver.ReadSQLiteFixture(path, input)
	require.Error(t, err)
	var quota int
	require.NoError(t, db.QueryRow("SELECT quota FROM users WHERE id=1").Scan(&quota))
	require.Equal(t, 986, quota)
	_, err = db.Exec("INSERT INTO quota_reservations VALUES(1,'pending','consume',14)")
	require.NoError(t, err)
	input.UserID = 1
	values, err = mockserver.ReadSQLiteFixture(path, input)
	require.NoError(t, err)
	require.Equal(t, int64(1), values[13])
}

func TestSQLiteFixtureRejectsInvalidActionAndDoesNotCreateMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one-api.db")
	for _, input := range []mockserver.SQLiteFixtureRequest{
		{Action: "tokens", UserID: 1}, {Action: "DELETE", UserID: 1}, {Action: "tokens", UserID: -1},
		{Action: "snapshot", UserID: 1, FiniteID: 2, UnlimitedID: 2},
	} {
		_, err := mockserver.ReadSQLiteFixture(path, input)
		require.Error(t, err)
	}
	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err))
}
