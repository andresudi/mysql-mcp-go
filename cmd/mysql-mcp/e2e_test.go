package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The SQL Claude is expected to write for the demo question
// "Which project has the most overdue tasks, and who owns them?"
const demoSQL = `
	SELECT p.name AS project, u.name AS owner, COUNT(*) AS overdue
	FROM tasks t
	JOIN projects p ON p.id = t.project_id
	JOIN users u ON u.id = p.owner_id
	WHERE t.due_date < CURDATE() AND t.status <> 'done'
	GROUP BY p.id, p.name, u.name
	ORDER BY overdue DESC
	LIMIT 1`

// TestEndToEndInMemory runs the real startup path against the seeded database
// and calls every tool through an MCP client.
func TestEndToEndInMemory(t *testing.T) {
	dsn := requireEnv(t, "MYSQL_TEST_DSN")
	auditPath := filepath.Join(t.TempDir(), "audit.log")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverT, clientT := mcp.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, envMap(map[string]string{"MYSQL_DSN": dsn, "MCP_AUDIT_LOG": auditPath}), serverT, &bytes.Buffer{})
	}()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}

	var tables struct {
		Tables []struct{ Name string } `json:"tables"`
	}
	structured(t, callTool(t, cs, "list_tables", nil), &tables)
	if len(tables.Tables) != 4 {
		t.Errorf("list_tables returned %d tables, want 4", len(tables.Tables))
	}

	var desc struct {
		Columns []struct{ Name string } `json:"columns"`
	}
	structured(t, callTool(t, cs, "describe_table", map[string]any{"table": "tasks"}), &desc)
	if !hasColumn(desc.Columns, "due_date") {
		t.Errorf("describe_table(tasks) columns = %v, want due_date among them", desc.Columns)
	}

	var answer struct {
		Columns []string `json:"columns"`
		Rows    [][]any  `json:"rows"`
	}
	structured(t, callTool(t, cs, "query", map[string]any{"sql": demoSQL}), &answer)
	if len(answer.Rows) != 1 || answer.Rows[0][0] != "Billing migration" || answer.Rows[0][1] != "Ben Lindqvist" {
		t.Errorf("demo answer = %v, want Billing migration owned by Ben Lindqvist", answer.Rows)
	}

	res := callTool(t, cs, "query", map[string]any{"sql": "DELETE FROM tasks"})
	if !res.IsError {
		t.Error("DELETE was not refused")
	}
	var count struct {
		Rows [][]any `json:"rows"`
	}
	structured(t, callTool(t, cs, "query", map[string]any{"sql": "SELECT COUNT(*) FROM tasks"}), &count)
	if len(count.Rows) != 1 || count.Rows[0][0] != float64(300) {
		t.Errorf("tasks count = %v, want 300", count.Rows)
	}

	cs.Close()
	if err := <-done; err != nil {
		t.Errorf("run() = %v after the client disconnected, want nil", err)
	}
	log, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(log), "\n"); n != 5 {
		t.Errorf("audit log has %d lines, want 5:\n%s", n, log)
	}
}

// TestEndToEndStdio builds the binary and talks to it over stdin/stdout, the
// way Claude Code and Claude Desktop do. Any stray write to stdout would
// break the protocol and fail this test.
func TestEndToEndStdio(t *testing.T) {
	dsn := requireEnv(t, "MYSQL_TEST_DSN")
	bin := filepath.Join(t.TempDir(), "mysql-mcp")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "MYSQL_DSN="+dsn)
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "0"}, nil).Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v\nstderr:\n%s", err, stderr.String())
	}
	defer cs.Close()

	list, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 3 {
		t.Errorf("got %d tools, want 3", len(list.Tools))
	}
	if res := callTool(t, cs, "list_tables", nil); res.IsError {
		t.Errorf("list_tables failed: %v", res.Content)
	}
	// Closing waits for the process to exit, so stderr is complete.
	cs.Close()
	// With no audit file configured, the audit line goes to stderr.
	if !strings.Contains(stderr.String(), `"tool":"list_tables"`) {
		t.Errorf("stderr has no audit line for list_tables:\n%s", stderr.String())
	}
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

func structured(t *testing.T, res *mcp.CallToolResult, out any) {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool error: %v", res.Content)
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatal(err)
	}
}

func hasColumn(cols []struct{ Name string }, name string) bool {
	for _, c := range cols {
		if c.Name == name {
			return true
		}
	}
	return false
}

// lockedBuffer is a bytes.Buffer that is safe to write from the process's
// stderr copier while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
