package store

import (
	"database/sql"
	"maps"
	"slices"
	"strings"
	"testing"
)

// exemptFromAccountScoping names the tables ADR-0002's account-
// scoping rule does not apply to: account is the scope root itself,
// and schema_version is process state, not account state.
var exemptFromAccountScoping = map[string]bool{
	"account":        true,
	"schema_version": true,
}

// TestAccountScopingRule walks the live schema and asserts ADR-0002's
// rule: every table not reachable by foreign key from a scoped
// parent carries account_id. sent_history has no parent to be
// reachable from, which is the case the rule exists to catch.
func TestAccountScopingRule(t *testing.T) {
	db := openMigratedTestDB(t)

	tables := storeTables(t, db)
	scoped := make(map[string]bool, len(tables))

	for _, table := range tables {
		if exemptFromAccountScoping[table] {
			continue
		}
		if !isAccountScoped(t, db, table, scoped, map[string]bool{}) {
			t.Errorf("table %s carries no account_id and is not reachable by foreign key from a table that does", table)
		}
	}
}

// storeTables returns every real, non-FTS5-shadow table in db.
func storeTables(t *testing.T, db *sql.DB) []string {
	t.Helper()

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		// message_fts and its shadow tables (_data, _idx, _docsize,
		// _config) are FTS5-managed derived state, not model tables.
		if strings.HasPrefix(name, "message_fts") {
			continue
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master: %v", err)
	}
	return names
}

// isAccountScoped reports whether table carries account_id directly
// or has a foreign key, direct or transitive, to a table that does.
// scoped memoizes results across calls; visiting guards against a
// foreign-key cycle and is scoped to the current call chain, not
// shared across sibling branches.
func isAccountScoped(t *testing.T, db *sql.DB, table string, scoped, visiting map[string]bool) bool {
	t.Helper()

	if result, ok := scoped[table]; ok {
		return result
	}
	if visiting[table] {
		return false
	}

	result := slices.Contains(tableColumns(t, db, table), "account_id")
	if !result {
		branch := maps.Clone(visiting)
		branch[table] = true
		for _, target := range foreignKeyTargets(t, db, table) {
			if isAccountScoped(t, db, target, scoped, branch) {
				result = true
				break
			}
		}
	}

	scoped[table] = result
	return result
}

func tableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()

	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	defer func() { _ = rows.Close() }()

	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan pragma_table_info(%s): %v", table, err)
		}
		cols = append(cols, name)
	}
	return cols
}

func foreignKeyTargets(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()

	rows, err := db.Query(`SELECT "table" FROM pragma_foreign_key_list(?)`, table)
	if err != nil {
		t.Fatalf("pragma_foreign_key_list(%s): %v", table, err)
	}
	defer func() { _ = rows.Close() }()

	var targets []string
	for rows.Next() {
		var target string
		if err := rows.Scan(&target); err != nil {
			t.Fatalf("scan pragma_foreign_key_list(%s): %v", table, err)
		}
		targets = append(targets, target)
	}
	return targets
}
