package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envMap(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func requireEnv(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Skip(name + " not set; skipping MySQL integration test")
	}
	return v
}

func TestRunRequiresDSN(t *testing.T) {
	err := run(context.Background(), envMap(nil), nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "MYSQL_DSN") {
		t.Errorf("run() = %v, want an error naming MYSQL_DSN", err)
	}
}

func TestRunFailsOnUnwritableAuditLog(t *testing.T) {
	err := run(context.Background(), envMap(map[string]string{
		"MYSQL_DSN":     "mcp_ro:mcp_ro@tcp(127.0.0.1:1)/demo",
		"MCP_AUDIT_LOG": filepath.Join(t.TempDir(), "missing-dir", "audit.log"),
	}), nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "audit log") {
		t.Errorf("run() = %v, want an audit log error", err)
	}
}

func TestRunRefusesWriteCapableUser(t *testing.T) {
	dsn := requireEnv(t, "MYSQL_TEST_ROOT_DSN")
	err := run(context.Background(), envMap(map[string]string{"MYSQL_DSN": dsn}), nil, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run() started with a write-capable user")
	}
	if msg := err.Error(); !strings.HasPrefix(msg, "refusing to start") || !strings.Contains(msg, "INSERT") {
		t.Errorf("run() = %q, want it to start with \"refusing to start\" and quote the INSERT grant", msg)
	}
}
