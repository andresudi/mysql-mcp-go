// Command mysql-mcp is an MCP server that gives read-only access to one MySQL
// database over stdio.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/andresudi/mysql-mcp-go/internal/audit"
	"github.com/andresudi/mysql-mcp-go/internal/config"
	"github.com/andresudi/mysql-mcp-go/internal/db"
	"github.com/andresudi/mysql-mcp-go/internal/guard"
	"github.com/andresudi/mysql-mcp-go/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, &mcp.StdioTransport{}, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "mysql-mcp:", err)
		os.Exit(1)
	}
}

// run loads config, connects, refuses to start unless the MySQL user is
// read-only, then serves MCP on transport until the client disconnects.
// Nothing here writes to stdout: in production that is the MCP channel.
func run(ctx context.Context, getenv func(string) string, transport mcp.Transport, stderr io.Writer) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}

	auditOut := stderr
	if cfg.AuditLogPath != "" {
		f, err := os.OpenFile(cfg.AuditLogPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
		if err != nil {
			return fmt.Errorf("opening audit log: %w", err)
		}
		defer f.Close()
		auditOut = f
	}

	store, err := db.Open(ctx, cfg.DSN, db.Limits{MaxRows: cfg.MaxRows, Timeout: cfg.Timeout})
	if err != nil {
		return err
	}
	defer store.Close()

	grants, err := store.Grants(ctx)
	if err != nil {
		return err
	}
	if err := guard.CheckGrants(grants); err != nil {
		return fmt.Errorf("refusing to start: the MySQL user must hold only SELECT and SHOW VIEW: %w", err)
	}

	return tools.NewServer(store, audit.New(auditOut)).Run(ctx, transport)
}
