-- Sample database for mysql-mcp-go: a fictional project tracker.
--
-- Every row is computed from its row number with recursive CTEs, so a fresh
-- database always gets the same data. Due dates are relative to the day the
-- seed runs, so some tasks are always overdue.

CREATE DATABASE IF NOT EXISTS demo;
USE demo;

CREATE TABLE users (
  id         INT          NOT NULL PRIMARY KEY,
  name       VARCHAR(100) NOT NULL,
  email      VARCHAR(255) NOT NULL UNIQUE,
  created_at DATETIME     NOT NULL
) COMMENT = 'People who own projects, are assigned tasks and write comments';

CREATE TABLE projects (
  id       INT          NOT NULL PRIMARY KEY,
  name     VARCHAR(100) NOT NULL,
  owner_id INT          NOT NULL,
  FOREIGN KEY (owner_id) REFERENCES users (id)
) COMMENT = 'Projects, each with one owner';

CREATE TABLE tasks (
  id          INT          NOT NULL PRIMARY KEY,
  project_id  INT          NOT NULL,
  assignee_id INT          NULL,
  title       VARCHAR(200) NOT NULL,
  status      ENUM('todo', 'in_progress', 'done') NOT NULL DEFAULT 'todo',
  due_date    DATE         NULL,
  created_at  DATETIME     NOT NULL,
  KEY idx_tasks_status_due (status, due_date),
  FOREIGN KEY (project_id) REFERENCES projects (id),
  FOREIGN KEY (assignee_id) REFERENCES users (id)
) COMMENT = 'Work items; a task is overdue when due_date has passed and status is not done';

CREATE TABLE comments (
  id         INT      NOT NULL PRIMARY KEY,
  task_id    INT      NOT NULL,
  author_id  INT      NOT NULL,
  body       TEXT     NOT NULL,
  created_at DATETIME NOT NULL,
  FOREIGN KEY (task_id) REFERENCES tasks (id),
  FOREIGN KEY (author_id) REFERENCES users (id)
) COMMENT = 'Discussion on tasks';

-- 50 users: 10 first names x 5 last names.
INSERT INTO users (id, name, email, created_at)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 50),
names AS (
  SELECT n,
         ELT(1 + (n - 1) MOD 10, 'Ava', 'Ben', 'Chloe', 'Dev', 'Elena',
                                 'Femi', 'Grace', 'Hiro', 'Ines', 'Jonas') AS first_name,
         ELT(1 + (n - 1) DIV 10, 'Okafor', 'Lindqvist', 'Moreau', 'Tanaka', 'Silva') AS last_name
  FROM seq
)
SELECT n,
       CONCAT(first_name, ' ', last_name),
       LOWER(CONCAT(first_name, '.', last_name, '@example.com')),
       TIMESTAMP('2025-01-01') + INTERVAL n DAY
FROM names;

INSERT INTO projects (id, name, owner_id) VALUES
  (1, 'Website redesign',  3),
  (2, 'Mobile app',        8),
  (3, 'Billing migration', 12),
  (4, 'Data warehouse',    17),
  (5, 'Customer portal',   21),
  (6, 'Security audit',    26),
  (7, 'Onboarding flow',   33),
  (8, 'Internal tools',    41);

-- 300 tasks spread evenly over the 8 projects. Due dates run from 45 days ago
-- to 44 days ahead; every 25th task has none and every 15th is unassigned.
INSERT INTO tasks (id, project_id, assignee_id, title, status, due_date, created_at)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 300)
SELECT n,
       1 + (n * 7) MOD 8,
       IF(n MOD 15 = 0, NULL, 1 + (n * 13) MOD 50),
       CONCAT(ELT(1 + n MOD 8, 'Fix', 'Write', 'Review', 'Update',
                               'Design', 'Test', 'Migrate', 'Document'),
              ' ',
              ELT(1 + (n DIV 8) MOD 7, 'login page', 'invoice export', 'API docs',
                                       'search index', 'payment webhook',
                                       'user settings', 'release notes')),
       CASE WHEN n MOD 10 < 4 THEN 'done'
            WHEN n MOD 10 < 7 THEN 'in_progress'
            ELSE 'todo' END,
       IF(n MOD 25 = 0, NULL, CURDATE() + INTERVAL ((n * 11) MOD 90) - 45 DAY),
       CURDATE() - INTERVAL (60 + n MOD 30) DAY + INTERVAL (9 + n MOD 8) HOUR
FROM seq;

-- Billing migration is behind schedule: its past-due tasks were never
-- finished. This gives the demo question one clear answer.
UPDATE tasks
SET status = 'in_progress'
WHERE project_id = 3 AND status = 'done' AND due_date < CURDATE();

-- 600 comments spread over the tasks.
INSERT INTO comments (id, task_id, author_id, body, created_at)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 600)
SELECT n,
       1 + (n * 17) MOD 300,
       1 + (n * 7) MOD 50,
       ELT(1 + n MOD 6, 'Looks good to me.', 'Blocked on review.',
                        'Pushed a fix, please re-check.', 'Can we move the due date?',
                        'Added test coverage.', 'Waiting on the design team.'),
       CURDATE() - INTERVAL (n MOD 60) DAY + INTERVAL (n MOD 24) HOUR
FROM seq;

-- The account the MCP server uses: read-only on this schema.
CREATE USER IF NOT EXISTS 'mcp_ro'@'%' IDENTIFIED BY 'mcp_ro';
GRANT SELECT, SHOW VIEW ON demo.* TO 'mcp_ro'@'%';

-- Refresh the row estimates that list_tables reports.
ANALYZE TABLE users, projects, tasks, comments;
