package sync

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
)

// fields decodes one backend.Record's Fields into the Go types
// Record's vocabulary pins per key. A key the record omits decodes to
// the zero value; a key present under another type is a backend
// defect, held in err until the caller reports it. Decoding the whole
// record before consulting err keeps the crossing flat, the shape
// database/sql uses for the same problem.
type fields struct {
	m   map[string]any
	err error
}

// field returns key's value as T. A wrong-typed value would otherwise
// decode to T's zero value and overwrite what the store already holds:
// an id list read as nil takes the message out of every folder, and a
// count read as 0 empties the folder's totals.
func field[T any](f *fields, key string) T {
	var want T
	v, ok := f.m[key]
	if !ok {
		return want
	}
	got, ok := v.(T)
	if !ok {
		if f.err == nil {
			f.err = fmt.Errorf("field %q has type %T, want %T", key, v, want)
		}
		return want
	}
	return got
}

func flagsFromFields(f *fields) store.Flags {
	var keywords []string
	for name, kw := range backend.MessageFlagKeywords {
		if field[bool](f, name) {
			keywords = append(keywords, kw)
		}
	}
	bits, _ := store.EncodeFlags(keywords)
	return bits
}

// firstAddress renders the first entry of an address-list field
// (Record.Fields' "from") as message.from_addr's single display
// string: "Name <email>" when a name is present, the bare address
// otherwise. internal/mail owns full envelope modeling from pass 3;
// this is the scalar column pass 1's list view reads.
func firstAddress(addrs []map[string]string) string {
	if len(addrs) == 0 {
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
	f := &fields{m: rec.Fields}
	up := store.MailboxUpsert{
		ServerID:    rec.ID,
		Role:        field[string](f, "role"),
		Name:        field[string](f, "name"),
		SortOrder:   field[int64](f, "sort_order"),
		TotalCount:  field[int64](f, "total_emails"),
		UnreadCount: field[int64](f, "unread_emails"),
	}
	if f.err != nil {
		return fmt.Errorf("sync: apply mailbox %s: %v", rec.ID, f.err)
	}
	return store.UpsertMailbox(tx, accountID, up)
}

// upsertMessage translates rec's backend field vocabulary into a
// store.MessageUpsert and writes it, the message counterpart of
// upsertMailbox.
func upsertMessage(tx *sql.Tx, accountID int64, rec backend.Record) error {
	f := &fields{m: rec.Fields}
	up := store.MessageUpsert{
		ServerID:      rec.ID,
		BlobID:        field[string](f, "blob_id"),
		ThreadKey:     field[string](f, "thread_id"),
		Subject:       field[string](f, "subject"),
		FromAddr:      firstAddress(field[[]map[string]string](f, "from")),
		Flags:         flagsFromFields(f),
		Size:          field[int64](f, "size"),
		HasAttachment: field[bool](f, "has_attachment"),
		ReceivedAt:    field[time.Time](f, "received_at"),
		MailboxIDs:    field[[]string](f, "mailbox_ids"),
		Unread:        !field[bool](f, "seen"),
	}
	if f.err != nil {
		return fmt.Errorf("sync: apply message %s: %v", rec.ID, f.err)
	}
	return store.UpsertMessage(tx, accountID, up)
}
