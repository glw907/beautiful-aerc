package cache

import (
	"database/sql"
	"slices"
	"testing"
)

// readPragmaTableInfo returns column names for the given table.
func readPragmaTableInfo(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan pragma_table_info: %v", err)
		}
		cols = append(cols, name)
	}
	return cols
}

// readPragmaIndexInfo returns the ordered column names for the given index.
func readPragmaIndexInfo(t *testing.T, db *sql.DB, index string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_index_info(?) ORDER BY seqno`, index)
	if err != nil {
		t.Fatalf("pragma_index_info(%s): %v", index, err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan pragma_index_info: %v", err)
		}
		cols = append(cols, name)
	}
	return cols
}

func TestMigrateV10AddsScheduledForAndDraftID(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	cols := readPragmaTableInfo(t, a.db, "outbox")
	if !slices.Contains(cols, "scheduled_for") {
		t.Errorf("outbox missing scheduled_for column; got %v", cols)
	}
	if !slices.Contains(cols, "draft_id") {
		t.Errorf("outbox missing draft_id column; got %v", cols)
	}

	idx := readPragmaIndexInfo(t, a.db, "outbox_pickup")
	if got, want := idx, []string{"scheduled_for", "next_eligible_at", "id"}; !slices.Equal(got, want) {
		t.Errorf("outbox_pickup index columns = %v, want %v", got, want)
	}
}
