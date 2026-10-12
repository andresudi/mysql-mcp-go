package db

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
)

// These statements go straight to ReadOnlyQuery, skipping the allow-list on
// purpose: they show that the read-only user's privileges, the read-only
// transaction and the disabled multi-statements stop writes on their own.
var craftedWrites = []string{
	"DROP TABLE users",
	"DELETE FROM tasks",
	"UPDATE tasks SET status = 'done'",
	"INSERT INTO users (id, name, email, created_at) VALUES (999, 'x', 'x@example.com', NOW())",
	"CREATE TABLE t (id INT)",
	"WITH x AS (SELECT 1) DELETE FROM tasks",
	"WITH x AS (SELECT 1) UPDATE tasks SET status = 'done'",
	"EXPLAIN ANALYZE DELETE FROM tasks",
	"EXPLAIN ANALYZE DELETE t FROM tasks t",
	"SELECT 1; DROP TABLE users",
	"SELECT * FROM users INTO OUTFILE '/tmp/users.txt'",
	"/*!50000 DROP TABLE users */",
}

func TestCraftedWritesChangeNothing(t *testing.T) {
	dsn := testDSN(t)
	s := openStore(t, dsn, defaultLimits)
	db := rawDB(t, dsn)
	before := tableCounts(t, db)

	for _, q := range craftedWrites {
		_, err := s.ReadOnlyQuery(context.Background(), q)
		if err == nil {
			t.Errorf("%q succeeded, want an error", q)
		} else {
			t.Logf("%q: %v", q, err)
		}
		if after := tableCounts(t, db); !reflect.DeepEqual(after, before) {
			t.Fatalf("after %q: counts = %v, want %v", q, after, before)
		}
	}
}

// tableCounts returns the row count of every table in the schema; it also
// catches tables that were created or dropped.
func tableCounts(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()
	rows, err := db.Query("SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	rows.Close()
	counts := map[string]int{}
	for _, name := range names {
		counts[name] = queryInt(t, db, "SELECT COUNT(*) FROM `"+name+"`")
	}
	return counts
}
