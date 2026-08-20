package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

// helpScreenName is HelpScreen's own registered state name, the
// switch table's own "help overlay" entry (design language section
// 2's eleven-entry digits-switch authority list).
const helpScreenName = "help overlay"

// helpGlobalKeys is the Global section's own curated set of
// GrammarKeys fields (UX-5, wireframe F5): the app-wide verbs every
// screen honors, regardless of which screen help was opened over.
// Every binding comes from GrammarKeys itself, never a re-typed key
// literal, so a grammar amendment can never leave this section stale.
var helpGlobalKeys = []key.Binding{
	GrammarKeys.Navigate, GrammarKeys.Page, GrammarKeys.Extremes,
	GrammarKeys.Open, GrammarKeys.Back, GrammarKeys.SurfaceSwitch,
	GrammarKeys.Undo, GrammarKeys.Help, GrammarKeys.Quit,
}

// HelpScreen is poplar's registry-derived help overlay (UX-5,
// wireframes F5/F6): a full-content-region takeover on App's own
// screen stack, its Global section built from helpGlobalKeys and its
// This-screen section from Covered's own keymap (helpContent), so
// neither section can ever drift from the registry that also drives
// the footer and the grammar checks. Covered is the entry help was
// pushed over, carried once at push time; theme and layout arrive
// through the same ThemeMsg/LayoutMsg every screen answers.
type HelpScreen struct {
	theme  theme.Theme
	layout LayoutMode
	scroll int

	Covered ScreenEntry
}

// Init implements Screen.
func (h HelpScreen) Init() tea.Cmd { return nil }

// Update implements Screen: LayoutMsg and ThemeMsg, the two every
// screen carries, plus the navigation family (j/k, Space/b, Home/End)
// scrolling the overlay's own body and a coalesced WheelMsg doing the
// same (ADR-0017 row 8). Esc never reaches here: App's own Back branch
// pops the stack before forwarding a key at all, since HelpScreen's
// SwitchState is StateDigitsSwitch, not StateModal (handleKey's own
// doc). A key outside this screen's own keymap, digits and Enter
// included, is a no-op: App's own precedence intercepts every one of
// those before a key ever reaches here.
func (h HelpScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LayoutMsg:
		h.layout = msg.Layout
	case ThemeMsg:
		h.theme = msg.Theme
	case WheelMsg:
		h.scroll += msg.Delta
	case tea.KeyPressMsg:
		h.scroll = h.applyKey(msg)
	default:
		return h, nil
	}
	h.scroll = clampScroll(h.scroll, len(h.lines()), h.viewportHeight())
	return h, nil
}

// applyKey returns h's own new scroll position for msg: Navigate
// steps one line, Page steps one viewport, and Extremes jumps to
// either end. A key matching none of the three (App's own precedence
// already filters out everything this screen does not itself bind)
// leaves the scroll position unchanged.
func (h HelpScreen) applyKey(msg tea.KeyPressMsg) int {
	switch {
	case key.Matches(msg, GrammarKeys.Navigate):
		if helpNavigateUp(msg) {
			return h.scroll - 1
		}
		return h.scroll + 1
	case key.Matches(msg, GrammarKeys.Page):
		if helpPageBack(msg) {
			return h.scroll - h.viewportHeight()
		}
		return h.scroll + h.viewportHeight()
	case key.Matches(msg, GrammarKeys.Extremes):
		if msg.String() == "home" {
			return 0
		}
		return len(h.lines())
	default:
		return h.scroll
	}
}

// helpNavigateUp reports whether msg is the Navigate binding's own
// "up" half (k, up) rather than its "down" half (j, down).
func helpNavigateUp(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "k", "up":
		return true
	default:
		return false
	}
}

// helpPageBack reports whether msg is the Page binding's own "back"
// half (b, pgup) rather than its "forward" half (space, pgdown).
func helpPageBack(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "b", "pgup":
		return true
	default:
		return false
	}
}

// clampScroll bounds scroll to [0, lineCount-viewport], the same
// range every wheel/key delta above lands within regardless of how
// far past either edge it reached.
func clampScroll(scroll, lineCount, viewport int) int {
	upper := max(0, lineCount-viewport)
	return min(max(scroll, 0), upper)
}

// viewportHeight returns h's own body height, the number of rows
// visible at once within the whole Main band (FullRegion's own
// rectangle, RenderInput's doc).
func (h HelpScreen) viewportHeight() int {
	return h.layout.Main.Rect.Dy()
}

// twoColumn reports whether h renders its Global and This-screen
// sections side by side (wireframe F6, 100 columns and up, the one
// layout boundary any pass-2 screen consumes) rather than stacked
// (F5, below it).
func (h HelpScreen) twoColumn() bool {
	return h.layout.Class >= WidthStandard
}

// lines returns h's own full, unscrolled body: one entry per rendered
// row, in column-mode-appropriate order. Update and View both call it,
// so a line count driving a clamp and a line set driving a render can
// never disagree with each other.
func (h HelpScreen) lines() [][]rowSeg {
	if h.twoColumn() {
		return helpLinesTwoColumn(h)
	}
	return helpLinesOneColumn(h)
}

// helpBlankLine is one empty spacer row within the overlay's own body.
var helpBlankLine = []rowSeg{}

// helpTitleSegs returns the overlay's own title row: "Help ·
// <covered's own state name>", covered.Name read directly rather than
// re-typed or abbreviated (BACKLOG #62's defect class), so a screen's
// own registered name is the only place the label can come from.
func helpTitleSegs(th theme.Theme, covered ScreenEntry) []rowSeg {
	return []rowSeg{{text: "Help " + th.Glyphs().Separator + " " + covered.Name, role: theme.RoleFg, bold: true}}
}

// helpKeyColWidth returns the widest Help().Key text among bindings:
// the column every row's own description aligns to within its own
// section, Global and This-screen each aligning independently since
// their own key sets differ.
func helpKeyColWidth(bindings []key.Binding) int {
	width := 0
	for _, b := range bindings {
		width = max(width, ansi.StringWidth(b.Help().Key))
	}
	return width
}

// helpKeyRowSegs returns one binding's own row: its key in RoleFg,
// padded to keyColWidth plus GapControl, then its description in
// RoleFgMuted (the same key/desc atom hintSegs renders for the
// footer, realigned to a shared column since the overlay lists many
// rows at once rather than one hint among neighbors).
func helpKeyRowSegs(b key.Binding, keyColWidth int) []rowSeg {
	k := b.Help().Key
	pad := keyColWidth - ansi.StringWidth(k) + theme.GapControl
	return []rowSeg{
		{text: k, role: theme.RoleFg},
		{text: strings.Repeat(" ", pad), role: theme.RoleFg},
		{text: b.Help().Desc, role: theme.RoleFgMuted},
	}
}

// helpSectionLines returns one section's own lines: a bold header row
// named title, then one row per binding.
func helpSectionLines(title string, bindings []key.Binding) [][]rowSeg {
	lines := [][]rowSeg{{{text: title, role: theme.RoleFg, bold: true}}}
	keyColWidth := helpKeyColWidth(bindings)
	for _, b := range bindings {
		lines = append(lines, helpKeyRowSegs(b, keyColWidth))
	}
	return lines
}

// helpLinesOneColumn composes h's own body for the spartan rung
// (wireframe F5): title, then the Global section, then the
// This-screen section, each stacked in reading order.
func helpLinesOneColumn(h HelpScreen) [][]rowSeg {
	lines := [][]rowSeg{helpTitleSegs(h.theme, h.Covered), helpBlankLine}
	lines = append(lines, helpSectionLines("Global", helpGlobalKeys)...)
	lines = append(lines, helpBlankLine)
	lines = append(lines, helpSectionLines("This screen", helpContent(h.Covered))...)
	return lines
}

// helpLinesTwoColumn composes h's own body for the standard rung and
// up (wireframe F6): title, then the Global and This-screen sections
// side by side, the shorter section padded with blank rows so both
// columns share the same row count.
func helpLinesTwoColumn(h HelpScreen) [][]rowSeg {
	global := helpSectionLines("Global", helpGlobalKeys)
	screen := helpSectionLines("This screen", helpContent(h.Covered))
	colWidth := (h.layout.Main.Rect.Dx() - 2*theme.PadBand - theme.GapPane) / 2

	rows := max(len(global), len(screen))
	lines := [][]rowSeg{helpTitleSegs(h.theme, h.Covered), helpBlankLine}
	for i := range rows {
		lines = append(lines, joinHelpColumns(helpLineAt(global, i), helpLineAt(screen, i), colWidth))
	}
	return lines
}

// helpLineAt returns lines[i], or helpBlankLine once i runs past its
// end: the shorter section's own padding within the two-column body.
func helpLineAt(lines [][]rowSeg, i int) []rowSeg {
	if i < len(lines) {
		return lines[i]
	}
	return helpBlankLine
}

// joinHelpColumns returns one two-column row: left padded out to
// colWidth plus a GapPane gutter, then right.
func joinHelpColumns(left, right []rowSeg, colWidth int) []rowSeg {
	pad := max(0, colWidth-ansi.StringWidth(segsPlainText(left))) + theme.GapPane
	out := append(append([]rowSeg{}, left...), rowSeg{text: strings.Repeat(" ", pad), role: theme.RoleFg})
	return append(out, right...)
}

// renderHelpLine renders segs as one exactly-width-cells row, inset
// PadBand from the pane's own left edge (a nil segs, past the end of
// h's own body, renders as a blank inset row).
func renderHelpLine(th theme.Theme, segs []rowSeg, width int) string {
	full := append([]rowSeg{{text: strings.Repeat(" ", theme.PadBand), role: theme.RoleFg}}, segs...)

	var out strings.Builder
	writeSegs(&out, th, full)
	if pad := width - ansi.StringWidth(segsPlainText(full)); pad > 0 {
		out.WriteString(th.Style(theme.RoleFg, chromeGround).Render(strings.Repeat(" ", pad)))
	}
	return out.String()
}

// View implements Screen: h's own body, clamped to its current scroll
// position and painted across the whole content pane (decision 11:
// the panel ground elevates the help overlay, the same chromeGround
// the status line and footer already paint against).
func (h HelpScreen) View() tea.View {
	rect := h.layout.Main.Rect
	width, height := rect.Dx(), rect.Dy()

	lines := h.lines()
	scroll := clampScroll(h.scroll, len(lines), height)

	rows := make([]string, height)
	for i := range rows {
		var segs []rowSeg
		if idx := scroll + i; idx < len(lines) {
			segs = lines[idx]
		}
		rows[i] = renderHelpLine(h.theme, segs, width)
	}
	return tea.NewView(strings.Join(rows, "\n"))
}

// helpKeys is HelpScreen's own operable keymap while it is showing:
// the navigation family plus Esc (UX-5's own carried ruling). Esc's
// help-context reading, "close," is the grammar's own Back verb
// (GrammarKeys.Back), not a new one, so it is bound and rendered
// exactly as Back rather than under a diverging description.
type helpKeys struct {
	Navigate, Page, Extremes, Back key.Binding
}

func (k helpKeys) ShortHelp() []key.Binding { return []key.Binding{k.Navigate, k.Page, k.Back} }
func (k helpKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Navigate, k.Page, k.Extremes, k.Back}}
}

// Entry implements Screen.
func (HelpScreen) Entry() ScreenEntry { return helpScreenEntry() }

func helpScreenEntry() ScreenEntry {
	return ScreenEntry{
		Name: helpScreenName,
		Keys: helpKeys{
			Navigate: GrammarKeys.Navigate,
			Page:     GrammarKeys.Page,
			Extremes: GrammarKeys.Extremes,
			Back:     GrammarKeys.Back,
		},
		Pointer:        []PointerBinding{{Target: PointerWheel, Key: GrammarKeys.Navigate}},
		SwitchState:    StateDigitsSwitch,
		FooterPriority: []key.Binding{GrammarKeys.Navigate, GrammarKeys.Page, GrammarKeys.Back},
	}
}

func init() {
	Register[HelpScreen](helpScreenEntry())
}
