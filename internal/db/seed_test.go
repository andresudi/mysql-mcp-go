package db

import "testing"

func TestSeedRowCounts(t *testing.T) {
	db := rawDB(t, testDSN(t))
	want := map[string]int{"users": 50, "projects": 8, "tasks": 300, "comments": 600}
	for table, n := range want {
		if got := queryInt(t, db, "SELECT COUNT(*) FROM "+table); got != n {
			t.Errorf("%s has %d rows, want %d", table, got, n)
		}
	}
}

func TestSeedHasOverdueTasks(t *testing.T) {
	db := rawDB(t, testDSN(t))
	got := queryInt(t, db, "SELECT COUNT(*) FROM tasks WHERE due_date < CURDATE() AND status <> 'done'")
	if got < 20 {
		t.Errorf("%d overdue tasks, want at least 20", got)
	}
}

// The demo asks which project has the most overdue tasks, so the answer must
// be clear: one project, ahead of the runner-up by at least 3.
func TestSeedHasClearTopOverdueProject(t *testing.T) {
	db := rawDB(t, testDSN(t))
	rows, err := db.Query(`
		SELECT project_id, COUNT(*) AS n FROM tasks
		WHERE due_date < CURDATE() AND status <> 'done'
		GROUP BY project_id ORDER BY n DESC LIMIT 2`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var counts []int
	for rows.Next() {
		var project, n int
		if err := rows.Scan(&project, &n); err != nil {
			t.Fatal(err)
		}
		counts = append(counts, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(counts) != 2 || counts[0]-counts[1] < 3 {
		t.Errorf("top two overdue counts = %v, want a lead of at least 3", counts)
	}
}
