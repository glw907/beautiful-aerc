// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
)

// pendingAction is the App-owned state for an in-flight optimistic
// triage action. The zero value means "no toast active".
type pendingAction struct {
	op       string    // "delete" | "archive" | "star" | "unstar" | "read" | "unread"
	n        int       // affected message count
	inverse  tea.Cmd   // the undo Cmd; nil for unrecoverable ops
	deadline time.Time // monotonic moment at which the toast expires
	onUndo   func()    // local roll-back; runs on `u` and on ErrorMsg
	uids     []mail.UID
}

// IsZero reports whether p represents "no active toast".
func (p pendingAction) IsZero() bool {
	return p.op == "" && p.n == 0 && p.inverse == nil && p.deadline.IsZero()
}

// renderToast produces the one-row toast string. Returns "" for the
// zero pendingAction. Width-bounded; truncates with ellipsis.
func renderToast(p pendingAction, width int, styles Styles) string {
	if p.IsZero() {
		return ""
	}
	verb := toastVerb(p.op)
	var body string
	switch p.op {
	case "star", "unstar", "read", "unread":
		if p.n > 1 {
			body = fmt.Sprintf("%s %d", verb, p.n)
		} else {
			body = verb
		}
	default:
		body = fmt.Sprintf("%s %d %s", verb, p.n, pluralize("message", p.n))
	}
	hint := "[u undo]"
	full := "✓ " + body + "   " + hint
	if lipgloss.Width(full) <= width {
		return styles.Toast.Render(full)
	}
	hintW := lipgloss.Width(hint)
	bodyBudget := width - hintW - 4 // "✓ " + "   "
	if bodyBudget < 1 {
		return styles.Toast.Render(truncateToWidth(full, width))
	}
	bodyTrunc := truncateToWidth("✓ "+body, bodyBudget+2)
	return styles.Toast.Render(bodyTrunc + "   " + hint)
}

func toastVerb(op string) string {
	switch op {
	case "delete":
		return "Deleted"
	case "archive":
		return "Archived"
	case "star":
		return "Starred"
	case "unstar":
		return "Unstarred"
	case "read":
		return "Marked read"
	case "unread":
		return "Marked unread"
	}
	return op
}

func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
