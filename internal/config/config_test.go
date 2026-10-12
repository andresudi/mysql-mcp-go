package config

import (
	"strings"
	"testing"
	"time"
)

const dsn = "mcp_ro:mcp_ro@tcp(127.0.0.1:3316)/demo"

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestLoadDefaults(t *testing.T) {
	got, err := Load(env(map[string]string{"MYSQL_DSN": dsn}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{DSN: dsn, MaxRows: 200, Timeout: 5 * time.Second, AuditLogPath: ""}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadAllSet(t *testing.T) {
	got, err := Load(env(map[string]string{
		"MYSQL_DSN":      dsn,
		"MCP_MAX_ROWS":   "50",
		"MCP_TIMEOUT_MS": "1500",
		"MCP_AUDIT_LOG":  "/var/log/mcp.log",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{DSN: dsn, MaxRows: 50, Timeout: 1500 * time.Millisecond, AuditLogPath: "/var/log/mcp.log"}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		vars    map[string]string
		wantVar string
	}{
		{map[string]string{}, "MYSQL_DSN"},
		{map[string]string{"MYSQL_DSN": dsn, "MCP_MAX_ROWS": "0"}, "MCP_MAX_ROWS"},
		{map[string]string{"MYSQL_DSN": dsn, "MCP_MAX_ROWS": "-1"}, "MCP_MAX_ROWS"},
		{map[string]string{"MYSQL_DSN": dsn, "MCP_MAX_ROWS": "abc"}, "MCP_MAX_ROWS"},
		{map[string]string{"MYSQL_DSN": dsn, "MCP_TIMEOUT_MS": "0"}, "MCP_TIMEOUT_MS"},
		{map[string]string{"MYSQL_DSN": dsn, "MCP_TIMEOUT_MS": "5s"}, "MCP_TIMEOUT_MS"},
	}
	for _, tt := range tests {
		_, err := Load(env(tt.vars))
		if err == nil {
			t.Errorf("Load(%v) = nil error, want one naming %s", tt.vars, tt.wantVar)
			continue
		}
		if !strings.Contains(err.Error(), tt.wantVar) {
			t.Errorf("Load(%v) error %q does not name %s", tt.vars, err, tt.wantVar)
		}
	}
}
