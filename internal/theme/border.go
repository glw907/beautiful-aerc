package theme

import "charm.land/lipgloss/v2"

// BorderKind selects one of the design language's three border
// weights (design decision 4).
type BorderKind int

// The three border kinds: single-line pane dividers, rounded modal
// and card frames, and the heavy focused-pane border.
const (
	BorderDivider BorderKind = iota
	BorderModal
	BorderFocused
)

// Border returns the lipgloss border set for kind. At
// ProfileTrueColor the three kinds render distinct weights; at
// ProfileANSI16 and ProfileNoColor every kind collapses to
// lipgloss.ASCIIBorder, since box-drawing weight is not a channel
// those profiles can render reliably. The focused state's own
// degrade-table channel is the edge bar's glyph weight
// (GlyphSet.EdgeBarFocused vs EdgeBarBlurred), not the border.
func (t Theme) Border(kind BorderKind) lipgloss.Border {
	if t.profile != ProfileTrueColor {
		return lipgloss.ASCIIBorder()
	}
	switch kind {
	case BorderModal:
		return lipgloss.RoundedBorder()
	case BorderFocused:
		return lipgloss.ThickBorder()
	default:
		return lipgloss.NormalBorder()
	}
}

// brailleFrames and asciiSpinnerFrames are the spinner's two frame
// sets (design decision 4).
var (
	brailleFrames      = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	asciiSpinnerFrames = []string{"|", "/", "-", "\\"}
)

// Spinner returns t's spinner frame cycle: braille at
// ProfileTrueColor, the ASCII cycle at ProfileANSI16 and
// ProfileNoColor.
func (t Theme) Spinner() []string {
	if t.profile == ProfileTrueColor {
		return brailleFrames
	}
	return asciiSpinnerFrames
}
