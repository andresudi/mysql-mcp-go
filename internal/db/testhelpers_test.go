package db

import (
	"database/sql"
	"os"
	"testing"
)

// testDSN returns the read-only test DSN, or skips the test when it is unset.
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set; skipping MySQL integration test")
	}
	return dsn
}

// rootDSN returns the admin test DSN, or skips the test when it is unset.
func rootDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("MYSQL_TEST_ROOT_DSN")
	if dsn == "" {
		t.Skip("MYSQL_TEST_ROOT_DSN not set; skipping MySQL integration test")
	}
	return dsn
}

// rawDB opens a plain connection for checks that must not go through Store.
func rawDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func queryInt(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}
