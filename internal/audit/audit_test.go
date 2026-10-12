package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogWritesOneJSONLine(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	err := l.Log(Entry{
		Time:     time.Date(2026, 10, 12, 13, 55, 42, 401_000_000, time.UTC),
		Tool:     "query",
		SQL:      "SELECT \"a\"\nFROM t",
		RowCount: 3,
		Duration: 12 * time.Millisecond,
		Err:      errors.New("boom"),
	})
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("want exactly one newline-terminated line, got %q", out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("line is not JSON: %v", err)
	}
	want := map[string]any{
		"time":        "2026-10-12T13:55:42.401Z",
		"tool":        "query",
		"sql":         "SELECT \"a\"\nFROM t",
		"row_count":   float64(3),
		"duration_ms": float64(12),
		"error":       "boom",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %#v, want %#v", k, got[k], v)
		}
	}
	if _, ok := got["table"]; ok {
		t.Error("empty table should be omitted")
	}
}

func TestLogOmitsEmptyFields(t *testing.T) {
	var buf bytes.Buffer
	if err := New(&buf).Log(Entry{Time: time.Now(), Tool: "describe_table", Table: "tasks"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["table"] != "tasks" {
		t.Errorf("table = %v, want tasks", got["table"])
	}
	for _, k := range []string{"sql", "error"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s should be omitted when empty", k)
		}
	}
}

func TestLogConcurrent(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = l.Log(Entry{Time: time.Now(), Tool: "list_tables"})
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 50 {
		t.Fatalf("got %d lines, want 50", len(lines))
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Errorf("invalid JSON line %q", line)
		}
	}
}
