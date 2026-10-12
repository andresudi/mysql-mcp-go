// Package guard holds the server's pure safety checks. Nothing here talks to
// the database, so every rule is covered by plain unit tests.
package guard

import "strings"

// allowedKeywords are the statement types the query tool accepts.
var allowedKeywords = map[string]bool{
	"SELECT":   true,
	"WITH":     true,
	"SHOW":     true,
	"DESCRIBE": true,
	"DESC":     true,
	"EXPLAIN":  true,
}

// AllowedStatement reports whether sql starts with a read keyword once
// leading whitespace and comments are stripped.
//
// It exists to give early, friendly errors. It is not the security boundary:
// "WITH ... DELETE" passes on purpose and is stopped by database privileges.
// A leading executable comment (/*! ... */) is rejected, because MySQL runs
// its contents.
func AllowedStatement(sql string) bool {
	rest, ok := skipLeadingComments(sql)
	if !ok {
		return false
	}
	end := 0
	for end < len(rest) && isLetter(rest[end]) {
		end++
	}
	if end == 0 || (end < len(rest) && isIdentChar(rest[end])) {
		return false
	}
	return allowedKeywords[strings.ToUpper(rest[:end])]
}

// skipLeadingComments strips whitespace and comments from the front of s.
// It returns false for an unterminated or executable comment.
func skipLeadingComments(s string) (string, bool) {
	for {
		s = strings.TrimLeft(s, " \t\r\n\f\v")
		switch {
		case strings.HasPrefix(s, "/*!"):
			return "", false
		case strings.HasPrefix(s, "/*"):
			end := strings.Index(s[2:], "*/")
			if end < 0 {
				return "", false
			}
			s = s[2+end+2:]
		case strings.HasPrefix(s, "#"), isDashComment(s):
			end := strings.IndexByte(s, '\n')
			if end < 0 {
				return "", true
			}
			s = s[end+1:]
		default:
			return s, true
		}
	}
}

// isDashComment reports whether s starts a "-- " comment. MySQL requires
// whitespace or a control character after the two dashes.
func isDashComment(s string) bool {
	return strings.HasPrefix(s, "--") && (len(s) == 2 || s[2] <= ' ')
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentChar(c byte) bool {
	return isLetter(c) || (c >= '0' && c <= '9') || c == '_' || c == '$'
}
