package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TopLine renders the top frame edge: ──┬──╮ with optional toast overlay.
type TopLine struct {
	styles Styles
	toast  string
}

// NewTopLine creates a TopLine with the given styles.
func NewTopLine(styles Styles) TopLine {
	return TopLine{styles: styles}
}

// SetToast sets a toast message to overlay on the right side.
func (tl *TopLine) SetToast(msg string) {
	tl.toast = msg
}

// ClearToast removes the toast message.
func (tl *TopLine) ClearToast() {
	tl.toast = ""
}

// View renders the top line at the given width. dividerCol is the
// column position of the panel divider (0 to skip the junction).
func (tl TopLine) View(width, dividerCol int) string {
	style := tl.styles.TopLine

	// Build the right portion: " toast ─╮" or just "─╮"
	rightEnd := "─╮"
	var toastPart string
	if tl.toast != "" {
		toastPart = " " + tl.toast + " "
	}
	rightEndWidth := lipgloss.Width(rightEnd)
	toastWidth := lipgloss.Width(toastPart)

	// Fill the line with ─, placing ┬ at dividerCol
	fillWidth := width - rightEndWidth - toastWidth
	if fillWidth < 1 {
		fillWidth = 1
	}

	var buf strings.Builder
	for i := 0; i < fillWidth; i++ {
		if dividerCol > 0 && i == dividerCol {
			buf.WriteRune('┬')
		} else {
			buf.WriteRune('─')
		}
	}

	line := buf.String()
	if tl.toast != "" {
		return style.Render(line) +
			tl.styles.ToastText.Render(toastPart) +
			style.Render(rightEnd)
	}
	return style.Render(line + rightEnd)
}
