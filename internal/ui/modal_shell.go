// SPDX-License-Identifier: MIT

package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ModalShell holds the shared lifecycle state for modal overlay components:
// open flag and terminal dimensions. Each overlay embeds a named shell field
// (not anonymous) to keep grep-ability and avoid Width/Height method
// collisions with parent types.
//
// Box renders the ┌─ title ─┐ / content rows / ├─┤ footer rows / └─┘ frame.
// The caller is responsible for computing contentW and building the rows;
// ModalShell owns only the frame chrome.
type ModalShell struct {
	open          bool
	width, height int
}

// IsOpen reports whether the modal overlay is currently visible.
func (s ModalShell) IsOpen() bool { return s.open }

// WithOpen returns a shell with open set to the given value.
func (s ModalShell) WithOpen(open bool) ModalShell { s.open = open; return s }

// SetSize returns a shell with updated terminal dimensions.
func (s ModalShell) SetSize(w, h int) ModalShell { s.width, s.height = w, h; return s }

// Width returns the terminal width stored on the shell.
func (s ModalShell) Width() int { return s.width }

// Height returns the terminal height stored on the shell.
func (s ModalShell) Height() int { return s.height }

// Box renders the shared ┌─ title ─┐ / ├─┤ / └─┘ frame around the caller-
// supplied sections. bodyRows are the main content lines; footerRows are
// rendered between the ├─┤ separator and the └─┘ bottom border; both are
// rendered with │ side borders. Each row in bodyRows and footerRows must
// already be padded or truncated to exactly contentW display cells — Box
// does not re-pad them, it only adds the │ borders.
//
// title is plain text; Box truncates it if "─ title " + borders would
// exceed contentW+2 (the full box width). The title border uses
// lipgloss.Width (icon-free strings), consistent with the invariant.
//
// styles supplies Dim for the separator style; no lipgloss.NewStyle calls
// are made inside Box.
func (s ModalShell) Box(styles Styles, title string, bodyRows []string, footerRows []string, contentW int) string {
	boxW := contentW + 2

	// Top border: ┌─ title ─┐
	// Overhead: "┌─" (2) + " " (before title, already in caller's title) +
	// "─┐" (2) = 4. Title is wrapped in spaces by convention: " Title ".
	// We compute remaining dashes after "┌─" + title + "┐".
	titleSeg := " " + title + " "
	// Overhead for the top border line: "┌─" (2 cells) + "┐" (1 cell) = 3.
	// Remaining dashes = boxW - 3 - lipgloss.Width(titleSeg).
	// Truncate title when it would leave no room for dashes.
	maxTitleW := boxW - 3
	if maxTitleW < 0 {
		maxTitleW = 0
	}
	if lipgloss.Width(titleSeg) > maxTitleW {
		titleSeg = " " + truncateToWidth(title, maxTitleW-2) + " "
	}
	rest := boxW - 3 - lipgloss.Width(titleSeg)
	if rest < 0 {
		rest = 0
	}

	var b strings.Builder
	b.WriteString("┌─" + titleSeg + strings.Repeat("─", rest) + "┐\n")

	// Body rows.
	for _, row := range bodyRows {
		b.WriteString("│" + row + "│\n")
	}

	// Separator between body and footer.
	if len(footerRows) > 0 {
		b.WriteString("├" + strings.Repeat("─", contentW) + "┤\n")
		for _, row := range footerRows {
			b.WriteString("│" + row + "│\n")
		}
	}

	// Bottom border.
	b.WriteString("└" + strings.Repeat("─", contentW) + "┘")

	return b.String()
}
