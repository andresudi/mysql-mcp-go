// Package db runs the server's read-only access to MySQL.
package db

import (
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

// normalizeDSN parses dsn and turns off options that would weaken the safety
// model: multiStatements would let "SELECT 1; DROP TABLE t" run as two
// statements, and interpolateParams would build SQL text on the client.
func normalizeDSN(dsn string) (string, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		// The driver's parse errors never include the DSN, and so never the
		// password. Keep it that way: do not add dsn to this message.
		return "", fmt.Errorf("invalid MYSQL_DSN: %w", err)
	}
	if cfg.DBName == "" {
		return "", errors.New("invalid MYSQL_DSN: it must name a database, e.g. .../demo")
	}
	cfg.MultiStatements = false
	cfg.InterpolateParams = false
	return cfg.FormatDSN(), nil
}
