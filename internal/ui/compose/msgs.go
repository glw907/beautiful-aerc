// SPDX-License-Identifier: MIT

package compose

import mailcompose "github.com/glw907/poplar/internal/compose"

// SeededMsg carries a pre-filled Draft from r/R/f. App opens
// Model and calls Seed when this msg arrives.
type SeededMsg struct {
	Draft mailcompose.Draft
}

// SentMsg fires after QueueOutbound returns. App stages a
// non-undoable "Sending…" toast.
type SentMsg struct{}

// EnqueuePushDraftMsg signals App to queue a PushDraft op against
// the cache outbox. Compose emits this from the 5-min server-push
// timer and from the close-with-save path.
type EnqueuePushDraftMsg struct {
	DraftID       string
	Folder        string
	MIME          []byte
	PrevServerUID string
}

// DraftPersistedMsg fires after a successful UpsertDraft.
type DraftPersistedMsg struct {
	DraftID string
}
