package compose

import mailcompose "github.com/glw907/poplar/internal/compose"

// SeededMsg carries a pre-filled Draft from r/R/f.
type SeededMsg struct {
	Draft mailcompose.Draft
}

// SentMsg fires after QueueOutbound returns; App stages a non-undoable
// "Sending…" toast.
type SentMsg struct{}

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
