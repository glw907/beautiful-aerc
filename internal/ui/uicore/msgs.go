package uicore

// ErrorMsg carries a failure from any tea.Cmd. App captures the most
// recent ErrorMsg into lastErr and renders "⚠ <Op>: <Err>" above the
// status bar. Last-write-wins: a subsequent ErrorMsg replaces the prior.
type ErrorMsg struct {
	Op  string
	Err error
}

// TriageOp identifies which triage action a toast or pendingAction
// represents. The empty value (TriageNone) means "no toast active".
type TriageOp string

const (
	TriageNone           TriageOp = ""
	TriageDelete         TriageOp = "delete"
	TriageArchive        TriageOp = "archive"
	TriageStar           TriageOp = "star"
	TriageUnstar         TriageOp = "unstar"
	TriageRead           TriageOp = "read"
	TriageUnread         TriageOp = "unread"
	TriageMove           TriageOp = "move"
	TriageEmpty          TriageOp = "empty"
	TriageSaveAttachment TriageOp = "save-attachment"
	TriageSending        TriageOp = "sending"
	TriageSendUndo       TriageOp = "send-undo"
)
