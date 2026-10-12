package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andresudi/mysql-mcp-go/internal/guard"
)

var defaultLimits = Limits{MaxRows: 200, Timeout: 5 * time.Second}

func openStore(t *testing.T, dsn string, lim Limits) *Store {
	t.Helper()
	s, err := Open(context.Background(), dsn, lim)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestGrantsReadOnlyUserPasses(t *testing.T) {
	s := openStore(t, testDSN(t), defaultLimits)
	lines, err := s.Grants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.CheckGrants(lines); err != nil {
		t.Errorf("read-only user rejected: %v (grants %q)", err, lines)
	}
}

func TestGrantsRootFails(t *testing.T) {
	s := openStore(t, rootDSN(t), defaultLimits)
	lines, err := s.Grants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = guard.CheckGrants(lines)
	if err == nil || !strings.Contains(err.Error(), "INSERT") {
		t.Errorf("CheckGrants(root) = %v, want an error quoting INSERT", err)
	}
}

func TestOpenUnreachableFailsWithoutPassword(t *testing.T) {
	start := time.Now()
	_, err := Open(context.Background(), "mcp_ro:s3cret@tcp(127.0.0.1:1)/demo", defaultLimits)
	if err == nil {
		t.Fatal("Open succeeded against a closed port")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Errorf("error leaks the password: %v", err)
	}
	if d := time.Since(start); d > 15*time.Second {
		t.Errorf("Open took %v, want under the 10 s ping timeout", d)
	}
}
