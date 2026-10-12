// Package tools exposes the database to MCP clients as three read-only tools.
package tools

import (
	"context"
	"errors"

	"github.com/andresudi/mysql-mcp-go/internal/audit"
	"github.com/andresudi/mysql-mcp-go/internal/db"
	"github.com/andresudi/mysql-mcp-go/internal/guard"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.1.0"

// Store is the part of *db.Store the tools use.
type Store interface {
	ListTables(ctx context.Context) ([]db.Table, error)
	DescribeTable(ctx context.Context, name string) (db.TableDesc, error)
	ReadOnlyQuery(ctx context.Context, sql string) (db.Result, error)
}

// The input and output types below are the JSON contract MCP clients see.

type ListTablesOutput struct {
	Tables []TableInfo `json:"tables"`
}

type TableInfo struct {
	Name          string `json:"name"`
	EstimatedRows *int64 `json:"estimated_rows" jsonschema:"approximate row count; null for views"`
	Comment       string `json:"comment"`
}

type DescribeTableInput struct {
	Table string `json:"table" jsonschema:"name of a table in the connected database"`
}

type DescribeTableOutput struct {
	Table   string       `json:"table"`
	Columns []ColumnInfo `json:"columns"`
	Indexes []IndexInfo  `json:"indexes"`
}

type ColumnInfo struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Nullable bool    `json:"nullable"`
	Default  *string `json:"default"`
	Key      string  `json:"key" jsonschema:"PRI, UNI, MUL or empty"`
}

type IndexInfo struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Columns []string `json:"columns"`
}

type QueryInput struct {
	SQL string `json:"sql" jsonschema:"one read-only statement: SELECT, WITH, SHOW, DESCRIBE or EXPLAIN"`
}

type QueryOutput struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows" jsonschema:"one array per row, values in column order"`
	RowCount  int      `json:"row_count"`
	Truncated bool     `json:"truncated" jsonschema:"true when more rows matched than the row cap allows"`
}

var errNotRead = errors.New("refused: only read statements are allowed " +
	"(SELECT, WITH, SHOW, DESCRIBE/DESC or EXPLAIN); this server cannot change data or schema")

// NewServer returns an MCP server with list_tables, describe_table and query.
func NewServer(store Store, log *audit.Logger) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "mysql-mcp", Version: version}, nil)
	h := handlers{store: store, log: log}
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_tables",
		Description: "List the tables in the connected MySQL database with estimated row counts and comments. Start here.",
		Annotations: readOnly,
	}, h.listTables)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "describe_table",
		Description: "Show one table's columns (name, type, nullable, default, key) and indexes.",
		Annotations: readOnly,
	}, h.describeTable)
	mcp.AddTool(s, &mcp.Tool{
		Name: "query",
		Description: "Run one read-only SQL statement (SELECT, WITH, SHOW, DESCRIBE or EXPLAIN) and return its rows. " +
			"Writes are refused. Results are capped; truncated is true when more rows matched.",
		Annotations: readOnly,
	}, h.query)
	return s
}

type handlers struct {
	store Store
	log   *audit.Logger
}

func (h handlers) listTables(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListTablesOutput, error) {
	tables, err := h.store.ListTables(ctx)
	if err != nil {
		return nil, ListTablesOutput{}, err
	}
	out := ListTablesOutput{Tables: make([]TableInfo, len(tables))}
	for i, t := range tables {
		out.Tables[i] = TableInfo{Name: t.Name, EstimatedRows: t.EstimatedRows, Comment: t.Comment}
	}
	return nil, out, nil
}

func (h handlers) describeTable(ctx context.Context, _ *mcp.CallToolRequest, in DescribeTableInput) (*mcp.CallToolResult, DescribeTableOutput, error) {
	desc, err := h.store.DescribeTable(ctx, in.Table)
	if err != nil {
		return nil, DescribeTableOutput{}, err
	}
	out := DescribeTableOutput{
		Table:   desc.Name,
		Columns: make([]ColumnInfo, len(desc.Columns)),
		Indexes: make([]IndexInfo, len(desc.Indexes)),
	}
	for i, c := range desc.Columns {
		out.Columns[i] = ColumnInfo{Name: c.Name, Type: c.Type, Nullable: c.Nullable, Default: c.Default, Key: c.Key}
	}
	for i, x := range desc.Indexes {
		out.Indexes[i] = IndexInfo{Name: x.Name, Unique: x.Unique, Columns: x.Columns}
	}
	return nil, out, nil
}

func (h handlers) query(ctx context.Context, _ *mcp.CallToolRequest, in QueryInput) (*mcp.CallToolResult, QueryOutput, error) {
	if !guard.AllowedStatement(in.SQL) {
		return nil, QueryOutput{}, errNotRead
	}
	res, err := h.store.ReadOnlyQuery(ctx, in.SQL)
	if err != nil {
		return nil, QueryOutput{}, err
	}
	return nil, QueryOutput{
		Columns:   res.Columns,
		Rows:      res.Rows,
		RowCount:  len(res.Rows),
		Truncated: res.Truncated,
	}, nil
}
