package outbox

import (
	"database/sql"
	"time"
)

// row is one outbox row as read back for dispatch: the columns
// dispatchOne and its callers need, decoded from the table's raw
// storage shape.
type row struct {
	id           int64
	kind         Kind
	payload      []byte
	undoGroup    string
	attemptCount int
}

// insertRow inserts one outbox row inside tx and returns its id.
func insertRow(tx *sql.Tx, accountID int64, kind Kind, payload []byte, undoGroup string, chunkSeq int, nextAttemptAt, createdAt time.Time) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO outbox (account_id, kind, payload, undo_group, chunk_seq, next_attempt_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		accountID, string(kind), payload, undoGroup, chunkSeq, nextAttemptAt.Unix(), createdAt.Unix(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// selectEligible returns accountID's queued rows whose hold has
// expired, oldest first: the dispatcher's claim candidates.
func selectEligible(tx *sql.Tx, accountID int64, now time.Time) ([]row, error) {
	rows, err := tx.Query(
		`SELECT id, kind, payload, COALESCE(undo_group, ''), attempt_count FROM outbox
		 WHERE account_id = ? AND state = 'queued' AND next_attempt_at <= ? ORDER BY id`,
		accountID, now.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []row
	for rows.Next() {
		var r row
		var kind string
		if err := rows.Scan(&r.id, &kind, &r.payload, &r.undoGroup, &r.attemptCount); err != nil {
			return nil, err
		}
		r.kind = Kind(kind)
		out = append(out, r)
	}
	return out, rows.Err()
}

// claimRow moves id from queued to dispatching inside tx, the state
// transition ADR-0006 revision 2 requires before any I/O. The WHERE
// guard is defensive: the store's single writer connection already
// serializes tx against every other writer transaction, so nothing
// can have moved id off queued between selectEligible's read and this
// statement in the same transaction.
func claimRow(tx *sql.Tx, id int64) error {
	_, err := tx.Exec(`UPDATE outbox SET state = 'dispatching' WHERE id = ? AND state = 'queued'`, id)
	return err
}

// deleteRow removes id: the ack a dispatched intent's success, or an
// unretriable failure's report-and-give-up, leaves behind.
func deleteRow(tx *sql.Tx, id int64) error {
	_, err := tx.Exec(`DELETE FROM outbox WHERE id = ?`, id)
	return err
}

// revertRow returns a claimed id to queued unchanged: this pass
// claimed it but never attempted it, because an earlier claimed
// intent's connection failure stopped the pass before its turn.
func revertRow(tx *sql.Tx, id int64) error {
	_, err := tx.Exec(`UPDATE outbox SET state = 'queued' WHERE id = ?`, id)
	return err
}

// requeueRow returns id to queued after a retriable failure,
// recording the failure for a caller to surface and bumping the
// backoff and attempt count.
func requeueRow(tx *sql.Tx, id int64, nextAttemptAt time.Time, class, detail string) error {
	_, err := tx.Exec(
		`UPDATE outbox SET state = 'queued', attempt_count = attempt_count + 1, next_attempt_at = ?, failure_class = ?, failure_detail = ? WHERE id = ?`,
		nextAttemptAt.Unix(), class, detail, id,
	)
	return err
}

// updatePayload overwrites id's payload: KindCreateMailbox persists
// its resolved server id here immediately after a successful backend
// call, before this pass's finalize step, so a crash between the two
// still leaves a replay able to see the resolution already recorded.
func updatePayload(tx *sql.Tx, id int64, payload []byte) error {
	_, err := tx.Exec(`UPDATE outbox SET payload = ? WHERE id = ?`, payload, id)
	return err
}

// annihilate deletes id only if it is still queued, reporting whether
// it did: the same guarded statement claimRow uses, from the other
// side of the race. Legal only against queued rows, decided in this
// one statement, is what makes the undo-versus-in-flight-dispatch
// race impossible rather than merely unlikely.
func annihilate(tx *sql.Tx, id int64) (bool, error) {
	res, err := tx.Exec(`DELETE FROM outbox WHERE id = ? AND state = 'queued'`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
