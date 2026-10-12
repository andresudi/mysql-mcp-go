package db

import (
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestNormalizeDSNForcesSafeOptions(t *testing.T) {
	out, err := normalizeDSN("mcp_ro:pw@tcp(127.0.0.1:3316)/demo?multiStatements=true&interpolateParams=true")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := mysql.ParseDSN(out)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MultiStatements {
		t.Error("multiStatements left on")
	}
	if cfg.InterpolateParams {
		t.Error("interpolateParams left on")
	}
	if cfg.DBName != "demo" || cfg.User != "mcp_ro" || cfg.Addr != "127.0.0.1:3316" {
		t.Errorf("connection details changed: %+v", cfg)
	}
}

func TestNormalizeDSNRequiresDatabase(t *testing.T) {
	_, err := normalizeDSN("mcp_ro:pw@tcp(127.0.0.1:3316)/")
	if err == nil || !strings.Contains(err.Error(), "database") {
		t.Errorf("err = %v, want one saying the DSN must name a database", err)
	}
}

func TestNormalizeDSNErrorHidesPassword(t *testing.T) {
	_, err := normalizeDSN("mcp_ro:s3cret@tcp(127.0.0.1:3316/demo")
	if err == nil {
		t.Fatal("want parse error")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Errorf("error leaks the password: %v", err)
	}
}
