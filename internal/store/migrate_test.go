package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
)

// TestMigrateFresh applies schema version 1 to an empty file and
// checks the resulting schema against a committed golden dump, so an
// unreviewed schema change fails here instead of surfacing as a
// mismatch somewhere downstream.
func TestMigrateFresh(t *testing.T) {
	db := openTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var version int
	if err := db.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version != 1 {
		t.Errorf("schema_version = %d, want 1", version)
	}

	got := dumpSchema(t, db)
	want := readGolden(t, "schema_v1.golden")
	if got != want {
		t.Errorf("schema dump does not match golden:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestMigrateRejectsNewerSchema proves Migrate refuses to run
// against a store a newer poplar binary already migrated forward,
// rather than silently applying migrations this build doesn't have.
func TestMigrateRejectsNewerSchema(t *testing.T) {
	db := openTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(`UPDATE schema_version SET version = version + 1`); err != nil {
		t.Fatalf("bump schema_version past known: %v", err)
	}

	err := Migrate(db)
	if err == nil {
		t.Fatal("Migrate returned nil error against a newer schema version")
	}

	uerrErr := uerrtest.AssertClass(t, err, uerr.ClassSchemaVersion)

	cause := uerrErr.Cause.Error()
	if !strings.Contains(cause, "2") || !strings.Contains(cause, "1") {
		t.Errorf("cause %q does not name both the on-disk version (2) and the known version (1)", cause)
	}
}

// TestMigrateFailureReachesUerrSeam proves a failed migration is not
// a bare error string: it reaches the caller as a uerr.Error under
// ClassStoreLocal, naming the underlying database failure as its
// Cause, so it logs through uerr's one seam (ER-1) instead of
// surfacing as an opaque, unclassified string.
func TestMigrateFailureReachesUerrSeam(t *testing.T) {
	db := openTestDB(t)

	// Pre-create a table the first migration also creates, without
	// IF NOT EXISTS, so applyMigration's CREATE TABLE fails partway
	// through: a realistic "failed migration" rather than a
	// synthetic error injected below the seam.
	if _, err := db.Exec(`CREATE TABLE account (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("pre-create account table: %v", err)
	}

	err := Migrate(db)
	if err == nil {
		t.Fatal("Migrate against a colliding table returned nil error")
	}

	uerrErr := uerrtest.AssertClass(t, err, uerr.ClassStoreLocal)
	if uerrErr.Cause == nil {
		t.Error("Cause is nil, want the underlying CREATE TABLE failure")
	}
}

// TestCurrentSchemaVersion proves it tracks Migrate's own bookkeeping:
// 0 on an unmigrated file, 1 once Migrate has run.
func TestCurrentSchemaVersion(t *testing.T) {
	db := openTestDB(t)

	before, err := CurrentSchemaVersion(db)
	if err != nil {
		t.Fatalf("CurrentSchemaVersion before Migrate: %v", err)
	}
	if before != 0 {
		t.Errorf("version before Migrate = %d, want 0", before)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	after, err := CurrentSchemaVersion(db)
	if err != nil {
		t.Fatalf("CurrentSchemaVersion after Migrate: %v", err)
	}
	if after != 1 {
		t.Errorf("version after Migrate = %d, want 1", after)
	}
}

// dumpSchema returns a deterministic text form of db's schema: every
// sqlite_master row with a CREATE statement, sorted by type and name.
func dumpSchema(t *testing.T, db *sql.DB) string {
	t.Helper()

	rows, err := db.Query(`SELECT type, name, sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY type, name`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var b strings.Builder
	for rows.Next() {
		var typ, name, stmt string
		if err := rows.Scan(&typ, &name, &stmt); err != nil {
			t.Fatalf("scan sqlite_master row: %v", err)
		}
		fmt.Fprintf(&b, "-- %s %s\n%s;\n\n", typ, name, stmt)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master: %v", err)
	}
	return b.String()
}

// readGolden returns the contents of testdata/name. The caller
// regenerates a stale golden by hand; there is no --update flag
// because a schema golden change belongs under review, not a script.
func readGolden(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	return string(data)
}
