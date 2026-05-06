// SPDX-License-Identifier: MIT

package ui

import (
	"github.com/glw907/poplar/internal/ui/uicore"
)

// renderErrorBanner formats an ErrorMsg for the single banner row
// above the status bar. Returns "" when msg.Err is nil. Output is
// at most width display cells wide. Longer text is truncated with
// "…". When msg.Op is empty, the format is "⚠ <err>". Otherwise
// "⚠ <op>: <err>".
func renderErrorBanner(msg ErrorMsg, width int, styles Styles) string {
	if msg.Err == nil {
		return ""
	}
	text := "⚠ "
	if msg.Op != "" {
		text += msg.Op + ": "
	}
	text += msg.Err.Error()
	text = uicore.TruncateToWidth(text, width)
	return styles.ErrorBanner.Render(text)
}
