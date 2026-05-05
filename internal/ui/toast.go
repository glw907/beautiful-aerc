// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// triageOp identifies which triage action a toast or pendingAction
// represents. The empty value (opNone) means "no toast active".
type triageOp string

const (
	opNone           triageOp = ""
	opDelete         triageOp = "delete"
	opArchive        triageOp = "archive"
	opStar           triageOp = "star"
	opUnstar         triageOp = "unstar"
	opRead           triageOp = "read"
	opUnread         triageOp = "unread"
	opMove           triageOp = "move"
	opEmpty          triageOp = "empty"
	opSaveAttachment triageOp = "save-attachment"
)

// pendingAction is the App-owned state for an in-flight optimistic
// triage action. The zero value means "no toast active". Local
// roll-back lives entirely in the cache layer (the inverse Cmd queues
// a compensating QueueOp). App holds only what the toast needs to
// render plus the undo Cmd to fire on `u`.
type pendingAction struct {
	op       triageOp
	n        int       // affected message count
	dest     string    // destination folder name, non-empty for opMove
	inverse  tea.Cmd   // the undo Cmd, nil for unrecoverable ops
	deadline time.Time // monotonic moment at which the toast expires
}

// IsZero reports whether p represents "no active toast". Every active
// pending action has op set (the verb is required for rendering), so a
// single check suffices.
func (p pendingAction) IsZero() bool { return p.op == opNone }

// renderToast produces the one-row toast string. Returns "" for the
// zero pendingAction. Width-bounded, truncates with ellipsis.
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
	default:
		body = fmt.Sprintf("%s %d %s", verb, p.n, pluralize("message", p.n))
	}
	hint := "[u undo]"
	if p.op == opEmpty || p.op == opSaveAttachment {
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
		return styles.Toast.Render(truncateToWidth(full, width))
	}
	hintW := lipgloss.Width(hint)
	bodyBudget := width - hintW - 4 // "✓ " + "   "
	if bodyBudget < 1 {
		return styles.Toast.Render(truncateToWidth(full, width))
	}
	bodyTrunc := truncateToWidth("✓ "+body, bodyBudget+2)
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
	}
	return string(op)
}

func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
