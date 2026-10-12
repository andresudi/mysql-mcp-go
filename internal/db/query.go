package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Result is the outcome of one ReadOnlyQuery. Each row holds one value per
// column, in column order. Rows is never nil.
type Result struct {
	Columns   []string
	Rows      [][]any
	Truncated bool // more than MaxRows rows were available
}

// ReadOnlyQuery runs one statement inside a read-only transaction that is
// always rolled back, with MAX_EXECUTION_TIME set and at most MaxRows rows
// returned.
//
// It does not check the statement type; callers do that for friendlier
// errors. What stops writes here is the user's privileges, which are checked
// at startup, and the read-only transaction.
func (s *Store) ReadOnlyQuery(ctx context.Context, query string) (Result, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()

	// The value is an integer from config, never user input.
	setTimeout := fmt.Sprintf("SET SESSION max_execution_time = %d", s.lim.Timeout.Milliseconds())
	if _, err := tx.ExecContext(ctx, setTimeout); err != nil {
		return Result{}, err
	}

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return Result{}, err
	}
	defer rows.Close()

	types, err := rows.ColumnTypes()
	if err != nil {
		return Result{}, err
	}
	res := Result{Columns: make([]string, len(types)), Rows: [][]any{}}
	raw := make([]sql.RawBytes, len(types))
	dest := make([]any, len(types))
	for i, ct := range types {
		res.Columns[i] = ct.Name()
		dest[i] = &raw[i]
	}
	for rows.Next() {
		if len(res.Rows) == s.lim.MaxRows {
			res.Truncated = true
			break
		}
		if err := rows.Scan(dest...); err != nil {
			return Result{}, err
		}
		row := make([]any, len(raw))
		for i, b := range raw {
			row[i] = convertValue(types[i].DatabaseTypeName(), b)
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return Result{}, err
	}
	return res, nil
}
