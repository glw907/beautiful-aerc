package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	mailcompose "github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// triageOp aliases uicore.TriageOp so App-side toast code reads without
// a package prefix. The canonical type lives in uicore so account can
// produce TriageStartedMsg values.
type triageOp = uicore.TriageOp

const (
	opNone           = uicore.TriageNone
	opDelete         = uicore.TriageDelete
	opArchive        = uicore.TriageArchive
	opStar           = uicore.TriageStar
	opUnstar         = uicore.TriageUnstar
	opRead           = uicore.TriageRead
	opUnread         = uicore.TriageUnread
	opMove           = uicore.TriageMove
	opEmpty          = uicore.TriageEmpty
	opSaveAttachment = uicore.TriageSaveAttachment
	opSending        = uicore.TriageSending
	opSendUndo       = uicore.TriageSendUndo
)

// pendingAction is App's state for an in-flight optimistic triage or a
// queued send. The zero value means no toast is active.
//
// For triage ops, inverse is the compensating Cmd; local roll-back lives
// in the cache layer. For opSendUndo, inverse is nil; cancellation goes
// through CancelOps using the sendOpIDs slice.
type pendingAction struct {
	op       triageOp
	n        int       // affected message count
	dest     string    // destination folder name, non-empty for opMove
	inverse  tea.Cmd   // the undo Cmd, nil for unrecoverable or send-undo ops
	deadline time.Time // moment at which the toast expires

	// populated only when op == opSendUndo
	sendOpIDs []int64
	sendDraft mailcompose.Draft
}

// IsZero reports whether p represents no active toast. Every active
// pending action has op set, so a single check suffices.
func (p pendingAction) IsZero() bool { return p.op == opNone }

// renderToast produces the one-row toast string, "" for the zero value.
// Truncated with ellipsis to fit width.
func renderToast(p pendingAction, width int, styles Styles) string {
	if p.IsZero() {
		return ""
	}
	verb := toastVerb(p.op)
	var body string
	switch p.op {
	case opStar, opUnstar, opRead, opUnread:
		if p.n > 1 {
			body = fmt.Sprintf("%s %d", verb, p.n)
		} else {
			body = verb
		}
	case opMove:
		body = fmt.Sprintf("%s %d %s to %s", verb, p.n, pluralize("message", p.n), p.dest)
	case opEmpty:
		body = fmt.Sprintf("%s %s (%d)", verb, p.dest, p.n)
	case opSaveAttachment:
		body = fmt.Sprintf("Saved to %s", p.dest)
	case opSending:
		body = "Sending…"
	default:
		body = fmt.Sprintf("%s %d %s", verb, p.n, pluralize("message", p.n))
	}
	hint := "[u undo]"
	if p.op == opEmpty || p.op == opSaveAttachment || p.op == opSending {
		hint = ""
	}
	full := "✓ " + body
	if hint != "" {
		full = full + "   " + hint
	}
	if lipgloss.Width(full) <= width {
		return styles.Toast.Render(full)
	}
	if hint == "" {
		return styles.Toast.Render(uicore.TruncateToWidth(full, width))
	}
	hintW := lipgloss.Width(hint)
	bodyBudget := width - hintW - 4 // "✓ " + "   "
	if bodyBudget < 1 {
		return styles.Toast.Render(uicore.TruncateToWidth(full, width))
	}
	bodyTrunc := uicore.TruncateToWidth("✓ "+body, bodyBudget+2)
	return styles.Toast.Render(bodyTrunc + "   " + hint)
}

func toastVerb(op triageOp) string {
	switch op {
	case opDelete:
		return "Deleted"
	case opArchive:
		return "Archived"
	case opStar:
		return "Starred"
	case opUnstar:
		return "Unstarred"
	case opRead:
		return "Marked read"
	case opUnread:
		return "Marked unread"
	case opMove:
		return "Moved"
	case opEmpty:
		return "Emptied"
	case opSaveAttachment:
		return "Saved"
	case opSending:
		return "Sending"
	}
	return string(op)
}

func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
