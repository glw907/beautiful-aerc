package compose

import (
	"time"

	mailcompose "github.com/glw907/poplar/internal/compose"
)

// SeededMsg carries a pre-filled Draft from r/R/f.
type SeededMsg struct {
	Draft mailcompose.Draft
}

// SentMsg fires after QueueOutbound returns. App stages send-undo
// state from these fields when ScheduledFor is in the future.
type SentMsg struct {
	OpIDs        []int64
	ScheduledFor time.Time  // zero = no hold (dispatch immediately)
	DraftID      string     // links the persisted drafts row
	Draft        mailcompose.Draft // in-memory copy for fast u-undo restore
}

// EnqueuePushDraftMsg asks App to queue a PushDraft outbox op. Compose
// emits this from the 5-min server-push timer and the close-with-save path.
type EnqueuePushDraftMsg struct {
	DraftID       string
	Folder        string
	MIME          []byte
	PrevServerUID string
}

type DraftPersistedMsg struct {
	DraftID string
}

// AttachAcceptedMsg fires when the user accepts a selection in
// AttachPicker. Paths are absolute. Caller is responsible for
// dedupe against the current Draft.Attachments.
type AttachAcceptedMsg struct{ Paths []string }

// AttachCancelledMsg fires when the user dismisses AttachPicker
// without selecting.
type AttachCancelledMsg struct{}
