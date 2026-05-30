package jobs

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

// errFakeFetch is the canned transport error a fake Fetcher returns for the
// fetch-failure test.
var errFakeFetch = errors.New("fake fetch failure")

// timeNowUTC is a tiny helper so tests can stamp job_run rows.
func timeNowUTC() time.Time { return time.Now().UTC() }

// seedOwnerChar inserts one owner + one character (the jobs package can't import
// the store package's test-only helper, so this mirrors it locally) and returns
// the character id, used by the wiki items-pass tests to give the inventory union
// some refs to fetch.
func seedOwnerChar(t *testing.T, db *sql.DB, ownerLabel, charName string) (ownerID, charID int64) {
	t.Helper()
	res, err := db.Exec(`INSERT INTO owner (label) VALUES (?)`, ownerLabel)
	if err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	ownerID, _ = res.LastInsertId()
	res, err = db.Exec(`INSERT INTO character (owner_id, name) VALUES (?, ?)`, ownerID, charName)
	if err != nil {
		t.Fatalf("seed character: %v", err)
	}
	charID, _ = res.LastInsertId()
	return ownerID, charID
}

// assertJobStatus checks the job_run row for jobName has the wanted last_status.
func assertJobStatus(t *testing.T, db *sql.DB, jobName, wantStatus string) {
	t.Helper()
	var status string
	err := db.QueryRow(`SELECT last_status FROM job_run WHERE job_name = ?`, jobName).Scan(&status)
	if err != nil {
		t.Fatalf("read job_run status for %q: %v", jobName, err)
	}
	if status != wantStatus {
		t.Errorf("job_run[%q].last_status = %q, want %q", jobName, status, wantStatus)
	}
}
