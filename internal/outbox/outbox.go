// Package outbox is poplar's durable intent queue (ADR-0006): every
// mutation a user or engine wants to make against the backend becomes
// an intent row, enqueued alongside its optimistic effect in one
// writer transaction, dispatched by draining queued rows in order,
// and either annihilated (undone before it ever reached the wire) or
// dispatched and, on failure, requeued or reported.
//
// Payloads reference poplar's internal keys only: a message or
// mailbox by its store row id, never a server id. The dispatcher
// resolves those keys to server ids at dispatch time, including an
// offline-created referent through another intent's own id as a
// creation-id back-reference (Kind, CreateMailboxPayload.ResolvedServerID).
package outbox

import "time"

// Kind names the mutation an intent describes.
type Kind string

const (
	// KindCreateMailbox creates a mailbox (CreateMailboxPayload).
	KindCreateMailbox Kind = "create_mailbox"
	// KindRenameMailbox renames an existing mailbox (RenameMailboxPayload).
	KindRenameMailbox Kind = "rename_mailbox"
	// KindDeleteMailbox removes an existing mailbox (DeleteMailboxPayload).
	KindDeleteMailbox Kind = "delete_mailbox"
	// KindMoveMessages moves messages into a mailbox (MoveMessagesPayload).
	KindMoveMessages Kind = "move_messages"
)

// UndoWindow is UX-9's hold: an undoable intent is enqueued with its
// next_attempt_at this far out, so it stays queued, and so
// annihilable, until the window closes.
const UndoWindow = 10 * time.Second

// CreateMailboxPayload is KindCreateMailbox's payload. ParentMailboxID
// names an existing mailbox row; ParentRef names another queued
// KindCreateMailbox intent's own id, for a parent created in the same
// offline session. ResolvedServerID is empty until the dispatcher's
// backend call succeeds, at which point it is persisted back into
// this same row: a replay that finds it already set skips the backend
// call, since CreateMailbox has no natural idempotency of its own.
type CreateMailboxPayload struct {
	Name             string `json:"name"`
	ParentMailboxID  int64  `json:"parent_mailbox_id,omitempty"`
	ParentRef        int64  `json:"parent_ref,omitempty"`
	ResolvedServerID string `json:"resolved_server_id,omitempty"`
}

// RenameMailboxPayload is KindRenameMailbox's payload.
type RenameMailboxPayload struct {
	MailboxID int64  `json:"mailbox_id"`
	Name      string `json:"name"`
}

// DeleteMailboxPayload is KindDeleteMailbox's payload.
type DeleteMailboxPayload struct {
	MailboxID int64 `json:"mailbox_id"`
}

// MoveMessagesPayload is KindMoveMessages's payload: one chunk of a
// bulk move, or a standalone move of one message. DestMailboxID names
// an existing mailbox row; DestRef names a KindCreateMailbox intent's
// own id instead, when the destination was itself created offline in
// the same batch. PriorMailboxIDs records each message's mailbox
// before this move (its only mailbox, pass 1's move semantics), so a
// caller can build the exact compensating move without a second store
// read once the original intent's row is gone.
type MoveMessagesPayload struct {
	MessageIDs      []int64         `json:"message_ids"`
	DestMailboxID   int64           `json:"dest_mailbox_id,omitempty"`
	DestRef         int64           `json:"dest_ref,omitempty"`
	PriorMailboxIDs map[int64]int64 `json:"prior_mailbox_ids,omitempty"`
}
