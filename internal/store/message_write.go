package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// MessageUpsert is one message's writable scalar fields. A caller
// outside this package (internal/sync's translation layer) builds
// one from whatever vocabulary its own source speaks; UpsertMessage
// is the only place that turns it into message's schema, so a
// column or a JSON shape never has to be known anywhere else.
type MessageUpsert struct {
	ServerID      string
	BlobID        string
	ThreadKey     string
	Subject       string
	FromAddr      string
	Flags         Flags
	Size          int64
	HasAttachment bool
	ReceivedAt    time.Time
	MailboxIDs    []string
	Unread        bool
}

// messageData is message.data's shape: the one key UpsertMessage
// writes today (mailbox_ids, so a later mailbox sync can repair an
// association SyncMessageMailboxes had to skip). internal/mail
// extends this with its own keys from pass 3 onward.
type messageData struct {
	MailboxIDs []string `json:"mailbox_ids"`
}

// UpsertMessage writes m as accountID's message inside tx, keyed by
// m.ServerID: an update if a row with that server id already exists,
// an insert otherwise. It also reconciles message_mailbox to
// m.MailboxIDs and leaves search_text at its default: a message's
// body, and the full-text terms derived from it, are internal/mail's
// concern from pass 3 onward. message_fts stays in step with
// whatever subject this writes through trg_message_fts_insert and
// trg_message_fts_update (this package's own schema triggers), so
// UpsertMessage never has to maintain the index itself.
func UpsertMessage(tx *sql.Tx, accountID int64, m MessageUpsert) error {
	data, err := json.Marshal(messageData{MailboxIDs: m.MailboxIDs})
	if err != nil {
		return err
	}

	var id int64
	err = tx.QueryRow(`SELECT id FROM message WHERE account_id = ? AND server_id = ?`, accountID, m.ServerID).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.Exec(
			`INSERT INTO message (account_id, server_id, blob_id, thread_key, received_at, subject, from_addr, flags, size, has_attachment, data)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			accountID, m.ServerID, m.BlobID, m.ThreadKey, m.ReceivedAt.Unix(), m.Subject, m.FromAddr, int64(m.Flags), m.Size, m.HasAttachment, data,
		)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if _, err := tx.Exec(
			`UPDATE message SET blob_id = ?, thread_key = ?, received_at = ?, subject = ?, from_addr = ?, flags = ?, size = ?, has_attachment = ?, data = ? WHERE id = ?`,
			m.BlobID, m.ThreadKey, m.ReceivedAt.Unix(), m.Subject, m.FromAddr, int64(m.Flags), m.Size, m.HasAttachment, data, id,
		); err != nil {
			return err
		}
	}
	return SyncMessageMailboxes(tx, accountID, id, m.ReceivedAt.Unix(), m.Unread, m.MailboxIDs)
}

// DeleteMessage removes accountID's message with server id serverID,
// if any.
func DeleteMessage(tx *sql.Tx, accountID int64, serverID string) error {
	_, err := tx.Exec(`DELETE FROM message WHERE account_id = ? AND server_id = ?`, accountID, serverID)
	return err
}

// DeleteMessageByID removes the message row with the given internal
// id.
func DeleteMessageByID(tx *sql.Tx, id int64) error {
	_, err := tx.Exec(`DELETE FROM message WHERE id = ?`, id)
	return err
}

// StaleMessageIDs returns the internal ids of accountID's origin =
// 'server' message rows whose server id is absent from keep: an
// origin = 'local' draft is never a deletion candidate, so a resync
// never considers it.
func StaleMessageIDs(tx *sql.Tx, accountID int64, keep map[string]bool) ([]int64, error) {
	rows, err := tx.Query(`SELECT id, server_id FROM message WHERE account_id = ? AND origin = 'server'`, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var stale []int64
	for rows.Next() {
		var id int64
		var serverID sql.NullString
		if err := rows.Scan(&id, &serverID); err != nil {
			return nil, err
		}
		if !serverID.Valid || !keep[serverID.String] {
			stale = append(stale, id)
		}
	}
	return stale, rows.Err()
}

// SyncMessageMailboxes reconciles message_mailbox to exactly the
// mailboxes named by mailboxServerIDs: dropping associations no
// longer present, refreshing the denormalized received_at and unread
// columns on the ones that remain, and inserting new ones. A server
// id with no matching local mailbox row yet is skipped, not failed;
// RepairMailboxAssociations revisits it once that mailbox's row
// exists.
func SyncMessageMailboxes(tx *sql.Tx, accountID, messageID, receivedAt int64, unread bool, mailboxServerIDs []string) error {
	want := make(map[int64]bool, len(mailboxServerIDs))
	for _, sid := range mailboxServerIDs {
		var mbID int64
		err := tx.QueryRow(`SELECT id FROM mailbox WHERE account_id = ? AND server_id = ?`, accountID, sid).Scan(&mbID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		want[mbID] = true
	}

	current, err := currentMailboxIDs(tx, messageID)
	if err != nil {
		return err
	}

	for _, mbID := range current {
		if want[mbID] {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM message_mailbox WHERE message_id = ? AND mailbox_id = ?`, messageID, mbID); err != nil {
			return err
		}
	}
	for mbID := range want {
		_, err := tx.Exec(
			`INSERT INTO message_mailbox (message_id, mailbox_id, received_at, unread) VALUES (?, ?, ?, ?)
			 ON CONFLICT(message_id, mailbox_id) DO UPDATE SET received_at = excluded.received_at, unread = excluded.unread`,
			messageID, mbID, receivedAt, unread,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func currentMailboxIDs(tx *sql.Tx, messageID int64) ([]int64, error) {
	rows, err := tx.Query(`SELECT mailbox_id FROM message_mailbox WHERE message_id = ?`, messageID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []int64
	for rows.Next() {
		var mbID int64
		if err := rows.Scan(&mbID); err != nil {
			return nil, err
		}
		ids = append(ids, mbID)
	}
	return ids, rows.Err()
}
