// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// pendingAction is the App-owned state for an in-flight optimistic
// triage action. The zero value means "no toast active".
type pendingAction struct {
	op       string    // "delete" | "archive" | "star" | "unstar" | "read" | "unread" | "move"
	n        int       // affected message count
	dest     string    // destination folder name; non-empty for "move"
	inverse  tea.Cmd   // the undo Cmd; nil for unrecoverable ops
	deadline time.Time // monotonic moment at which the toast expires
	onUndo   func()    // local roll-back; runs on `u` and on ErrorMsg
}

// IsZero reports whether p represents "no active toast". Every active
// pending action has op set (the verb is required for rendering), so a
// single check suffices.
func (p pendingAction) IsZero() bool { return p.op == "" }

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
	case "move":
		body = fmt.Sprintf("%s %d %s to %s", verb, p.n, pluralize("message", p.n), p.dest)
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
	case "move":
		return "Moved"
	}
	return op
}

func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
