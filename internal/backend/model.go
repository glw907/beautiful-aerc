package backend

import "time"

// RecordFields is one Record's payload: MessageFields for
// ObjectKindMessage, MailboxFields for ObjectKindMailbox. Only this
// package implements it, so the set of payloads a consumer has to
// handle is closed, and no consumer can reach a field belonging to
// another kind. ObjectKindEvent and ObjectKindContact gain their own
// payloads once internal/calendar and internal/contacts land.
type RecordFields interface{ recordFields() }

// MessageFields is a message record's hydrated payload, poplar's own
// vocabulary rather than any backend's wire property names. A backend
// fills what its protocol supplies and leaves the rest at the zero
// value. MailboxIDs is the message's whole folder membership rather
// than a delta, since internal/sync reconciles message_mailbox
// against it, so a backend that leaves it at its zero value (nil)
// removes the message from every folder rather than leaving its
// membership untouched.
type MessageFields struct {
	BlobID        string
	ThreadKey     string
	Subject       string
	From          []Address
	MailboxIDs    []string
	ReceivedAt    time.Time
	Size          int64
	HasAttachment bool
	Flags         MessageFlags
}

func (MessageFields) recordFields() {}

// MailboxFields is a mailbox record's hydrated payload. Role is the
// role the server declared and is empty when it declared none;
// internal/store owns the name heuristic that fills that gap (FO-1).
type MailboxFields struct {
	Role        string
	Name        string
	SortOrder   int64
	TotalCount  int64
	UnreadCount int64
}

func (MailboxFields) recordFields() {}

// Address is one mail address: the display name, empty when the
// message carries none, and the addr-spec.
type Address struct {
	Name  string
	Email string
}

// MessageFlags is a set of the message flags poplar tracks. The zero
// value is the empty set, which is what a hydrated record carries for
// a message the server holds no keywords for.
type MessageFlags uint8

// The flags a message record carries and a message patch changes.
// Each is a one-element MessageFlags, so a set is their bitwise or.
const (
	FlagSeen MessageFlags = 1 << iota
	FlagFlagged
	FlagAnswered
	FlagDraft
	FlagForwarded
)

// MessageFlagKeywords maps each flag to the keyword it stands for on
// the wire and in the store. A backend translates its own keyword set
// through this table on the way out of Changes and back through it on
// the way into ApplyBatch; internal/sync reads the same table to reach
// store.EncodeFlags's keyword vocabulary.
var MessageFlagKeywords = map[MessageFlags]string{
	FlagSeen:      "$seen",
	FlagFlagged:   "$flagged",
	FlagAnswered:  "$answered",
	FlagDraft:     "$draft",
	FlagForwarded: "$forwarded",
}

// MutationFields is one Mutation's payload: MailboxCreate for a
// mailbox create, MessagePatch for a message update. A destroy
// carries none, since Mutation.ID names everything the backend needs.
type MutationFields interface{ mutationFields() }

// MailboxCreate is a mailbox create's payload. An empty ParentID
// creates the mailbox at the root.
type MailboxCreate struct {
	Name     string
	ParentID string
}

func (MailboxCreate) mutationFields() {}

// MessagePatch is a message update's payload, carrying what the
// update changes and nothing else. SetFlags names the flags to turn
// on and ClearFlags the ones to turn off, so a flag in neither keeps
// whatever the server holds, and a flag in both resolves to set. A
// nil MailboxIDs leaves the message's folder membership alone; a
// non-nil one replaces it whole.
type MessagePatch struct {
	SetFlags   MessageFlags
	ClearFlags MessageFlags
	MailboxIDs []string
}

func (MessagePatch) mutationFields() {}
