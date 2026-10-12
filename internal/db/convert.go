package db

import (
	"strconv"
	"strings"
)

// convertValue turns one text-protocol value into a JSON-friendly Go value,
// using the column's database type name as reported by the driver.
// Integers and floats become numbers; everything else, DECIMAL included,
// becomes a string so no precision is lost. NULL becomes nil.
func convertValue(dbType string, raw []byte) any {
	if raw == nil {
		return nil
	}
	s := string(raw)
	switch dbType {
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "BIGINT", "YEAR":
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
	case "FLOAT", "DOUBLE":
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	default:
		if strings.HasPrefix(dbType, "UNSIGNED ") {
			if n, err := strconv.ParseUint(s, 10, 64); err == nil {
				return n
			}
		}
	}
	return s
}
