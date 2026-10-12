package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Limits bound each call to ReadOnlyQuery.
type Limits struct {
	MaxRows int           // rows returned before the result is marked truncated
	Timeout time.Duration // MAX_EXECUTION_TIME for each query
}

// Store is the server's connection to MySQL.
type Store struct {
	db  *sql.DB
	lim Limits
}

// Open connects to MySQL with safe DSN options and checks the connection.
func Open(ctx context.Context, dsn string, lim Limits) (*Store, error) {
	dsn, err := normalizeDSN(dsn)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to MySQL: %w", err)
	}
	return &Store{db: db, lim: lim}, nil
}

// Close closes the connection pool.
func (s *Store) Close() error {
	return s.db.Close()
}

// Grants returns the lines of SHOW GRANTS FOR CURRENT_USER().
func (s *Store) Grants(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SHOW GRANTS FOR CURRENT_USER()")
	if err != nil {
		return nil, fmt.Errorf("reading grants: %w", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("reading grants: %w", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading grants: %w", err)
	}
	return lines, nil
}
