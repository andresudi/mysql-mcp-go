PUBLIC

# mysql-mcp-go — design

A small MCP server, written in Go, that lets Claude (or any MCP client) explore and query a
MySQL database **read-only**. It is a portfolio piece: it must be safe to point at a real
database, demonstrable in under 90 seconds, and readable by a client reviewing the code.

## Outcome and success criteria

| # | Criterion |
|---|-----------|
| 1 | One command connects it to Claude Code or Claude Desktop (stdio transport). |
| 2 | Claude can answer a question about the sample database using only the three tools below. |
| 3 | No tool call can change data or schema, even when the SQL is crafted to try. |
| 4 | The server refuses to start when its MySQL user can do more than read. |
| 5 | `go test ./...` passes locally and in CI against a real MySQL 8. |
| 6 | README explains setup, the safety model, and links a 60–90 s demo. |

## Current state

| Piece | Status |
|-------|--------|
| Repository `github.com/andresudi/practice2`, to be renamed `mysql-mcp-go` and made public at launch | EXISTS, private; contents not inspected |
| Go module, MCP server, tools | MISSING |
| Sample database and read-only user | MISSING |
| Tests and CI | MISSING |

## Tools

| Tool | Input | Output |
|------|-------|--------|
| `list_tables` | none | tables in the connected schema: name, estimated row count, comment |
| `describe_table` | `table` | columns (name, type, nullable, default, key) and indexes |
| `query` | `sql` | `{columns, rows, row_count, truncated}` for one read statement |

`list_tables` and `describe_table` read `information_schema` with bound parameters and the
schema from `DATABASE()`; a table name is never concatenated into SQL. Values come back as
JSON; `[]byte` columns are returned as strings.

## Safety model

Four layers. Only the first is the security boundary; the rest give earlier, clearer errors
and bound resource use.

1. **Database privileges (the boundary).** The server connects as a user holding only
   `SELECT` and `SHOW VIEW` (plus the implicit `USAGE`). At startup it runs
   `SHOW GRANTS FOR CURRENT_USER()` and exits with a clear message if any grant line is
   anything else — including `ALL PRIVILEGES`, `INSERT`, `FILE`, `EXECUTE`, role grants,
   or a line it cannot parse. It fails closed.
2. **Statement allow-list.** The first keyword, after stripping leading whitespace and
   comments, must be `SELECT`, `WITH`, `SHOW`, `DESCRIBE`/`DESC` or `EXPLAIN`. The driver's
   `multiStatements` option stays off, so `SELECT 1; DROP TABLE t` is rejected by MySQL.
   `WITH … DELETE` passes this filter by design; layers 1 and 3 stop it.
3. **Per-query limits.** Each `query` call runs inside `START TRANSACTION READ ONLY`, sets
   `MAX_EXECUTION_TIME` (default 5000 ms), reads at most `MAX_ROWS + 1` rows (default 200)
   to set `truncated`, and always rolls back.
4. **Audit log.** One JSON line per tool call: time, tool, SQL, row count, duration, error.

**Rejected: relying on a read-only transaction alone.** In MySQL, DDL causes an implicit
commit. `START TRANSACTION READ ONLY; DROP TABLE t;` ends the read-only transaction and
then drops the table. Only privileges stop that, which is why layer 1 is the boundary.

**Rejected: parsing SQL to decide what is safe.** A keyword check is fooled by comments,
string literals and `WITH`-prefixed writes; a full MySQL parser is a large dependency for
a guarantee privileges already give. The allow-list exists for friendly errors only.

## Configuration

| Variable | Default | Meaning |
|----------|---------|---------|
| `MYSQL_DSN` | required | go-sql-driver format, e.g. `mcp_ro:mcp_ro@tcp(127.0.0.1:3316)/demo` |
| `MCP_MAX_ROWS` | `200` | row cap per `query` |
| `MCP_TIMEOUT_MS` | `5000` | `MAX_EXECUTION_TIME` per `query` |
| `MCP_AUDIT_LOG` | stderr | file path for the audit log |

stdout is the MCP stdio channel. Nothing else may write to it; logs go to stderr or the
audit file.

## Layout

```
cmd/mysql-mcp/main.go        load config, connect, grant check, register tools, serve stdio
internal/config/             env parsing and defaults
internal/guard/              AllowedStatement(sql) bool; CheckGrants(lines []string) error — pure, no DB
internal/db/                 Open; ListTables; DescribeTable; ReadOnlyQuery
internal/tools/              the three MCP handlers; map db results to tool output
internal/audit/              JSON-lines writer
testdata/seed.sql            sample schema, generated rows, read-only user
docker-compose.yml           mysql:8.0 on host port 3316, runs seed.sql on first start
.github/workflows/test.yml   go test with a mysql:8.0 service container
README.md, LICENSE (MIT)
```

## Sample data

A fictional project tracker, so the demo touches no real or sensitive domain:
`users` (~50), `projects` (~8), `tasks` (~300, with status and due date, some overdue),
`comments` (~600). Rows are generated in `seed.sql` with recursive CTEs, so every fresh
container has the same data. Due dates are offset from `CURDATE()` at seed time so
"overdue" stays meaningful. `seed.sql` also creates `mcp_ro` with `SELECT, SHOW VIEW` on
`demo.*`.

## Testing

| Level | Needs MySQL | Cases |
|-------|-------------|-------|
| Unit — `guard` | no | allow-list accepts `SELECT`, lowercase, leading comments, `WITH`; rejects `INSERT`, `UPDATE`, `DELETE`, `DROP`, `CALL`, `SET`, `LOAD`. Grants parser accepts SELECT-only; rejects `ALL PRIVILEGES`, `INSERT`, `FILE`, role grants, unparseable lines |
| Integration — `db` | yes, skipped unless `MYSQL_TEST_DSN` is set | `DROP` via `query` fails and the table survives; root user fails the grant check; row cap sets `truncated`; `SELECT SLEEP(10)` hits the timeout |
| End-to-end | yes | an in-memory MCP client calls all three tools against the seeded database |

## Demo script (60–90 s)

1. `docker compose up -d`, then `claude mcp add mysql -- ./mysql-mcp` with `MYSQL_DSN` set.
2. Ask: "Which project has the most overdue tasks, and who owns them?" — Claude lists,
   describes, then queries.
3. Ask: "Delete those overdue tasks." — refused; show the audit-log line.
4. Restart with the root DSN — the server refuses to start.

## Non-goals

Remote HTTP transport and hosting, authentication, Postgres support, write tools,
per-table allow-lists. Each is a follow-up if a client asks.

## Assumptions to confirm at planning time

- **MCP SDK.** Use the official Go SDK, `github.com/modelcontextprotocol/go-sdk`. Its exact
  API for registering tools, the stdio transport and in-memory transports is not verified
  here; read its README and pin the version in `go.mod` before writing handlers.
- **MySQL version.** Target 8.0. `START TRANSACTION READ ONLY` and `MAX_EXECUTION_TIME`
  also exist in 5.7, but 5.7 is untested and not claimed in the README.
- **Build environment.** A Claude Code cloud session on the author's personal account,
  which provides Go and Docker, so tests and `docker compose` run there.
