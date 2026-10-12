package guard

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// grantLine matches "GRANT <privileges> ON <object> TO <user>" with nothing
// after the user, so "WITH GRANT OPTION" does not match.
var grantLine = regexp.MustCompile(`^GRANT (.+?) ON (\S+) TO (\S+)$`)

// readPrivileges are the only privileges a grant line may hold.
var readPrivileges = map[string]bool{
	"SELECT":    true,
	"SHOW VIEW": true,
	"USAGE":     true,
}

// CheckGrants returns nil only if every line from SHOW GRANTS gives no more
// than SELECT, SHOW VIEW or USAGE. It fails closed: role grants, grant
// options, column-level grants, partial revokes and any line it cannot parse
// are errors, and so is an empty list.
func CheckGrants(lines []string) error {
	if len(lines) == 0 {
		return errors.New("SHOW GRANTS returned nothing, so the user cannot be confirmed read-only")
	}
	for _, line := range lines {
		m := grantLine.FindStringSubmatch(line)
		if m == nil {
			return fmt.Errorf("grant is not a plain SELECT/SHOW VIEW grant: %q", line)
		}
		for _, p := range strings.Split(m[1], ",") {
			if !readPrivileges[strings.TrimSpace(p)] {
				return fmt.Errorf("grant allows more than SELECT/SHOW VIEW: %q", line)
			}
		}
	}
	return nil
}
