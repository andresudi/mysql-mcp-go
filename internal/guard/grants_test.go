package guard

import (
	"strings"
	"testing"
)

// Grant lines as MySQL 8.0 prints them for SHOW GRANTS FOR CURRENT_USER().
const (
	roUsage = "GRANT USAGE ON *.* TO `mcp_ro`@`%`"
	roRead  = "GRANT SELECT, SHOW VIEW ON `demo`.* TO `mcp_ro`@`%`"

	rootStatic  = "GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, RELOAD, SHUTDOWN, PROCESS, FILE, REFERENCES, INDEX, ALTER, SHOW DATABASES, SUPER, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, REPLICATION SLAVE, REPLICATION CLIENT, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, CREATE USER, EVENT, TRIGGER, CREATE TABLESPACE, CREATE ROLE, DROP ROLE ON *.* TO `root`@`localhost` WITH GRANT OPTION"
	rootDynamic = "GRANT APPLICATION_PASSWORD_ADMIN,AUDIT_ABORT_EXEMPT,AUDIT_ADMIN,BACKUP_ADMIN,SYSTEM_USER,SYSTEM_VARIABLES_ADMIN,XA_RECOVER_ADMIN ON *.* TO `root`@`localhost` WITH GRANT OPTION"
	rootProxy   = "GRANT PROXY ON ``@`` TO `root`@`localhost` WITH GRANT OPTION"
)

func TestCheckGrantsAccepts(t *testing.T) {
	tests := [][]string{
		{roUsage, roRead},
		{roUsage},
		{"GRANT SELECT ON *.* TO `mcp_ro`@`%`"},
		{roUsage, "GRANT SELECT ON `demo`.`tasks` TO `mcp_ro`@`%`"},
	}
	for _, lines := range tests {
		if err := CheckGrants(lines); err != nil {
			t.Errorf("CheckGrants(%q) = %v, want nil", lines, err)
		}
	}
}

func TestCheckGrantsRejects(t *testing.T) {
	tests := []string{
		rootStatic,
		rootDynamic,
		rootProxy,
		"GRANT ALL PRIVILEGES ON `demo`.* TO `mcp_ro`@`%`",
		"GRANT INSERT ON `demo`.* TO `mcp_ro`@`%`",
		"GRANT FILE ON *.* TO `mcp_ro`@`%`",
		"GRANT EXECUTE ON `demo`.* TO `mcp_ro`@`%`",
		"GRANT SELECT, INSERT ON `demo`.* TO `mcp_ro`@`%`",
		"GRANT `reader`@`%` TO `mcp_ro`@`%`",
		"GRANT SELECT ON `demo`.* TO `mcp_ro`@`%` WITH GRANT OPTION",
		"GRANT SELECT (`id`) ON `demo`.`t` TO `mcp_ro`@`%`",
		"REVOKE SELECT ON `mysql`.* FROM `mcp_ro`@`%`",
		"garbage",
		"",
	}
	for _, bad := range tests {
		// The bad line sits between two good ones, so the check has to look
		// at every line.
		err := CheckGrants([]string{roUsage, bad, roRead})
		if err == nil {
			t.Errorf("CheckGrants accepted %q", bad)
			continue
		}
		if !strings.Contains(err.Error(), bad) {
			t.Errorf("error %q does not quote the offending line %q", err, bad)
		}
	}
}

func TestCheckGrantsRejectsEmpty(t *testing.T) {
	if err := CheckGrants(nil); err == nil {
		t.Error("CheckGrants(nil) = nil, want error")
	}
}
