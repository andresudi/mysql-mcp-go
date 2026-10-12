package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/andresudi/mysql-mcp-go/internal/audit"
	"github.com/andresudi/mysql-mcp-go/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeStore records what the handlers ask for and returns canned answers.
type fakeStore struct {
	tables    []db.Table
	desc      db.TableDesc
	result    db.Result
	err       error
	described []string
	queries   []string
}

func (f *fakeStore) ListTables(context.Context) ([]db.Table, error) {
	return f.tables, f.err
}

func (f *fakeStore) DescribeTable(_ context.Context, name string) (db.TableDesc, error) {
	f.described = append(f.described, name)
	return f.desc, f.err
}

func (f *fakeStore) ReadOnlyQuery(_ context.Context, sql string) (db.Result, error) {
	f.queries = append(f.queries, sql)
	return f.result, f.err
}

// connect starts the server on an in-memory transport and returns a client
// session talking to it.
func connect(t *testing.T, store Store, log *audit.Logger) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := NewServer(store, log).Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	return res
}

// decode converts a result's structured content into out.
func decode(t *testing.T, res *mcp.CallToolResult, out any) {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool error: %s", errorText(res))
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatal(err)
	}
}

func errorText(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func ptr[T any](v T) *T { return &v }

func TestListToolsAreReadOnly(t *testing.T) {
	cs := connect(t, &fakeStore{}, audit.New(io.Discard))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("%s has no description", tool.Name)
		}
	}
	slices.Sort(names)
	if want := []string{"describe_table", "list_tables", "query"}; !reflect.DeepEqual(names, want) {
		t.Errorf("tools = %v, want %v", names, want)
	}
}

func TestListTables(t *testing.T) {
	store := &fakeStore{tables: []db.Table{
		{Name: "tasks", EstimatedRows: ptr(int64(300)), Comment: "Work items"},
		{Name: "v_open", Comment: ""},
	}}
	cs := connect(t, store, audit.New(io.Discard))

	var out ListTablesOutput
	decode(t, call(t, cs, "list_tables", nil), &out)
	want := ListTablesOutput{Tables: []TableInfo{
		{Name: "tasks", EstimatedRows: ptr(int64(300)), Comment: "Work items"},
		{Name: "v_open"},
	}}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("output = %+v, want %+v", out, want)
	}
}

func TestDescribeTable(t *testing.T) {
	store := &fakeStore{desc: db.TableDesc{
		Name: "tasks",
		Columns: []db.Column{
			{Name: "id", Type: "int", Key: "PRI"},
			{Name: "status", Type: "enum('todo','done')", Default: ptr("todo")},
			{Name: "due_date", Type: "date", Nullable: true},
		},
		Indexes: []db.Index{{Name: "PRIMARY", Unique: true, Columns: []string{"id"}}},
	}}
	cs := connect(t, store, audit.New(io.Discard))

	var out DescribeTableOutput
	decode(t, call(t, cs, "describe_table", map[string]any{"table": "tasks"}), &out)
	if !reflect.DeepEqual(store.described, []string{"tasks"}) {
		t.Errorf("store asked for %v, want [tasks]", store.described)
	}
	want := DescribeTableOutput{
		Table: "tasks",
		Columns: []ColumnInfo{
			{Name: "id", Type: "int", Key: "PRI"},
			{Name: "status", Type: "enum('todo','done')", Default: ptr("todo")},
			{Name: "due_date", Type: "date", Nullable: true},
		},
		Indexes: []IndexInfo{{Name: "PRIMARY", Unique: true, Columns: []string{"id"}}},
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("output = %+v, want %+v", out, want)
	}
}

func TestDescribeTableRequiresTable(t *testing.T) {
	store := &fakeStore{}
	cs := connect(t, store, audit.New(io.Discard))
	res := call(t, cs, "describe_table", map[string]any{})
	if !res.IsError {
		t.Error("describe_table without a table succeeded")
	}
	if len(store.described) != 0 {
		t.Errorf("store was called with %v", store.described)
	}
}

func TestQuery(t *testing.T) {
	store := &fakeStore{result: db.Result{
		Columns:   []string{"id", "title"},
		Rows:      [][]any{{int64(1), "Fix login page"}, {int64(2), nil}},
		Truncated: true,
	}}
	cs := connect(t, store, audit.New(io.Discard))

	var out QueryOutput
	decode(t, call(t, cs, "query", map[string]any{"sql": "SELECT id, title FROM tasks"}), &out)
	if !reflect.DeepEqual(store.queries, []string{"SELECT id, title FROM tasks"}) {
		t.Errorf("store got %q", store.queries)
	}
	want := QueryOutput{
		Columns:   []string{"id", "title"},
		Rows:      [][]any{{float64(1), "Fix login page"}, {float64(2), nil}}, // JSON numbers decode as float64
		RowCount:  2,
		Truncated: true,
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("output = %+v, want %+v", out, want)
	}
}

func TestQueryRefusesWritesBeforeTheStore(t *testing.T) {
	store := &fakeStore{}
	cs := connect(t, store, audit.New(io.Discard))
	res := call(t, cs, "query", map[string]any{"sql": "DELETE FROM tasks"})
	if !res.IsError {
		t.Fatal("DELETE was not refused")
	}
	if msg := errorText(res); !strings.Contains(msg, "SELECT") || !strings.Contains(msg, "read") {
		t.Errorf("refusal %q should explain that only read statements are allowed", msg)
	}
	if len(store.queries) != 0 {
		t.Errorf("store was called with %q", store.queries)
	}
}

func TestStoreErrorBecomesToolError(t *testing.T) {
	store := &fakeStore{err: errors.New("Error 1142 (42000): DROP command denied")}
	cs := connect(t, store, audit.New(io.Discard))
	for tool, args := range map[string]map[string]any{
		"list_tables":    nil,
		"describe_table": {"table": "tasks"},
		"query":          {"sql": "SELECT 1"},
	} {
		res := call(t, cs, tool, args)
		if !res.IsError || !strings.Contains(errorText(res), "1142") {
			t.Errorf("%s: IsError=%v text=%q, want the store error", tool, res.IsError, errorText(res))
		}
	}
}
