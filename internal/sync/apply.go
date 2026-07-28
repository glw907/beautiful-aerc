package sync

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
)

// flagKeyword maps sync's Record.Fields boolean vocabulary (task 9's
// keys: "seen", "flagged", ...) to the $-prefixed keyword
// store.EncodeFlags expects, the same shape every backend already
// translates its wire keywords into before Changes returns.
var flagKeyword = map[string]string{
	"seen":      "$seen",
	"flagged":   "$flagged",
	"answered":  "$answered",
	"draft":     "$draft",
	"forwarded": "$forwarded",
}

func flagsFromFields(fields map[string]any) store.Flags {
	var keywords []string
	for name, kw := range flagKeyword {
		if v, _ := fields[name].(bool); v {
			keywords = append(keywords, kw)
		}
	}
	bits, _ := store.EncodeFlags(keywords)
	return bits
}

func stringField(f map[string]any, key string) string {
	s, _ := f[key].(string)
	return s
}

func int64Field(f map[string]any, key string) int64 {
	n, _ := f[key].(int64)
	return n
}

func boolField(f map[string]any, key string) bool {
	b, _ := f[key].(bool)
	return b
}

// firstAddress renders the first entry of an address-list field
// (Record.Fields' "from") as message.from_addr's single display
// string: "Name <email>" when a name is present, the bare address
// otherwise. internal/mail owns full envelope modeling from pass 3;
// this is the scalar column pass 1's list view reads.
func firstAddress(v any) string {
	addrs, ok := v.([]map[string]string)
	if !ok || len(addrs) == 0 {
		return ""
	}
	if name := addrs[0]["name"]; name != "" {
		return name + " <" + addrs[0]["email"] + ">"
	}
	return addrs[0]["email"]
}

// applyChangeSet writes cs's created, updated, and destroyed records
// for kind into account accountID, inside tx.
func applyChangeSet(tx *sql.Tx, accountID int64, kind backend.ObjectKind, cs backend.ChangeSet) error {
	for _, rec := range cs.Created {
		if err := upsertRecord(tx, accountID, kind, rec); err != nil {
			return err
		}
	}
	for _, rec := range cs.Updated {
		if err := upsertRecord(tx, accountID, kind, rec); err != nil {
			return err
		}
	}
	for _, id := range cs.Destroyed {
		if err := destroyRecord(tx, accountID, kind, id); err != nil {
			return err
		}
	}
	return nil
}

func upsertRecord(tx *sql.Tx, accountID int64, kind backend.ObjectKind, rec backend.Record) error {
	switch kind {
	case backend.ObjectKindMessage:
		return upsertMessage(tx, accountID, rec)
	case backend.ObjectKindMailbox:
		return upsertMailbox(tx, accountID, rec)
	default:
		return fmt.Errorf("sync: apply: unsupported kind %v", kind)
	}
}

func destroyRecord(tx *sql.Tx, accountID int64, kind backend.ObjectKind, serverID string) error {
	switch kind {
	case backend.ObjectKindMessage:
		_, err := tx.Exec(`DELETE FROM message WHERE account_id = ? AND server_id = ?`, accountID, serverID)
		return err
	case backend.ObjectKindMailbox:
		_, err := tx.Exec(`DELETE FROM mailbox WHERE account_id = ? AND server_id = ?`, accountID, serverID)
		return err
	default:
		return fmt.Errorf("sync: destroy: unsupported kind %v", kind)
	}
}

// upsertMailbox writes rec as accountID's mailbox, keyed by
// rec.ID (the server id): an update if a row with that server id
// already exists, an insert otherwise.
func upsertMailbox(tx *sql.Tx, accountID int64, rec backend.Record) error {
	f := rec.Fields
	name := stringField(f, "name")
	role := stringField(f, "role")
	sortOrder := int64Field(f, "sort_order")
	totalCount := int64Field(f, "total_emails")
	unreadCount := int64Field(f, "unread_emails")

	var id int64
	err := tx.QueryRow(`SELECT id FROM mailbox WHERE account_id = ? AND server_id = ?`, accountID, rec.ID).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.Exec(
			`INSERT INTO mailbox (account_id, server_id, role, name, sort_order, total_count, unread_count) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			accountID, rec.ID, role, name, sortOrder, totalCount, unreadCount,
		)
		return err
	case err != nil:
		return err
	default:
		_, err = tx.Exec(
			`UPDATE mailbox SET role = ?, name = ?, sort_order = ?, total_count = ?, unread_count = ? WHERE id = ?`,
			role, name, sortOrder, totalCount, unreadCount, id,
		)
		return err
	}
}

// upsertMessage writes rec as accountID's message, keyed by rec.ID,
// and reconciles its mailbox associations to fields["mailbox_ids"].
// It leaves search_text at its default: a message's body, and the
// full-text terms derived from it, are internal/mail's concern from
// pass 3 onward. message_fts stays in step with whatever subject this
// writes through trg_message_fts_insert and trg_message_fts_update
// (internal/store's schema triggers), so this function never has to
// maintain the index itself.
func upsertMessage(tx *sql.Tx, accountID int64, rec backend.Record) error {
	f := rec.Fields
	blobID := stringField(f, "blob_id")
	threadKey := stringField(f, "thread_id")
	subject := stringField(f, "subject")
	size := int64Field(f, "size")
	hasAttachment := boolField(f, "has_attachment")
	receivedAt, _ := f["received_at"].(time.Time)
	fromAddr := firstAddress(f["from"])
	flags := flagsFromFields(f)
	mailboxIDs, _ := f["mailbox_ids"].([]string)
	unread := !boolField(f, "seen")

	var id int64
	err := tx.QueryRow(`SELECT id FROM message WHERE account_id = ? AND server_id = ?`, accountID, rec.ID).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.Exec(
			`INSERT INTO message (account_id, server_id, blob_id, thread_key, received_at, subject, from_addr, flags, size, has_attachment)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			accountID, rec.ID, blobID, threadKey, receivedAt.Unix(), subject, fromAddr, int64(flags), size, hasAttachment,
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
			`UPDATE message SET blob_id = ?, thread_key = ?, received_at = ?, subject = ?, from_addr = ?, flags = ?, size = ?, has_attachment = ? WHERE id = ?`,
			blobID, threadKey, receivedAt.Unix(), subject, fromAddr, int64(flags), size, hasAttachment, id,
		); err != nil {
			return err
		}
	}
	return syncMessageMailboxes(tx, accountID, id, receivedAt.Unix(), unread, mailboxIDs)
}

// syncMessageMailboxes reconciles message_mailbox to exactly the
// mailboxes named by mailboxServerIDs: dropping associations no
// longer present, refreshing the denormalized received_at and unread
// columns on the ones that remain, and inserting new ones. A server
// id with no matching local mailbox row yet is skipped rather than
// failed, so message and mailbox kinds never have to sync in a fixed
// order: a later mailbox sync pass fills the association in.
func syncMessageMailboxes(tx *sql.Tx, accountID, messageID, receivedAt int64, unread bool, mailboxServerIDs []string) error {
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
