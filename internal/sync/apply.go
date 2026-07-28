package sync

import (
	"database/sql"
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
// for kind into account accountID, inside tx. A record whose id is in
// skip is left untouched: it is the dispatcher's own change, already
// reflected locally, that this same page's self-echo suppression
// (ADR-0005 revision 2) is skipping, not the whole page.
func applyChangeSet(tx *sql.Tx, accountID int64, kind backend.ObjectKind, cs backend.ChangeSet, skip map[string]bool) error {
	for _, rec := range cs.Created {
		if skip[rec.ID] {
			continue
		}
		if err := upsertRecord(tx, accountID, kind, rec); err != nil {
			return err
		}
	}
	for _, rec := range cs.Updated {
		if skip[rec.ID] {
			continue
		}
		if err := upsertRecord(tx, accountID, kind, rec); err != nil {
			return err
		}
	}
	for _, id := range cs.Destroyed {
		if skip[id] {
			continue
		}
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
		return store.DeleteMessage(tx, accountID, serverID)
	case backend.ObjectKindMailbox:
		return store.DeleteMailbox(tx, accountID, serverID)
	default:
		return fmt.Errorf("sync: destroy: unsupported kind %v", kind)
	}
}

// upsertMailbox translates rec's backend field vocabulary into a
// store.MailboxUpsert and writes it. Every mailbox column and its
// JSON shape are store.UpsertMailbox's concern, not this package's;
// this function's only job is the vocabulary crossing.
func upsertMailbox(tx *sql.Tx, accountID int64, rec backend.Record) error {
	f := rec.Fields
	return store.UpsertMailbox(tx, accountID, store.MailboxUpsert{
		ServerID:    rec.ID,
		Role:        stringField(f, "role"),
		Name:        stringField(f, "name"),
		SortOrder:   int64Field(f, "sort_order"),
		TotalCount:  int64Field(f, "total_emails"),
		UnreadCount: int64Field(f, "unread_emails"),
	})
}

// upsertMessage translates rec's backend field vocabulary into a
// store.MessageUpsert and writes it, the message counterpart of
// upsertMailbox.
func upsertMessage(tx *sql.Tx, accountID int64, rec backend.Record) error {
	f := rec.Fields
	mailboxIDs, _ := f["mailbox_ids"].([]string)
	receivedAt, _ := f["received_at"].(time.Time)

	return store.UpsertMessage(tx, accountID, store.MessageUpsert{
		ServerID:      rec.ID,
		BlobID:        stringField(f, "blob_id"),
		ThreadKey:     stringField(f, "thread_id"),
		Subject:       stringField(f, "subject"),
		FromAddr:      firstAddress(f["from"]),
		Flags:         flagsFromFields(f),
		Size:          int64Field(f, "size"),
		HasAttachment: boolField(f, "has_attachment"),
		ReceivedAt:    receivedAt,
		MailboxIDs:    mailboxIDs,
		Unread:        !boolField(f, "seen"),
	})
}
