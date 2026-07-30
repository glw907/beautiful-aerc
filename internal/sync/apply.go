package sync

import (
	"database/sql"
	"fmt"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
)

// storeFlags encodes set as the store's flag bits. The seam and the
// store number their bits independently, so the crossing goes through
// the keyword both of them name rather than assuming the two layouts
// line up.
func storeFlags(set backend.MessageFlags) store.Flags {
	var keywords []string
	for flag, keyword := range backend.MessageFlagKeywords {
		if set&flag != 0 {
			keywords = append(keywords, keyword)
		}
	}
	bits, _ := store.EncodeFlags(keywords)
	return bits
}

// firstAddress renders the first entry of an address list as
// message.from_addr's single display string: "Name <email>" when a
// name is present, the bare address otherwise. internal/mail owns full
// envelope modeling from pass 3; this is the scalar column pass 1's
// list view reads.
func firstAddress(addrs []backend.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	if addrs[0].Name != "" {
		return addrs[0].Name + " <" + addrs[0].Email + ">"
	}
	return addrs[0].Email
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

// upsertRecord writes rec by the type of its payload, checked against
// kind, the collection the Changes call that produced rec asked for.
// The payload type and kind are two discriminators this package
// already relies on separately: destroyRecord and staleIDs switch on
// kind alone, since a destroyed or stale record carries no payload to
// check. This is where the two discriminators meet, so it is where a
// payload that disagrees with the page it arrived on is caught
// before it writes into the wrong table.
func upsertRecord(tx *sql.Tx, accountID int64, kind backend.ObjectKind, rec backend.Record) error {
	switch f := rec.Fields.(type) {
	case backend.MessageFields:
		if kind == backend.ObjectKindMessage {
			return upsertMessage(tx, accountID, rec.ID, f)
		}
	case backend.MailboxFields:
		if kind == backend.ObjectKindMailbox {
			return upsertMailbox(tx, accountID, rec.ID, f)
		}
	default:
		return fmt.Errorf("sync: apply: record %s carries unsupported fields %T", rec.ID, rec.Fields)
	}
	return fmt.Errorf("sync: apply: record %s carries %T on a %s page", rec.ID, rec.Fields, kindName(kind))
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

// upsertMailbox translates f's backend vocabulary into a
// store.MailboxUpsert and writes it under serverID. Every mailbox
// column and its JSON shape are store.UpsertMailbox's concern, not
// this package's; this function's only job is the vocabulary
// crossing.
func upsertMailbox(tx *sql.Tx, accountID int64, serverID string, f backend.MailboxFields) error {
	return store.UpsertMailbox(tx, accountID, store.MailboxUpsert{
		ServerID:    serverID,
		Role:        f.Role,
		Name:        f.Name,
		SortOrder:   f.SortOrder,
		TotalCount:  f.TotalCount,
		UnreadCount: f.UnreadCount,
	})
}

// upsertMessage translates f's backend vocabulary into a
// store.MessageUpsert and writes it, the message counterpart of
// upsertMailbox.
func upsertMessage(tx *sql.Tx, accountID int64, serverID string, f backend.MessageFields) error {
	return store.UpsertMessage(tx, accountID, store.MessageUpsert{
		ServerID:      serverID,
		BlobID:        f.BlobID,
		ThreadKey:     f.ThreadKey,
		Subject:       f.Subject,
		FromAddr:      firstAddress(f.From),
		Flags:         storeFlags(f.Flags),
		Size:          f.Size,
		HasAttachment: f.HasAttachment,
		ReceivedAt:    f.ReceivedAt,
		MailboxIDs:    f.MailboxIDs,
		Unread:        f.Flags&backend.FlagSeen == 0,
	})
}
