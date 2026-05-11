package compose

import (
	"time"

	"github.com/glw907/poplar/internal/mailcompose"
)

// SeededMsg carries a pre-filled Draft from r/R/f.
type SeededMsg struct {
	Draft mailcompose.Draft
}

// SentMsg fires after QueueOutbound returns. App stages send-undo
// state from these fields when ScheduledFor is in the future.
type SentMsg struct {
	OpIDs        []int64
	ScheduledFor time.Time         // zero = no hold (dispatch immediately)
	Draft        mailcompose.Draft // in-memory copy for fast undo restore
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

// ScheduleAcceptedMsg is emitted when the user picks a schedule preset
// or commits a parsed custom time.
type ScheduleAcceptedMsg struct{ When time.Time }

// ScheduleCancelledMsg is emitted when the user dismisses the picker.
type ScheduleCancelledMsg struct{}
