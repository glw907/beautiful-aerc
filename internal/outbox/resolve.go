package outbox

import (
	"database/sql"
	"errors"
)

// mailboxServerID returns id's server id, or "" if id names no
// mailbox row or one still awaiting its own server assignment.
func mailboxServerID(tx *sql.Tx, id int64) (string, error) {
	var serverID sql.NullString
	err := tx.QueryRow(`SELECT server_id FROM mailbox WHERE id = ?`, id).Scan(&serverID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", nil
	case err != nil:
		return "", err
	}
	return serverID.String, nil
}

// messageServerID returns id's server id, or "" if id names no
// message row or an origin = 'local' draft with none yet.
func messageServerID(tx *sql.Tx, id int64) (string, error) {
	var serverID sql.NullString
	err := tx.QueryRow(`SELECT server_id FROM message WHERE id = ?`, id).Scan(&serverID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", nil
	case err != nil:
		return "", err
	}
	return serverID.String, nil
}

// currentMailboxID returns messageID's mailbox, pass 1's
// single-mailbox move model: the first message_mailbox row found, or
// 0 if the message currently belongs to none.
func currentMailboxID(tx *sql.Tx, messageID int64) (int64, error) {
	var mailboxID int64
	err := tx.QueryRow(`SELECT mailbox_id FROM message_mailbox WHERE message_id = ? LIMIT 1`, messageID).Scan(&mailboxID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, nil
	case err != nil:
		return 0, err
	}
	return mailboxID, nil
}
