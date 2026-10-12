package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/andresudi/mysql-mcp-go/internal/audit"
	"github.com/andresudi/mysql-mcp-go/internal/db"
)

func TestEveryCallIsAudited(t *testing.T) {
	var buf bytes.Buffer
	store := &fakeStore{
		tables: []db.Table{{Name: "tasks"}, {Name: "users"}},
		result: db.Result{Columns: []string{"n"}, Rows: [][]any{{int64(1)}}},
	}
	cs := connect(t, store, audit.New(&buf))

	call(t, cs, "list_tables", nil)
	call(t, cs, "query", map[string]any{"sql": "SELECT 1"})
	call(t, cs, "query", map[string]any{"sql": "DELETE FROM t"})
	store.err = errors.New("boom")
	call(t, cs, "describe_table", map[string]any{"table": "tasks"})

	type line struct {
		Time       string  `json:"time"`
		Tool       string  `json:"tool"`
		SQL        string  `json:"sql"`
		Table      string  `json:"table"`
		RowCount   int     `json:"row_count"`
		DurationMS *int64  `json:"duration_ms"`
		Error      *string `json:"error"`
	}
	var got []line
	for _, raw := range strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n") {
		var l line
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			t.Fatalf("bad audit line %q: %v", raw, err)
		}
		got = append(got, l)
	}
	if len(got) != 4 {
		t.Fatalf("got %d audit lines, want 4:\n%s", len(got), buf.String())
	}

	want := []struct {
		tool, sql, table string
		rows             int
		errPart          string // "" means no error
	}{
		{"list_tables", "", "", 2, ""},
		{"query", "SELECT 1", "", 1, ""},
		{"query", "DELETE FROM t", "", 0, "refused"},
		{"describe_table", "", "tasks", 0, "boom"},
	}
	for i, w := range want {
		g := got[i]
		if g.Tool != w.tool || g.SQL != w.sql || g.Table != w.table || g.RowCount != w.rows {
			t.Errorf("line %d = %+v, want tool=%s sql=%q table=%q rows=%d", i, g, w.tool, w.sql, w.table, w.rows)
		}
		if g.Time == "" || g.DurationMS == nil || *g.DurationMS < 0 {
			t.Errorf("line %d: missing time or duration: %+v", i, g)
		}
		switch {
		case w.errPart == "" && g.Error != nil:
			t.Errorf("line %d: unexpected error %q", i, *g.Error)
		case w.errPart != "" && (g.Error == nil || !strings.Contains(*g.Error, w.errPart)):
			t.Errorf("line %d: error = %v, want it to contain %q", i, g.Error, w.errPart)
		}
	}
}
