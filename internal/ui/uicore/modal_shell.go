package uicore

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ModalShell holds the shared lifecycle state (open flag and dimensions)
// for modal overlay components. Overlays embed a named field (not anonymous)
// to keep it greppable and to avoid Width/Height collisions with parents.
type ModalShell struct {
	open          bool
	width, height int
}

func (s ModalShell) IsOpen() bool                  { return s.open }
func (s ModalShell) WithOpen(open bool) ModalShell { s.open = open; return s }
func (s ModalShell) SetSize(w, h int) ModalShell   { s.width, s.height = w, h; return s }
func (s ModalShell) Width() int                    { return s.width }
func (s ModalShell) Height() int                   { return s.height }

// Box renders the ┌─ title ─┐ / ├─┤ / └─┘ frame around the caller-supplied
// rows. Each row in bodyRows and footerRows must already be padded or
// truncated to exactly contentW display cells. Box only adds the side
// borders. The title is truncated to fit when the framed width would exceed
// contentW+2.
func (s ModalShell) Box(title string, bodyRows []string, footerRows []string, contentW int) string {
	boxW := contentW + 2

	titleSeg := " " + title + " "
	maxTitleW := boxW - 3
	if maxTitleW < 0 {
		maxTitleW = 0
	}
	if lipgloss.Width(titleSeg) > maxTitleW {
		tgtW := maxTitleW - 2
		if tgtW < 1 {
			titleSeg = ""
		} else {
			titleSeg = " " + TruncateToWidth(title, tgtW) + " "
		}
	}
	rest := boxW - 3 - lipgloss.Width(titleSeg)
	if rest < 0 {
		rest = 0
	}

	var b strings.Builder
	b.WriteString("┌─" + titleSeg + strings.Repeat("─", rest) + "┐\n")

	for _, row := range bodyRows {
		b.WriteString("│" + row + "│\n")
	}

	if len(footerRows) > 0 {
		b.WriteString("├" + strings.Repeat("─", contentW) + "┤\n")
		for _, row := range footerRows {
			b.WriteString("│" + row + "│\n")
		}
	}

	b.WriteString("└" + strings.Repeat("─", contentW) + "┘")

	return b.String()
}
