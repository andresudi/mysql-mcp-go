package guard

import "testing"

func TestAllowedStatement(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"SELECT 1", true},
		{"select * from t", true},
		{"  \n\tSELECT 1", true},
		{"-- note\nSELECT 1", true},
		{"# note\nSELECT 1", true},
		{"/* note */ SELECT 1", true},
		{"/* a */ -- b\n SELECT 1", true},
		{"SELECT\n*\nFROM t", true},
		{"WITH x AS (SELECT 1) SELECT * FROM x", true},
		{"SHOW TABLES", true},
		{"DESCRIBE t", true},
		{"DESC t", true},
		{"EXPLAIN SELECT 1", true},
		// Passes by design: privileges and the read-only transaction stop it.
		{"WITH x AS (SELECT 1) DELETE FROM t", true},

		{"INSERT INTO t VALUES (1)", false},
		{"UPDATE t SET a=1", false},
		{"DELETE FROM t", false},
		{"DROP TABLE t", false},
		{"CALL p()", false},
		{"SET @a = 1", false},
		{"LOAD DATA INFILE 'x' INTO TABLE t", false},
		{"TRUNCATE t", false},
		{"GRANT SELECT ON *.* TO u", false},
		{"ALTER TABLE t ADD c INT", false},
		{"", false},
		{"   \n\t", false},
		{"-- only a comment", false},
		{"/* only a comment */", false},
		{"/*!50000 DROP TABLE t */", false},
		{"/*! DELETE FROM t */ SELECT 1", false},
		{"SELECTX 1", false},
		{"SELECT_1", false},
		{"/* unterminated SELECT 1", false},
		// MySQL only treats "--" as a comment when whitespace follows.
		{"--SELECT 1", false},
	}
	for _, tt := range tests {
		if got := AllowedStatement(tt.sql); got != tt.want {
			t.Errorf("AllowedStatement(%q) = %v, want %v", tt.sql, got, tt.want)
		}
	}
}
