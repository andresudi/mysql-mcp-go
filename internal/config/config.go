// Package config reads the server's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Config holds the server settings. An empty AuditLogPath means stderr.
type Config struct {
	DSN          string
	MaxRows      int
	Timeout      time.Duration
	AuditLogPath string
}

// Load builds a Config from getenv, which is os.Getenv outside tests.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DSN:          getenv("MYSQL_DSN"),
		MaxRows:      200,
		Timeout:      5000 * time.Millisecond,
		AuditLogPath: getenv("MCP_AUDIT_LOG"),
	}
	if cfg.DSN == "" {
		return Config{}, errors.New("MYSQL_DSN is required, e.g. mcp_ro:mcp_ro@tcp(127.0.0.1:3316)/demo")
	}
	if v := getenv("MCP_MAX_ROWS"); v != "" {
		n, err := positiveInt("MCP_MAX_ROWS", v)
		if err != nil {
			return Config{}, err
		}
		cfg.MaxRows = n
	}
	if v := getenv("MCP_TIMEOUT_MS"); v != "" {
		n, err := positiveInt("MCP_TIMEOUT_MS", v)
		if err != nil {
			return Config{}, err
		}
		cfg.Timeout = time.Duration(n) * time.Millisecond
	}
	return cfg, nil
}

func positiveInt(name, v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", name, v)
	}
	return n, nil
}
