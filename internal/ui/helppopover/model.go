package helppopover

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// Styles holds the subset of UI styles the help popover needs.
type Styles struct {
	HelpBoxBorder   lipgloss.Style
	HelpTitle       lipgloss.Style
	HelpGroupHeader lipgloss.Style
	HelpKey         lipgloss.Style
	FrameBorder     lipgloss.Style
	Dim             lipgloss.Style
}

// Context selects which binding layout the popover renders.
type Context int

const (
	Account Context = iota
	Viewer
)

// cache holds the memoised Box output and the inputs that
// determine when it must be rebuilt.
//
// Pre-beta escape hatch: Model is an immutable value type, so a
// cache that lives in the value would be lost on every SetSize/return.
// Rather than switching to pointer receivers (which would break the Elm
// immutable-model contract), we heap-allocate one small cache struct at
// construction time and access it through a pointer. The pointer itself
// is copied with the value, so every generation of the popover shares
// the same cache. A deliberate choice: they all render the same logical
// state, and the dirty flag ensures stale renders are never served.
type cache struct {
	dirty     bool
	context   Context
	w, h      int
	box       string
	tooNarrow string
}

// Model is the modal help overlay. App owns key routing;
// this model only renders.
type Model struct {
	styles  Styles
	context Context
	width   int
	height  int
	c       *cache // heap-allocated, shared across value copies
}

// New constructs a popover for the given context.
func New(styles Styles, context Context) Model {
	return Model{
		styles:  styles,
		context: context,
		c:       &cache{dirty: true},
	}
}

// SetSize updates the popover's box dimensions. App threads
// WindowSizeMsg here, mirroring the other overlay surfaces.
func (h Model) SetSize(width, height int) Model {
	h.width = width
	h.height = height
	return h
}

// bindingRow is a single key/description entry in the popover.
// Unwired rows render dim per the future-binding policy.
type bindingRow struct {
	key   string
	desc  string
	wired bool
}

// bindingGroup is a labeled cluster of bindingRow entries
// (e.g., "Navigate", "Triage").
type bindingGroup struct {
	title string
	rows  []bindingRow
}

// accountGroups is the binding map shown when the popover opens
// from the account view. Order is the visual layout order.
var accountGroups = []bindingGroup{
	{
		title: "Navigate",
		rows: []bindingRow{
			{"j/k", "up/down", true},
			{"g/G", "top/bot", true},
		},
	},
	{
		title: "Triage",
		rows: []bindingRow{
			{"d", "delete", true},
			{"a", "archive", true},
			{"s", "star", true},
			{".", "read/unrd", true},
			{"m", "move", true},
			{"E", "empty", true},
			{"u", "undo", true},
		},
	},
	{
		title: "Reply",
		rows: []bindingRow{
			{"r", "reply", true},
			{"R", "all", true},
			{"f", "forward", true},
			{"c", "compose", true},
		},
	},
	{
		title: "Search",
		rows: []bindingRow{
			{"/", "search", true},
			{"n", "next", false},
			{"N", "prev", false},
		},
	},
	{
		title: "Select",
		rows: []bindingRow{
			{"v", "select", true},
			{"␣", "toggle", true},
		},
	},
	{
		title: "Threads",
		rows: []bindingRow{
			{"␣", "fold", true},
			{"F", "fold all", true},
		},
	},
	{
		title: "Go To",
		rows: []bindingRow{
			{"I", "inbox", true},
			{"D", "drafts", true},
			{"S", "sent", true},
			{"A", "archive", true},
			{"X", "spam", true},
			{"T", "trash", true},
		},
	},
}

// accountBottomHints is the trailing line under the groups in the
// account context: "Enter open    Q outbox    ! conflicts    ? close".
var accountBottomHints = []bindingRow{
	{"Enter", "open", true},
	{"Q", "outbox", true},
	{"!", "conflicts", true},
	{"?", "close", true},
}

// viewerGroups is the binding map shown when the popover opens
// from the message viewer.
var viewerGroups = []bindingGroup{
	{
		title: "Navigate",
		rows: []bindingRow{
			{"j/k", "scroll", true},
			{"g/G", "top/bot", true},
			{"␣/b", "page d/u", true},
			{"1-9", "open link", true},
		},
	},
	{
		title: "Triage",
		rows: []bindingRow{
			{"d", "delete", true},
			{"a", "archive", true},
			{"s", "star", true},
			{"u", "undo", true},
		},
	},
	{
		title: "Reply",
		rows: []bindingRow{
			{"r", "reply", true},
			{"R", "all", true},
			{"f", "forward", true},
			{"c", "compose", true},
		},
	},
}

// viewerBottomHints is the trailing line in the viewer context:
// "Tab link picker    q  close    ?  close".
var viewerBottomHints = []bindingRow{
	{"Tab", "link picker", true},
	{"Q", "outbox", true},
	{"!", "conflicts", true},
	{"q", "close", true},
	{"?", "close", true},
}

// Box returns the popover box string sized from its content. The returned
// string does NOT include full-screen padding. It is the raw box ready
// for overlay compositing. The second return value is a "too narrow"
// fallback string, non-empty when the box does not fit within
// (width, height) and the caller should display it instead.
//
// The result is cached keyed on (context, width, height). A context or
// dimension change counts as dirty even when the flag is clear;
// NewHelpPopover sets dirty=true, so the first call always rebuilds.
func (h Model) Box(width, height int) (box string, tooNarrow string) {
	c := h.c
	if !c.dirty && c.context == h.context && c.w == width && c.h == height {
		return c.box, c.tooNarrow
	}

	var title, body string
	var bottomHints []bindingRow
	switch h.context {
	case Viewer:
		title = "Message Viewer"
		body = renderViewerLayout(h.styles, viewerGroups)
		bottomHints = viewerBottomHints
	default:
		title = "Message List"
		body = renderAccountLayout(h.styles, accountGroups)
		bottomHints = accountBottomHints
	}
	inner := body + "\n\n" + renderHintLine(h.styles, bottomHints)

	// Wrap inner in a rounded box, with top border drawn manually
	// so the title can be embedded. Style is defined in styles.go.
	b := h.styles.HelpBoxBorder.Render(inner)

	boxWidth := lipgloss.Width(b)
	titleSeg := h.styles.HelpTitle.Render(title)
	border := h.styles.FrameBorder
	prefix := border.Render("╭─ ") + titleSeg + border.Render(" ")
	pad := boxWidth - lipgloss.Width(prefix) - 1 // -1 for the closing ╮
	if pad < 0 {
		pad = 0
	}
	topEdge := prefix + border.Render(strings.Repeat("─", pad)+"╮")
	popover := topEdge + "\n" + b

	if boxWidth > width || lipgloss.Height(popover) > height {
		c.box = ""
		c.tooNarrow = h.styles.Dim.Render("Terminal too narrow for help popover")
	} else {
		c.box = popover
		c.tooNarrow = ""
	}
	c.context = h.context
	c.w = width
	c.h = height
	c.dirty = false
	return c.box, c.tooNarrow
}

// Position returns the top-left (x, y) cell coordinates at which the
// popover box should be placed to appear centered on (width, height).
func (h Model) Position(box string, width, height int) (x, y int) {
	return uicore.CenterOverlay(box, width, height)
}

// View renders the popover centered on a width × height area.
// When the underlying account frame is available the caller should use
// Box + Position + PlaceOverlay instead. View is retained as a fallback
// for callers that need a standalone full-screen string (e.g. tests).
func (h Model) View(width, height int) string {
	box, tooNarrow := h.Box(width, height)
	if tooNarrow != "" {
		return lipgloss.Place(
			width, height,
			lipgloss.Center, lipgloss.Center,
			tooNarrow,
		)
	}
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

// renderAccountLayout builds the four-section layout for the
// account context: three rows (Nav/Triage/Reply, then
// Search/Select/Threads, then Go To grid). Bottom hint line is
// added by View.
func renderAccountLayout(styles Styles, groups []bindingGroup) string {
	row1 := joinColumnsRow(renderGap(),
		renderGroup(styles, groups[0]),
		renderGroup(styles, groups[1]),
		renderGroup(styles, groups[2]),
	)
	row2 := joinColumnsRow(renderGap(),
		renderGroup(styles, groups[3]),
		renderGroup(styles, groups[4]),
		renderGroup(styles, groups[5]),
	)
	gotoBlock := renderGotoGrid(styles, groups[6])
	return strings.Join([]string{row1, "", row2, "", gotoBlock}, "\n")
}

// renderViewerLayout builds the single-row layout for the viewer
// context: Nav/Triage/Reply side-by-side.
func renderViewerLayout(styles Styles, groups []bindingGroup) string {
	return joinColumnsRow(renderGap(),
		renderGroup(styles, groups[0]),
		renderGroup(styles, groups[1]),
		renderGroup(styles, groups[2]),
	)
}

// renderGroup builds a single labeled column: heading on top,
// then key/desc rows.
func renderGroup(styles Styles, g bindingGroup) string {
	lines := []string{styles.HelpGroupHeader.Render(g.title)}
	for _, r := range g.rows {
		lines = append(lines, renderRow(styles, r))
	}
	return strings.Join(lines, "\n")
}

// joinColumnsRow concatenates pre-rendered multi-line columns
// side-by-side, padding each column to its widest line and the
// row to the tallest column. Gap is inserted between columns. Use
// instead of lipgloss.JoinHorizontal so the result is correct
// under both icon-mode cell widths (per ADR-0084).
func joinColumnsRow(gap string, cols ...string) string {
	if len(cols) == 0 {
		return ""
	}
	splits := make([][]string, len(cols))
	widths := make([]int, len(cols))
	height := 0
	for i, col := range cols {
		lines := strings.Split(col, "\n")
		splits[i] = lines
		if len(lines) > height {
			height = len(lines)
		}
		for _, line := range lines {
			if w := lipgloss.Width(line); w > widths[i] {
				widths[i] = w
			}
		}
	}
	rows := make([]string, height)
	for r := 0; r < height; r++ {
		var b strings.Builder
		for i, lines := range splits {
			var line string
			if r < len(lines) {
				line = lines[r]
			}
			b.WriteString(uicore.PadOrTruncate(line, widths[i]))
			if i < len(splits)-1 {
				b.WriteString(gap)
			}
		}
		rows[r] = b.String()
	}
	return strings.Join(rows, "\n")
}

// renderRow builds "<key>  <desc>" for a single row, padding the key
// column to a fixed width.
func renderRow(styles Styles, r bindingRow) string {
	const keyWidth = 5
	keyPadded := r.key
	for lipgloss.Width(keyPadded) < keyWidth {
		keyPadded += " "
	}
	return renderKeyDesc(styles, keyPadded, r.desc, r.wired)
}

// renderKeyDesc applies the wired-vs-unwired styling to a key+desc
// pair. Wired: bright-bold key, dim desc. Unwired: entire pair dim
// (no bold). The contrast is the future-binding signal.
func renderKeyDesc(styles Styles, key, desc string, wired bool) string {
	if wired {
		return styles.HelpKey.Render(key) + "  " + styles.Dim.Render(desc)
	}
	return styles.Dim.Render(key + "  " + desc)
}

func renderGap() string { return "    " }

// renderGotoGrid builds the Go To group as a 3×2 grid:
// "I inbox    D drafts    S sent" / "A archive  X spam  T trash".
// The group's heading is rendered above. Falls back to a flat
// column if the row count drifts from 6. Defensive against
// careless edits to the binding tables.
func renderGotoGrid(styles Styles, g bindingGroup) string {
	heading := styles.HelpGroupHeader.Render(g.title)
	if len(g.rows) != 6 {
		return renderGroup(styles, g)
	}
	gap := renderGap()
	row1 := renderRow(styles, g.rows[0]) + gap +
		renderRow(styles, g.rows[1]) + gap +
		renderRow(styles, g.rows[2])
	row2 := renderRow(styles, g.rows[3]) + gap +
		renderRow(styles, g.rows[4]) + gap +
		renderRow(styles, g.rows[5])
	return strings.Join([]string{heading, row1, row2}, "\n")
}

// renderHintLine builds the bottom hint line: "Enter  open    ?  close".
func renderHintLine(styles Styles, hints []bindingRow) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, renderKeyDesc(styles, h.key, h.desc, h.wired))
	}
	return strings.Join(parts, "    ")
}
