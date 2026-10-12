package db

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReadOnlyQueryTypedRows(t *testing.T) {
	s := openStore(t, testDSN(t), defaultLimits)
	res, err := s.ReadOnlyQuery(context.Background(), "SELECT id, title, due_date FROM tasks ORDER BY id LIMIT 3")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"id", "title", "due_date"}; !reflect.DeepEqual(res.Columns, want) {
		t.Errorf("columns = %v, want %v", res.Columns, want)
	}
	if len(res.Rows) != 3 || res.Truncated {
		t.Fatalf("got %d rows, truncated=%v; want 3, false", len(res.Rows), res.Truncated)
	}
	if id, ok := res.Rows[0][0].(int64); !ok || id != 1 {
		t.Errorf("id = %#v, want int64(1)", res.Rows[0][0])
	}
	if _, ok := res.Rows[0][2].(string); !ok {
		t.Errorf("due_date = %#v, want a string", res.Rows[0][2])
	}
}

func TestReadOnlyQueryEmptyResult(t *testing.T) {
	s := openStore(t, testDSN(t), defaultLimits)
	res, err := s.ReadOnlyQuery(context.Background(), "SELECT id FROM tasks WHERE id < 0")
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows == nil || len(res.Rows) != 0 {
		t.Errorf("rows = %#v, want empty non-nil slice", res.Rows)
	}
}

func TestReadOnlyQueryTruncates(t *testing.T) {
	s := openStore(t, testDSN(t), Limits{MaxRows: 10, Timeout: 5 * time.Second})
	ctx := context.Background()

	res, err := s.ReadOnlyQuery(ctx, "SELECT * FROM tasks")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 10 || !res.Truncated {
		t.Errorf("got %d rows, truncated=%v; want 10, true", len(res.Rows), res.Truncated)
	}

	res, err = s.ReadOnlyQuery(ctx, "SELECT * FROM tasks LIMIT 10")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 10 || res.Truncated {
		t.Errorf("got %d rows, truncated=%v; want 10, false", len(res.Rows), res.Truncated)
	}
}

func TestReadOnlyQueryOtherReadStatements(t *testing.T) {
	s := openStore(t, testDSN(t), defaultLimits)
	for _, q := range []string{"SHOW TABLES", "DESCRIBE tasks", "EXPLAIN SELECT * FROM tasks"} {
		res, err := s.ReadOnlyQuery(context.Background(), q)
		if err != nil {
			t.Errorf("%s: %v", q, err)
			continue
		}
		if len(res.Rows) == 0 {
			t.Errorf("%s returned no rows", q)
		}
	}
}

func TestReadOnlyQueryTimeout(t *testing.T) {
	s := openStore(t, testDSN(t), Limits{MaxRows: 200, Timeout: 500 * time.Millisecond})
	ctx := context.Background()

	// When SLEEP is the whole query, MySQL stops it at the limit and returns 1
	// instead of an error.
	start := time.Now()
	res, err := s.ReadOnlyQuery(ctx, "SELECT SLEEP(10)")
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("SELECT SLEEP(10) took %v, want it stopped near 500ms", d)
	}
	if !reflect.DeepEqual(res.Rows, [][]any{{int64(1)}}) {
		t.Errorf("rows = %#v, want [[1]] (interrupted sleep)", res.Rows)
	}

	// As part of a larger query, the timeout is an error.
	start = time.Now()
	_, err = s.ReadOnlyQuery(ctx, "SELECT COUNT(*) FROM users WHERE SLEEP(1)")
	if err == nil || !strings.Contains(err.Error(), "3024") {
		t.Errorf("err = %v, want MySQL error 3024", err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("query took %v, want it stopped near 500ms", d)
	}
}
