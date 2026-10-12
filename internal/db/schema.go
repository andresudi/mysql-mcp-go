package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrTableNotFound means the connected schema has no table by that name, or
// the user cannot see it.
var ErrTableNotFound = errors.New("table not found")

// Table is one row of ListTables.
type Table struct {
	Name          string
	EstimatedRows *int64 // nil for views
	Comment       string
}

// Column describes one column of a table.
type Column struct {
	Name     string
	Type     string
	Nullable bool
	Default  *string
	Key      string // PRI, UNI, MUL or empty
}

// Index describes one index of a table, with its columns in index order.
type Index struct {
	Name    string
	Unique  bool
	Columns []string
}

// TableDesc is the result of DescribeTable.
type TableDesc struct {
	Name    string
	Columns []Column
	Indexes []Index
}

// The schema always comes from DATABASE() and the table name is always a
// bound parameter: names are never concatenated into SQL.
const (
	listTablesSQL = `
		SELECT TABLE_NAME, TABLE_ROWS, TABLE_COMMENT
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		ORDER BY TABLE_NAME`
	columnsSQL = `
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, COLUMN_KEY
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`
	indexesSQL = `
		SELECT INDEX_NAME, NON_UNIQUE, COALESCE(COLUMN_NAME, EXPRESSION)
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY INDEX_NAME <> 'PRIMARY', INDEX_NAME, SEQ_IN_INDEX`
)

// ListTables returns the tables and views in the connected schema.
func (s *Store) ListTables(ctx context.Context) ([]Table, error) {
	rows, err := s.db.QueryContext(ctx, listTablesSQL)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer rows.Close()
	tables := []Table{}
	for rows.Next() {
		var t Table
		var est sql.NullInt64
		if err := rows.Scan(&t.Name, &est, &t.Comment); err != nil {
			return nil, fmt.Errorf("listing tables: %w", err)
		}
		if est.Valid {
			t.EstimatedRows = &est.Int64
		}
		tables = append(tables, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	return tables, nil
}

// DescribeTable returns the columns and indexes of one table.
func (s *Store) DescribeTable(ctx context.Context, name string) (TableDesc, error) {
	desc := TableDesc{Name: name, Columns: []Column{}, Indexes: []Index{}}

	rows, err := s.db.QueryContext(ctx, columnsSQL, name)
	if err != nil {
		return TableDesc{}, fmt.Errorf("describing %q: %w", name, err)
	}
	defer rows.Close()
	for rows.Next() {
		var c Column
		var nullable string
		var def sql.NullString
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &def, &c.Key); err != nil {
			return TableDesc{}, fmt.Errorf("describing %q: %w", name, err)
		}
		c.Nullable = nullable == "YES"
		if def.Valid {
			c.Default = &def.String
		}
		desc.Columns = append(desc.Columns, c)
	}
	if err := rows.Err(); err != nil {
		return TableDesc{}, fmt.Errorf("describing %q: %w", name, err)
	}
	if len(desc.Columns) == 0 {
		return TableDesc{}, fmt.Errorf("%w: %q", ErrTableNotFound, name)
	}

	idx, err := s.db.QueryContext(ctx, indexesSQL, name)
	if err != nil {
		return TableDesc{}, fmt.Errorf("describing %q: %w", name, err)
	}
	defer idx.Close()
	for idx.Next() {
		var indexName, column string
		var nonUnique int
		if err := idx.Scan(&indexName, &nonUnique, &column); err != nil {
			return TableDesc{}, fmt.Errorf("describing %q: %w", name, err)
		}
		if n := len(desc.Indexes); n > 0 && desc.Indexes[n-1].Name == indexName {
			desc.Indexes[n-1].Columns = append(desc.Indexes[n-1].Columns, column)
			continue
		}
		desc.Indexes = append(desc.Indexes, Index{Name: indexName, Unique: nonUnique == 0, Columns: []string{column}})
	}
	if err := idx.Err(); err != nil {
		return TableDesc{}, fmt.Errorf("describing %q: %w", name, err)
	}
	return desc, nil
}
