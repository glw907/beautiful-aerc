package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// MailboxUpsert is one mailbox's writable scalar fields, the same
// shape MessageUpsert is for message: a caller outside this package
// translates its own vocabulary into it, and UpsertMailbox is the
// only place that turns it into mailbox's schema.
type MailboxUpsert struct {
	ServerID    string
	Role        string
	Name        string
	SortOrder   int64
	TotalCount  int64
	UnreadCount int64
}

// mailboxData is mailbox.data's shape: the role the backend declared,
// kept verbatim because duplicate resolution has to tell a declared
// role from one the name heuristic guessed.
type mailboxData struct {
	ServerRole string `json:"server_role,omitempty"`
}

// UpsertMailbox writes m as accountID's mailbox, keyed by m.ServerID:
// an update if a row with that server id already exists, an insert
// otherwise. mailbox.role holds the classifier's answer rather than
// m.Role (FO-1), and resolveAccountMailboxRoles reclassifies the
// account on every write that can change a role. Inserting a new row
// also repairs any message whose mailbox_ids named this mailbox before
// its local row existed (see RepairMailboxAssociations).
func UpsertMailbox(tx *sql.Tx, accountID int64, m MailboxUpsert) error {
	data, err := json.Marshal(mailboxData{ServerRole: m.Role})
	if err != nil {
		return err
	}

	var id int64
	var priorName, priorData string
	err = tx.QueryRow(`SELECT id, name, data FROM mailbox WHERE account_id = ? AND server_id = ?`, accountID, m.ServerID).Scan(&id, &priorName, &priorData)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.Exec(
			`INSERT INTO mailbox (account_id, server_id, name, sort_order, total_count, unread_count, data) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			accountID, m.ServerID, m.Name, m.SortOrder, m.TotalCount, m.UnreadCount, string(data),
		)
		if err != nil {
			return err
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if err := RepairMailboxAssociations(tx, accountID, newID, m.ServerID); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		_, err = tx.Exec(
			`UPDATE mailbox SET name = ?, sort_order = ?, total_count = ?, unread_count = ?, data = ? WHERE id = ?`,
			m.Name, m.SortOrder, m.TotalCount, m.UnreadCount, string(data), id,
		)
		if err != nil {
			return err
		}
		// A role follows from the account's names and declared roles
		// alone, so a count refresh (the common update by far) leaves
		// every role where it stands.
		if m.Name == priorName && string(data) == priorData {
			return nil
		}
	}
	return resolveAccountMailboxRoles(tx, accountID)
}

// resolveAccountMailboxRoles classifies every mailbox in accountID and
// writes each resolved role whose row disagrees with it. The pass
// covers the whole account because a sync page delivers mailboxes one
// at a time, and resolveMailboxRoles settles a contested role only
// while it can see every claimant.
func resolveAccountMailboxRoles(tx *sql.Tx, accountID int64) error {
	rows, err := tx.Query(`SELECT id, name, role, data FROM mailbox WHERE account_id = ?`, accountID)
	if err != nil {
		return err
	}

	var candidates []mailboxRoleCandidate
	stored := map[int64]string{}
	for rows.Next() {
		c := mailboxRoleCandidate{AccountID: accountID}
		var role, data string
		if err := rows.Scan(&c.ID, &c.Name, &role, &data); err != nil {
			_ = rows.Close()
			return err
		}
		var md mailboxData
		if err := json.Unmarshal([]byte(data), &md); err != nil {
			_ = rows.Close()
			return err
		}
		c.ServerRole = md.ServerRole
		candidates = append(candidates, c)
		stored[c.ID] = role
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	resolved := resolveMailboxRoles(candidates)
	for _, c := range candidates {
		role := string(resolved[c.ID])
		if role == stored[c.ID] {
			continue
		}
		if _, err := tx.Exec(`UPDATE mailbox SET role = ? WHERE id = ?`, role, c.ID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteMailbox removes accountID's mailbox with server id serverID,
// if any.
func DeleteMailbox(tx *sql.Tx, accountID int64, serverID string) error {
	_, err := tx.Exec(`DELETE FROM mailbox WHERE account_id = ? AND server_id = ?`, accountID, serverID)
	return err
}

// DeleteMailboxByID removes the mailbox row with the given internal
// id.
func DeleteMailboxByID(tx *sql.Tx, id int64) error {
	_, err := tx.Exec(`DELETE FROM mailbox WHERE id = ?`, id)
	return err
}

// StaleMailboxIDs returns the internal ids of accountID's mailbox
// rows whose server id is absent from keep.
func StaleMailboxIDs(tx *sql.Tx, accountID int64, keep map[string]bool) ([]int64, error) {
	rows, err := tx.Query(`SELECT id, server_id FROM mailbox WHERE account_id = ?`, accountID)
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

// RepairMailboxAssociations re-associates any message in accountID
// whose stored mailbox_ids (message.data, written by UpsertMessage)
// names mailboxServerID but has no message_mailbox row for mailboxID
// yet: that message's own sync page arrived before this mailbox's
// local row existed, so SyncMessageMailboxes skipped the association.
// UpsertMailbox calls this right after inserting a new mailbox row,
// closing the ordering gap instead of waiting on a later update of
// that same message to repair it.
func RepairMailboxAssociations(tx *sql.Tx, accountID, mailboxID int64, mailboxServerID string) error {
	rows, err := tx.Query(
		`SELECT m.id, m.received_at, m.flags, m.data FROM message m
		 WHERE m.account_id = ?
		 AND EXISTS (SELECT 1 FROM json_each(json_extract(m.data, '$.mailbox_ids')) je WHERE je.value = ?)
		 AND m.id NOT IN (SELECT message_id FROM message_mailbox WHERE mailbox_id = ?)`,
		accountID, mailboxServerID, mailboxID,
	)
	if err != nil {
		return err
	}

	type orphan struct {
		id, receivedAt int64
		unread         bool
		mailboxIDs     []string
	}
	var orphans []orphan
	for rows.Next() {
		var o orphan
		var flags int64
		var data string
		if err := rows.Scan(&o.id, &o.receivedAt, &flags, &data); err != nil {
			_ = rows.Close()
			return err
		}
		var md messageData
		if err := json.Unmarshal([]byte(data), &md); err != nil {
			_ = rows.Close()
			return err
		}
		o.unread = Flags(flags)&FlagSeen == 0 //nolint:gosec // G115: message.flags is written only through EncodeFlags's uint32 bitfield, never a value outside its range
		o.mailboxIDs = md.MailboxIDs
		orphans = append(orphans, o)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, o := range orphans {
		if err := SyncMessageMailboxes(tx, accountID, o.id, o.receivedAt, o.unread, o.mailboxIDs); err != nil {
			return err
		}
	}
	return nil
}
