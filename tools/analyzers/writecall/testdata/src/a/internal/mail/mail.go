// Package mail stands in for a poplar package outside ADR-0003's
// writer cast: the guard is default-deny, so it holds here the same
// way it holds for internal/ui.
package mail

import (
	"database/sql"

	"a/internal/store"
)

func Save(tx *sql.Tx, accountID int64, m store.MessageUpsert) error {
	return store.UpsertMessage(tx, accountID, m) // want `internal/mail reaches store\.UpsertMessage, part of internal/store's write surface`
}
