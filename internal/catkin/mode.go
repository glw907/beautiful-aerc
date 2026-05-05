package catkin

// DisplayMode controls Catkin's typewriter scroll and focus
// dimming. Modes cycle Normal → Typewriter → Focus →
// Typewriter+Focus → Normal via Ctrl+\.
type DisplayMode int

const (
	ModeNormal DisplayMode = iota
	ModeTypewriter
	ModeFocus
	ModeFocusTypewriter
)

func (m DisplayMode) typewriter() bool { return m == ModeTypewriter || m == ModeFocusTypewriter }
func (m DisplayMode) focus() bool      { return m == ModeFocus || m == ModeFocusTypewriter }

func (m DisplayMode) next() DisplayMode {
	if m >= ModeFocusTypewriter {
		return ModeNormal
	}
	return m + 1
}

// activeParagraphRange returns [first, last] line indices (inclusive)
// of the paragraph containing cursorRow. A paragraph is a run of
// non-blank lines. If cursorRow is on a blank, the range is just
// the blank itself.
func activeParagraphRange(ctxs []LineContext, cursorRow int) (int, int) {
	if cursorRow < 0 || cursorRow >= len(ctxs) {
		return cursorRow, cursorRow
	}
	first := cursorRow
	for first > 0 && ctxs[first-1].Kind != BlockBlank {
		first--
	}
	last := cursorRow
	for last < len(ctxs)-1 && ctxs[last+1].Kind != BlockBlank {
		last++
	}
	return first, last
}
