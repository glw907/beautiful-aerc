package theme

import "charm.land/lipgloss/v2"

// BorderKind selects one of the design language's three border
// weights (design decision 4).
type BorderKind int

// The three border kinds: single-line pane dividers, rounded modal
// and card frames, and the heavy focused-pane border. The focused
// weight is also the design language's degrade-table channel for
// the focused state (section 7), so it never changes with Profile.
const (
	BorderDivider BorderKind = iota
	BorderModal
	BorderFocused
)

// Border returns the lipgloss border set for kind.
func (t Theme) Border(kind BorderKind) lipgloss.Border {
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
