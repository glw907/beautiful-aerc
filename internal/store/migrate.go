// Package store owns poplar's single SQLite file: schema, migrations,
// the hot-query SQL every read path issues, and the flags encoding
// (SY-1, C4). One writer goroutine owns every write; reads serve the
// UI through the store's read-only handle. This file is the
// migration runner: a hand-rolled schema_version walk over the SQL
// files embedded from migrations/, per ADR-0001.
package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"slices"

	"github.com/glw907/poplar/internal/uerr"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// localErr surfaces err as op's user-visible failure. Every failure
// this package raises is local to the store file, and none of them
// names an entity worth correlating, so ClassStoreLocal and a nil id
// list are the whole classification. The one exception, a store a
// newer poplar migrated forward, calls uerr.New directly.
func localErr(op string, err error) error {
	return uerr.New(op, nil, uerr.ClassStoreLocal, err)
}

// Migrate brings db up to the schema version this build knows,
// starting from whatever version it finds on disk (0 for a fresh
// file). Each migration runs in its own transaction, so a failure
// partway through leaves a known, prior version on disk rather than a
// half-applied schema. Migrate rejects a store whose on-disk version
// exceeds this build's known maximum: that store was migrated forward
// by a newer poplar binary.
func Migrate(db *sql.DB) error {
	names, err := migrationNames()
	if err != nil {
		return localErr("store.migrate", err)
	}

	if err := ensureSchemaVersionTable(db); err != nil {
		return localErr("store.migrate", err)
	}
	current, err := readSchemaVersion(db)
	if err != nil {
		return localErr("store.migrate", err)
	}

	maxVersion := len(names)
	if current > maxVersion {
		return uerr.New("store.migrate", nil, uerr.ClassSchemaVersion,
			fmt.Errorf("on-disk schema version %d exceeds this build's known version %d", current, maxVersion))
	}

	for current < maxVersion {
		stmt, err := migrationFiles.ReadFile("migrations/" + names[current])
		if err != nil {
			return localErr("store.migrate",
				fmt.Errorf("read migration %s: %w", names[current], err))
		}
		if err := applyMigration(db, string(stmt), current+1); err != nil {
			return localErr("store.migrate", err)
		}
		current++
	}
	return nil
}

// CurrentSchemaVersion returns the schema version recorded on db,
// seeding it to 0 first if Migrate has never run against db. A caller
// compares it against a version read before a Migrate call to tell
// whether that call actually advanced the schema, the "after a
// migration" trigger for an integrity check (SY-8).
func CurrentSchemaVersion(db *sql.DB) (int, error) {
	if err := ensureSchemaVersionTable(db); err != nil {
		return 0, localErr("store.migrate", err)
	}
	version, err := readSchemaVersion(db)
	if err != nil {
		return 0, localErr("store.migrate", err)
	}
	return version, nil
}

// migrationNames returns the embedded migration filenames sorted so
// that index i is the step from schema version i to i+1.
func migrationNames() ([]string, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	slices.Sort(names)
	return names, nil
}

func ensureSchemaVersionTable(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	return nil
}

func readSchemaVersion(db *sql.DB) (int, error) {
	var current int
	err := db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&current)
	switch {
	case err == nil:
		return current, nil
	case errors.Is(err, sql.ErrNoRows):
		if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (0)`); err != nil {
			return 0, fmt.Errorf("seed schema_version: %w", err)
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("read schema_version: %w", err)
	}
}

// applyMigration runs stmt and bumps schema_version to version in one
// transaction.
func applyMigration(db *sql.DB, stmt string, version int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration v%d: %w", version, err)
	}
	if _, err := tx.Exec(stmt); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration v%d: %w", version, err)
	}
	if _, err := tx.Exec(`UPDATE schema_version SET version = ?`, version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("bump schema_version to v%d: %w", version, err)
	}
	return tx.Commit()
}
