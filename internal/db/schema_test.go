package db

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestListTables(t *testing.T) {
	s := openStore(t, testDSN(t), defaultLimits)
	tables, err := s.ListTables(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tb := range tables {
		names = append(names, tb.Name)
		if tb.EstimatedRows == nil || *tb.EstimatedRows <= 0 {
			t.Errorf("%s: estimated rows = %v, want > 0", tb.Name, tb.EstimatedRows)
		}
		if tb.Comment == "" {
			t.Errorf("%s: empty comment", tb.Name)
		}
	}
	if want := []string{"comments", "projects", "tasks", "users"}; !reflect.DeepEqual(names, want) {
		t.Errorf("tables = %v, want %v", names, want)
	}
}

func TestDescribeTable(t *testing.T) {
	s := openStore(t, testDSN(t), defaultLimits)
	desc, err := s.DescribeTable(context.Background(), "tasks")
	if err != nil {
		t.Fatal(err)
	}

	cols := map[string]Column{}
	var order []string
	for _, c := range desc.Columns {
		cols[c.Name] = c
		order = append(order, c.Name)
	}
	wantOrder := []string{"id", "project_id", "assignee_id", "title", "status", "due_date", "created_at"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Errorf("columns = %v, want %v", order, wantOrder)
	}
	if cols["id"].Key != "PRI" || cols["id"].Nullable {
		t.Errorf("id = %+v, want non-null PRI", cols["id"])
	}
	if !cols["due_date"].Nullable {
		t.Errorf("due_date = %+v, want nullable", cols["due_date"])
	}
	if st := cols["status"]; !strings.HasPrefix(st.Type, "enum(") || st.Default == nil || *st.Default != "todo" {
		t.Errorf("status = %+v, want enum type with default todo", st)
	}

	idx := map[string]Index{}
	for _, i := range desc.Indexes {
		idx[i.Name] = i
	}
	if p := idx["PRIMARY"]; !p.Unique || !reflect.DeepEqual(p.Columns, []string{"id"}) {
		t.Errorf("PRIMARY = %+v, want unique on [id]", p)
	}
	if i := idx["idx_tasks_status_due"]; i.Unique || !reflect.DeepEqual(i.Columns, []string{"status", "due_date"}) {
		t.Errorf("idx_tasks_status_due = %+v, want non-unique on [status due_date]", i)
	}
}

func TestDescribeTableNotFound(t *testing.T) {
	dsn := testDSN(t)
	s := openStore(t, dsn, defaultLimits)
	for _, name := range []string{"nope", "tasks; DROP TABLE users", "tasks' OR '1'='1"} {
		_, err := s.DescribeTable(context.Background(), name)
		if !errors.Is(err, ErrTableNotFound) {
			t.Errorf("DescribeTable(%q) = %v, want ErrTableNotFound", name, err)
		}
	}
	if n := queryInt(t, rawDB(t, dsn), "SELECT COUNT(*) FROM users"); n != 50 {
		t.Errorf("users has %d rows after injection attempts, want 50", n)
	}
}
