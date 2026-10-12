# mysql-mcp-go — implementation plan

Spec: [`docs/superpowers/specs/2026-10-12-mysql-mcp-go-design.md`](../specs/2026-10-12-mysql-mcp-go-design.md)

**Goal.** Build the read-only MySQL MCP server described in the spec, task by task. Each task
writes a failing test first, adds only the code needed to pass it, then commits.

**Per-task loop.** Write the test and run it to see it fail for the expected reason.
Implement the minimum, then run `go test ./...`, `go vet ./...` and `gofmt -l .`
(which must print nothing). Commit with the message shown in the task and push to
`claude/youthful-gauss-x3vnnj`.

**Two test modes.**

```sh
go test ./...                                   # unit only; integration tests skip
docker compose up -d --wait                     # seeded MySQL 8.0 on :3316
MYSQL_TEST_DSN='mcp_ro:mcp_ro@tcp(127.0.0.1:3316)/demo' \
MYSQL_TEST_ROOT_DSN='root:root@tcp(127.0.0.1:3316)/demo' \
go test ./...                                   # unit + integration + end-to-end
```

---

## Planning-time findings

I checked the spec's three assumptions, plus some MySQL behaviors the tests depend on.
I ran each check against MySQL 8.0.46 in a throwaway container, or read it from the
pinned library source.

| # | Finding | Effect on the plan |
|---|---------|--------------------|
| F1 | **MCP SDK:** I'm pinning `github.com/modelcontextprotocol/go-sdk` **v1.8.0**, the latest stable. I checked the API in its source. `mcp.NewServer`, generic `mcp.AddTool(s, &mcp.Tool{…}, func(ctx, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error))` and `server.Run(ctx, &mcp.StdioTransport{})` all exist. `mcp.NewInMemoryTransports()` exists too (connect the server before the client), and so does `mcp.CommandTransport`. When a handler returns a plain `error`, the client gets a result with `IsError: true`, not a protocol error. That is how refusals reach Claude. Input and output schemas come from struct types and `jsonschema:"…"` tags, and `ToolAnnotations.ReadOnlyHint` exists. | Handlers are typed `AddTool` functions. Tests use in-memory transports. |
| F2 | **Go version:** SDK v1.6.1 and later need `go 1.25.0`. The container has 1.24.7, and with `GOTOOLCHAIN=auto` it downloads 1.25 without trouble (verified). Driver: `github.com/go-sql-driver/mysql` **v1.10.1**, which needs go 1.24. | `go.mod` says `go 1.25.0`. CI uses `go-version-file: go.mod`. |
| F3 | **Docker:** At session start the daemon wasn't running. Running `dockerd` by hand works. Docker Hub answered `429 Too Many Requests` for `mysql:8.0`, but `mirror.gcr.io/library/mysql:8.0` pulled fine. | In this container only: `docker tag mirror.gcr.io/library/mysql:8.0 mysql:8.0`. `docker-compose.yml` keeps `mysql:8.0` as the spec says. |
| F4 | **Repo state:** The repo is already `andresudi/mysql-mcp-go`. It was empty, with no commits before the spec. The spec's "Current state" row about `practice2` is out of date. I committed the spec verbatim anyway. | Nothing to rename. Making it public is your launch step. |
| F5 | **Timeout:** under `max_execution_time`, `SELECT SLEEP(10)` doesn't error. It stops at the limit and returns `1`, which is documented MySQL behavior when `SLEEP` is the whole query. `SELECT COUNT(*) FROM t WHERE SLEEP(1)` fails with `ERROR 3024 … maximum statement execution time exceeded`. | The timeout test checks both: `SLEEP(10)` returns quickly with value 1, and `WHERE SLEEP(1)` returns error 3024. |
| F6 | **Grant lines:** `mcp_ro` shows `` GRANT USAGE ON *.* TO `mcp_ro`@`%` `` and `` GRANT SELECT, SHOW VIEW ON `demo`.* TO `mcp_ro`@`%` ``. Root shows a static-privilege line `… WITH GRANT OPTION`, a dynamic-privilege line separated by commas with no spaces, and ``` GRANT PROXY ON ``@`` … ```. | These are the exact fixtures for `CheckGrants`. |
| F7 | **Layers:** as `mcp_ro`, `DROP TABLE` and `WITH x AS (SELECT 1) DELETE FROM t` fail with privilege error 1142. As root inside `START TRANSACTION READ ONLY`, `DELETE` fails (1792), **but `DROP TABLE` succeeds**. That confirms the spec's "rejected" reasoning. | The crafted-write test runs as `mcp_ro` and checks that data survives. |
| F8 | **Values:** with no bind args, the driver uses the text protocol, so every non-NULL value arrives as `[]byte`. Turning all `[]byte` into strings would make numbers into strings too. | Values are converted by column type (Task 7): integers and floats become JSON numbers; `DECIMAL`, dates, text and binary become strings; NULL becomes `null`. |
| F9 | **Row estimates:** `information_schema.TABLES.TABLE_ROWS` is a cached statistic in 8.0. | `seed.sql` ends with `ANALYZE TABLE`, so `list_tables` shows real estimates on a fresh container. |
| F10 | **Startup:** the mysql image runs init scripts on a socket-only temporary server, then restarts with TCP. | The compose healthcheck uses TCP (`mysqladmin ping -h 127.0.0.1`), so `--wait` returns only after the seed has run. A CI service container can't mount files before checkout, so CI loads `seed.sql` with the `mysql` client instead. |

### Decisions I made that you may want to change

1. **`MYSQL_TEST_ROOT_DSN`:** a second test variable, added for the spec's "root user fails the grant
   check" case. That subtest skips when the variable is unset.
2. **Value typing:** as in F8. `DECIMAL` stays a string so no precision is lost.
3. **`query` output shape:** `rows` is an array of arrays in `columns` order, so duplicate
   column names (`SELECT 1, 1`) still work.
4. **Grant check fails closed on more than the spec lists:** it also rejects
   `WITH GRANT OPTION`, column-level grants (`SELECT (col)`), partial-revoke lines and an
   empty grant list. `SELECT ON *.*` passes because it is still read-only. The README will
   recommend scoping it to one schema, since `*.*` can read the `mysql` system tables.
5. **Executable comments:** the allow-list rejects any statement whose leading comments include an
   executable comment (`/*! … */`), because MySQL runs its contents.
6. **Where the allow-list runs:** in the `query` tool handler, not in `db.ReadOnlyQuery`. That
   lets the crafted-write integration test hit the database layer directly and show that
   privileges alone hold.

---

## Tasks

### Task 1 — Module scaffold
- **Files:** `go.mod` (`module github.com/andresudi/mysql-mcp-go`, `go 1.25.0`), `LICENSE`
  (MIT, 2026, andresudi), `.gitignore` (binary, `*.log`).
- **Check:** `go list -m` prints the module path. There's nothing to test yet.
  `go mod tidy` drops unused requirements, so each dependency is pinned with
  `go get …@version` when code first imports it: driver v1.10.1 in Task 6, SDK v1.8.0 in
  Task 13.
- **Commit:** `Scaffold Go module and MIT license`

### Task 2 — `guard.AllowedStatement(sql string) bool`
- **Files:** `internal/guard/statement.go`, `internal/guard/statement_test.go`
- **Test first:** a table-driven test, with no database.

  | Accept | Reject |
  |--------|--------|
  | `SELECT 1` | `INSERT INTO t VALUES (1)` |
  | `select * from t` | `UPDATE t SET a=1` |
  | `  \n\tSELECT 1` | `DELETE FROM t` |
  | `-- note\nSELECT 1` | `DROP TABLE t` |
  | `# note\nSELECT 1` | `CALL p()` |
  | `/* note */ SELECT 1` | `SET @a = 1` |
  | `/* a */ -- b\n SELECT 1` | `LOAD DATA INFILE 'x' INTO TABLE t` |
  | `WITH x AS (SELECT 1) SELECT * FROM x` | empty string, whitespace only, comment only |
  | `SHOW TABLES` | `/*!50000 DROP TABLE t */` |
  | `DESCRIBE t`, `DESC t` | `/*! DELETE FROM t */ SELECT 1` |
  | `EXPLAIN SELECT 1` | `SELECTX 1`, `/* unterminated SELECT 1` |
  | `WITH x AS (SELECT 1) DELETE FROM t` (passes by design) | `TRUNCATE t`, `GRANT …`, `ALTER …` |
- **Implement:** strip leading whitespace and comments (`--`, `#`, `/* */`). Reject if any of
  those comments is `/*!`. Read the leading letters, uppercase them, and look them up in the
  allow-set. The keyword has to end at a non-identifier character or at end of input.
- **Commit:** `Add statement allow-list guard`

### Task 3 — `guard.CheckGrants(lines []string) error`
- **Files:** `internal/guard/grants.go`, `internal/guard/grants_test.go`
- **Test first:** all cases use the exact strings from F6.
  - Accept: the two `mcp_ro` lines; a lone `USAGE` line; `GRANT SELECT ON *.* TO …`.
  - Reject, each with an error that quotes the offending line:
    - root's three lines, one at a time
    - `` GRANT ALL PRIVILEGES ON `demo`.* TO … ``
    - `GRANT INSERT ON …`, `GRANT FILE ON *.* …`, `GRANT EXECUTE ON …`
    - `GRANT SELECT, INSERT ON …` (one bad privilege in a list)
    - the role grant `` GRANT `reader`@`%` TO `mcp_ro`@`%` ``
    - `` GRANT SELECT ON `demo`.* TO … WITH GRANT OPTION ``
    - `` GRANT SELECT (`id`) ON `demo`.`t` TO … ``
    - `REVOKE …` and `garbage`
    - an empty list
- **Implement:** match each line against `^GRANT (.+?) ON (\S+) TO (\S+)$`. Split the privilege
  list on `,` and trim each entry. Every entry must be `SELECT`, `SHOW VIEW` or `USAGE`.
  Anything else, including a line that doesn't match, is an error.
- **Commit:** `Add fail-closed grants checker`

### Task 4 — `config.Load(getenv func(string) string) (Config, error)`
- **Files:** `internal/config/config.go`, `internal/config/config_test.go`
- **Test first:**
  - Only `MYSQL_DSN` set: defaults are `MaxRows=200`, `Timeout=5s`, `AuditLogPath=""` (stderr).
  - Every variable set: each one is applied.
  - `MYSQL_DSN` missing: the error names `MYSQL_DSN`.
  - `MCP_MAX_ROWS` set to `0`, `-1` or `abc` is an error, and so is `MCP_TIMEOUT_MS=0`.
    Each error names its variable.
- **Commit:** `Add environment config loader`

### Task 5 — `audit.Logger`
- **Files:** `internal/audit/audit.go`, `internal/audit/audit_test.go`
- **API:** `audit.New(w io.Writer) *Logger`; `(*Logger).Log(Entry)`.
  `Entry{Time, Tool, SQL, Table, RowCount, Duration, Err}` is written as one JSON line with
  these fields:
  - `time` (RFC 3339 with milliseconds)
  - `tool`
  - `sql` (omitempty)
  - `table` (omitempty)
  - `row_count`
  - `duration_ms`
  - `error` (omitempty)
- **Test first:**
  - One `Log` call writes exactly one `\n`-terminated line, and it decodes to the expected
    fields.
  - An SQL string containing a newline or quotes stays on one line.
  - Fifty concurrent `Log` calls produce fifty lines that each parse (run with `-race`).
- **Commit:** `Add JSON-lines audit logger`

### Task 6 — `db.normalizeDSN(dsn string) (string, error)` (pure)
- **Files:** `internal/db/dsn.go`, `internal/db/dsn_test.go`
- **Test first:**
  - `…/demo?multiStatements=true` comes back with multi-statements off.
  - A DSN with no database name is an error ("DSN must name a database").
  - An unparseable DSN is an error, and the error message doesn't contain the password.
- **Implement:** `mysql.ParseDSN`, then set `MultiStatements = false` and
  `InterpolateParams = false`, then `FormatDSN`.
- **Commit:** `Force safe DSN options`

### Task 7 — `db.convertValue(dbType string, raw []byte) any` (pure)
- **Files:** `internal/db/convert.go`, `internal/db/convert_test.go`
- **Test first:**

  | Input | Output |
  |-------|--------|
  | `INT "42"` | `int64(42)` |
  | `BIGINT "-9"` | `int64(-9)` |
  | `UNSIGNED BIGINT "18446744073709551615"` | `uint64(…)` |
  | `DOUBLE "1.5"` | `float64(1.5)` |
  | `DECIMAL "12.30"` | `"12.30"` |
  | `DATE "2026-10-12"` | string |
  | `VARCHAR`, `JSON`, `BLOB` | strings |
  | `nil` | `nil` |
  | anything that fails to parse | the original string |
- **Commit:** `Add type-aware value conversion`

### Task 8 — Sample database: `testdata/seed.sql` and `docker-compose.yml`
- **Files:**
  - `testdata/seed.sql`
  - `docker-compose.yml`: `mysql:8.0`, `3316:3306`, `MYSQL_ROOT_PASSWORD=root`, seed
    mounted into `/docker-entrypoint-initdb.d/`, TCP healthcheck
  - `internal/db/testhelpers_test.go`: `testDSN(t)` and `rootDSN(t)`, which call `t.Skip`
    when their variable is unset
  - `internal/db/seed_test.go`
- **Test first (integration):** written as `mcp_ro` against the seeded database.
  - Exact row counts: users 50, projects 8, tasks 300, comments 600.
  - At least 20 tasks are overdue (`due_date < CURDATE() AND status <> 'done'`).
  - Exactly one project has the most overdue tasks, ahead of the runner-up by at least 3.
    That keeps the demo answer unambiguous.
- **Seed contents:**
  - `CREATE DATABASE IF NOT EXISTS demo`.
  - Tables `users`, `projects` (`owner_id`), `tasks` (`project_id`, `assignee_id`, `title`,
    `status` enum `todo`/`in_progress`/`done`, `due_date`) and `comments` (`task_id`,
    `author_id`, `body`, `created_at`).
  - Foreign keys, an index on `tasks(status, due_date)`, and table comments.
  - Rows come from recursive CTEs with arithmetic (`MOD`) formulas, not `RAND()`. Names come
    from small fictional `ELT()` lists. Due dates are `CURDATE() ± N DAY`.
  - `CREATE USER IF NOT EXISTS 'mcp_ro'@'%' IDENTIFIED BY 'mcp_ro'; GRANT SELECT, SHOW VIEW ON demo.* TO 'mcp_ro'@'%';`
  - `ANALYZE TABLE …` at the end.
  - I'll adjust the formulas until the "unique top project" test passes.
- **Check:** `docker compose down -v && docker compose up -d --wait`, then run the
  integration tests.
- **Commit:** `Add deterministic sample database and compose file`

### Task 9 — `db.Open` and `(*Store).Grants`
- **Files:** `internal/db/store.go`, `internal/db/store_test.go`
- **API:** `db.Open(ctx, dsn string, lim Limits) (*Store, error)`, where
  `Limits{MaxRows int; Timeout time.Duration}`. It normalizes the DSN (Task 6), opens the
  connection and pings with a 10 s timeout. `(*Store).Grants(ctx) ([]string, error)` runs
  `SHOW GRANTS FOR CURRENT_USER()`.
- **Test first (integration):**
  - With `MYSQL_TEST_DSN`, `Grants` passes `guard.CheckGrants`.
  - With `MYSQL_TEST_ROOT_DSN`, `CheckGrants` returns an error that mentions `INSERT`.
  - A DSN pointing at a closed port makes `Open` fail within the timeout, and the error has
    no password in it.
- **Commit:** `Connect to MySQL and read current grants`

### Task 10 — `ListTables` and `DescribeTable`
- **Files:** `internal/db/schema.go`, `internal/db/schema_test.go`
- **API:**
  - `ListTables(ctx) ([]Table, error)` with `Table{Name, EstimatedRows *int64, Comment}`.
  - `DescribeTable(ctx, name) (TableDesc, error)` with
    `TableDesc{Columns []Column{Name, Type, Nullable, Default *string, Key}, Indexes []Index{Name, Unique, Columns []string}}`.
  - `ErrTableNotFound`.
- **Test first (integration):**
  - `ListTables` returns the four seed tables in name order, each with an estimate above 0
    and its comment.
  - `DescribeTable("tasks")` lists columns in ordinal order: `id` is the `PRI` key,
    `due_date` is nullable, and `status` has type `enum(…)`. Indexes include `PRIMARY`
    (unique) and the `(status, due_date)` index with its columns in order.
  - `DescribeTable("nope")` returns `ErrTableNotFound`.
  - `DescribeTable("tasks; DROP TABLE users")` returns `ErrTableNotFound`, and `users` still
    exists.
- **Implement:** queries on `information_schema.TABLES`, `COLUMNS` and `STATISTICS` with
  `WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`. Nothing is concatenated into SQL.
- **Commit:** `List and describe tables via information_schema`

### Task 11 — `ReadOnlyQuery`: results and limits
- **Files:** `internal/db/query.go`, `internal/db/query_test.go`
- **API:** `ReadOnlyQuery(ctx, sql) (Result, error)` with
  `Result{Columns []string, Rows [][]any, Truncated bool}`. `Rows` is never nil.
- **Test first (integration):**
  - `SELECT id, title, due_date FROM tasks ORDER BY id LIMIT 3`: three rows, `id` is an
    `int64`, `due_date` is a string, `Truncated` is false.
  - With `Limits.MaxRows=10`, `SELECT * FROM tasks` returns 10 rows and `Truncated` is true.
    A query returning exactly 10 rows has `Truncated` false.
  - `SHOW TABLES`, `DESCRIBE tasks` and `EXPLAIN SELECT * FROM tasks` all return rows.
  - With `Limits.Timeout=500ms`:
    - `SELECT SLEEP(10)` returns within 3 s with value `1` (F5).
    - `SELECT COUNT(*) FROM users WHERE SLEEP(1)` returns an error containing `3024`,
      within 3 s.
- **Implement:**
  1. `BeginTx(ctx, &sql.TxOptions{ReadOnly: true})`; the driver sends
     `START TRANSACTION READ ONLY`.
  2. `defer tx.Rollback()`.
  3. `SET SESSION max_execution_time = <ms>`, with the integer from config formatted in.
  4. `QueryContext` with no args.
  5. Scan at most `MaxRows+1` rows and convert each value with `convertValue`.
- **Commit:** `Run read-only queries with row cap and timeout`

### Task 12 — `ReadOnlyQuery`: crafted writes cannot change anything (criterion 3)
- **Files:** `internal/db/query_attack_test.go`
- **Test first (integration, as `mcp_ro`, bypassing the allow-list on purpose):** each statement below must return an error.

  | Statement | Expected stopper |
  |-----------|------------------|
  | `DROP TABLE users` | privileges |
  | `DELETE FROM tasks` | privileges / read-only tx |
  | `WITH x AS (SELECT 1) DELETE FROM tasks` | privileges |
  | `EXPLAIN ANALYZE DELETE FROM tasks` | privileges |
  | `SELECT 1; DROP TABLE users` | multi-statements off |
  | `SELECT * FROM users INTO OUTFILE '/tmp/x'` | no `FILE` privilege |
  | `/*!50000 DROP TABLE users */` | privileges |
  | `CREATE TABLE t (id INT)` | privileges |

  After each statement, the test checks that all four table row counts are unchanged and
  that `users` still exists.
- **Implement:** nothing new is expected. This task pins the guarantee down.
- **Commit:** `Prove crafted writes cannot modify data`

### Task 13 — MCP server and the three tools (fake store, no MySQL)
- **Files:** `internal/tools/server.go`, `internal/tools/server_test.go`
- **API:**
  - The `tools.Store` interface has the three `db` methods.
  - `tools.NewServer(store Store, log *audit.Logger) *mcp.Server` registers `list_tables`,
    `describe_table` (`{"table": string}`, required) and `query` (`{"sql": string}`,
    required).
  - Every tool has `ReadOnlyHint: true`.
  - The tools package owns the output structs. Those structs are the contract Claude sees.
- **Test first:** an in-memory client and a recording fake store.
  - `ListTools` returns exactly the three tools, each marked read-only.
  - `list_tables` and `describe_table` map the fake's data into the structured output.
  - `query` with `SELECT 1` returns `{columns, rows, row_count, truncated}`.
  - `query` with `DELETE FROM t` returns `IsError` with a friendly message listing the
    allowed keywords, **and the store is never called**.
  - `describe_table` with no `table` is rejected by schema validation.
  - A store error becomes `IsError` with the error's text.
- **Commit:** `Expose list_tables, describe_table and query over MCP`

### Task 14 — Audit every tool call
- **Files:** `internal/tools/server.go`, `internal/tools/audit_test.go`
- **Test first:** three calls (one ok, one refused by the allow-list, one store error)
  produce three audit lines. Each line has the right `tool`, `sql` or `table`,
  `row_count`, `duration_ms >= 0`, and `error` set exactly when the call failed.
- **Commit:** `Write an audit line per tool call`

### Task 15 — `cmd/mysql-mcp`: startup wiring and refusal (criterion 4)
- **Files:** `cmd/mysql-mcp/main.go`, `cmd/mysql-mcp/main_test.go`
- **API:**
  - `run(ctx, getenv, stdin, stdout, stderr) error` runs these steps in order:
    1. config
    2. audit writer (file or stderr)
    3. `db.Open`
    4. `Grants` → `CheckGrants`
    5. `tools.NewServer`
    6. `server.Run` over stdio, using the given readers and writers
  - `main` prints the error to stderr and exits 1.
- **Test first:**
  - With no `MYSQL_DSN`, `run` fails with a config error and writes nothing to stdout.
  - (integration) With `MYSQL_TEST_ROOT_DSN`, `run` returns an error starting
    "refusing to start" and quoting a grant line, and stdout stays empty.
- **Commit:** `Wire startup with grant check and stdio server`

### Task 16 — End-to-end (criteria 1–3)
- **Files:** `cmd/mysql-mcp/e2e_test.go`
- **Test first (integration):**
  - **In-memory:** the real `Store` on the seeded database, plus `tools.NewServer` and an
    in-memory client.
    - Call `list_tables`, `describe_table("tasks")` and `query` with the demo question's SQL.
      The query must name the unique top project from Task 8.
    - `query("DELETE FROM tasks")` returns `IsError`.
  - **Stdio smoke test:** `go build` the binary into `t.TempDir()`. Start it through
    `mcp.CommandTransport` with `MYSQL_DSN` set and call `ListTools`. This proves stdout
    carries only protocol traffic.
- **Commit:** `Add end-to-end tests over in-memory and stdio transports`

### Task 17 — CI (criterion 5)
- **Files:** `.github/workflows/test.yml`
- **Workflow:**
  - Runs on `push` and `pull_request` on `ubuntu-latest`.
  - Service `mysql:8.0` with `MYSQL_ROOT_PASSWORD=root` on `3316:3306` and a health check.
  - Steps:
    1. checkout
    2. `actions/setup-go` with `go-version-file: go.mod`
    3. `mysql -h127.0.0.1 -P3316 -uroot -proot < testdata/seed.sql`
    4. `gofmt` check
    5. `go vet ./...`
    6. `go test -race ./...` with both test DSNs
- **Check:** push, then watch the run go green. I'll pin actions to released major
  versions.
- **Commit:** `Run tests against MySQL 8.0 in CI`

### Task 18 — README (criterion 6)
- **Files:** `README.md`
- **Contents:**
  - **What it is:** the three tools and an example output.
  - **Quick start:** `docker compose up -d --wait`,
    `go build -o mysql-mcp ./cmd/mysql-mcp`, then
    `claude mcp add mysql -e MYSQL_DSN=… -- ./mysql-mcp`. I'll check the flag spelling against
    `claude mcp add --help`.
  - **Claude Desktop:** a JSON config snippet.
  - **Configuration:** a table of the environment variables.
  - **Safety model:** the four layers, the two rejected approaches (including the F7 demo),
    and a sample `CREATE USER`/`GRANT` for your own database.
  - **Testing:** the two test modes.
  - **Demo:** a placeholder link that you fill in after recording.
  - **License.**
- **Check:** run every command in the README in this container and confirm the output
  matches.
- **Commit:** `Add README with setup and safety model`

---

## Out of scope for me

- Recording the 60–90 s demo and adding its link.
- Making the repository public.
- Running the `claude mcp add` step in Claude Code or Desktop on your machine. I'll test the
  binary over stdio here; the real-client check is the demo itself.
