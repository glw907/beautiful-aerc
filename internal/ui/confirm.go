package ui

import (
	"image"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

// confirmRows is the modal's fixed row count (composition rule 5:
// "natural sizes, not slopes"): top edge, a blank inset row, the
// question, the consequence, a blank row, the y/n answer row, a
// blank row, and the bottom edge.
const confirmRows = 8

// Confirm is poplar's generic modal-confirm component (UX-9's
// vocabulary, "one question, named consequence"): Question is the
// question itself, Consequence names what each answer does,
// YesLabel/NoLabel are the y/n hint text ("quit"/"stay"), and
// YesCmd/NoCmd are what each answer dispatches once App pops it off
// the stack. n (No) is the ruled default, rendered on the selectedBg
// pill (the ratified shell exemplar). Confirm implements Screen, so
// App's screen stack (task 5a's ruling) renders it directly: the
// plain stack-top render that ruling settled, full terminal, clamped
// and centered per composition rule 5, with no further chrome.
type Confirm struct {
	theme  theme.Theme
	layout LayoutMode

	Question    string
	Consequence string
	YesLabel    string
	NoLabel     string
	YesCmd      tea.Cmd
	NoCmd       tea.Cmd
}

// Init implements Screen.
func (c Confirm) Init() tea.Cmd { return nil }

// ConfirmAnsweredMsg is Confirm's answer signal (elm-conventions
// rule 4, children signal parents via Msg types; task-8-findings-r1.md
// conventions ruling): Next is the Cmd the y/n/Esc key named. App's
// Update catches it, pops the stack, and runs Next: the template
// every future modal follows, rather than a concrete type assertion
// in handleKey.
type ConfirmAnsweredMsg struct {
	Next tea.Cmd
}

// Update implements Screen: LayoutMsg and ThemeMsg, the two every
// screen carries, plus Confirm's y/n/Esc answer
// (GrammarExemptModalConfirm: y is not yank here). A key that answers
// emits ConfirmAnsweredMsg for App to catch; every other key,
// including digits, is a no-op (UX-4's modal no-op rule), which
// App.handleKey guarantees reaches here at all only once its
// Back/digit/quit precedence has found nothing else to do with it.
func (c Confirm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LayoutMsg:
		c.layout = msg.Layout
	case ThemeMsg:
		c.theme = msg.Theme
	case tea.KeyPressMsg:
		if next, answered := c.answer(msg); answered {
			return c, func() tea.Msg { return ConfirmAnsweredMsg{Next: next} }
		}
	}
	return c, nil
}

// answer reports the Cmd msg names under Confirm's y/n/Esc
// grammar and whether msg named an answer at all. Every other key,
// digits included, names no answer.
func (c Confirm) answer(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch {
	case key.Matches(msg, c.yesBinding()):
		return c.YesCmd, true
	case key.Matches(msg, c.noBinding()), key.Matches(msg, GrammarKeys.Back):
		return c.NoCmd, true
	default:
		return nil, false
	}
}

func (c Confirm) yesBinding() key.Binding {
	return key.NewBinding(key.WithKeys("y"), key.WithHelp("y", c.YesLabel))
}

func (c Confirm) noBinding() key.Binding {
	return key.NewBinding(key.WithKeys("n"), key.WithHelp("n", c.NoLabel))
}

// confirmKeys is Confirm's per-instance keymap.
type confirmKeys struct {
	Yes, No key.Binding
}

func (k confirmKeys) ShortHelp() []key.Binding  { return []key.Binding{k.Yes, k.No} }
func (k confirmKeys) FullHelp() [][]key.Binding { return [][]key.Binding{{k.Yes, k.No}} }

// Entry implements Screen.
func (c Confirm) Entry() ScreenEntry {
	return ScreenEntry{
		Name:          "modal confirm",
		Keys:          confirmKeys{Yes: c.yesBinding(), No: c.noBinding()},
		SwitchState:   StateModal,
		GrammarExempt: GrammarExemptModalConfirm,
	}
}

// confirmEntry is the type-level registration Register[Confirm]
// carries (registry.go): generic yes/no labels stand in for whatever
// question a live instance actually asks, since Register runs once at
// init with no instance to read YesLabel/NoLabel from. The registry's
// mechanical checks (grammar exemption, pointer-target legality, valid
// switch state) hold regardless of the label text a live Entry()
// carries.
func confirmEntry() ScreenEntry {
	yes := key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes"))
	no := key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "no"))
	return ScreenEntry{
		Name:          "modal confirm",
		Keys:          confirmKeys{Yes: yes, No: no},
		Pointer:       []PointerBinding{{Target: PointerModalAnswer, Key: yes}, {Target: PointerModalAnswer, Key: no}},
		SwitchState:   StateModal,
		GrammarExempt: GrammarExemptModalConfirm,
	}
}

func init() {
	Register[Confirm](confirmEntry())
}

// confirmGeometry is Confirm's natural-size, clamped-and-centered
// box geometry (composition rule 5): a boxWidth×confirmRows box at
// (x, y) within the terminal c.layout names. View and ConfirmHitSpans
// both derive from geometry, so a hit span never depends on having
// rendered first (StatusLineHitSpans's precedent).
type confirmGeometry struct {
	x, y, width int
}

func (c Confirm) geometry() confirmGeometry {
	width := confirmBoxWidth(c, c.layout.Width)
	return confirmGeometry{
		x:     (c.layout.Width - width) / 2,
		y:     (c.layout.Height - confirmRows) / 2,
		width: width,
	}
}

// confirmPillInset is the default answer's selectedBg pill inset
// (task-8-findings-r1.md F6): two cells before the key, two after the
// label, named once rather than a raw "  " literal repeated at each
// of its two call sites.
const confirmPillInset = 2

// confirmAnswerText is the answer row's plain-text pieces: yes
// ("y quit") and no ("  n stay  ", the selectedBg pill's
// confirmPillInset padding included), used both to size the box and
// to compose the row.
func confirmAnswerText(c Confirm) (yes, no string) {
	inset := strings.Repeat(" ", confirmPillInset)
	return "y " + c.YesLabel, inset + "n " + c.NoLabel + inset
}

// confirmFloorWidth is confirmBoxWidth's defensive floor
// (correctness m7): App.View's own floor bypass, ahead of the modal
// branch, already keeps a modal from ever rendering below the width
// floor, so this never actually engages in the running product; it
// exists only so a future caller that reaches Confirm.View() with a
// termWidth this narrow cannot panic strings.Repeat(mid, width-2) at
// a negative count. 4 cells is two border columns plus a two-cell
// interior, the smallest width renderConfirmBox's row builders never
// go negative against.
const confirmFloorWidth = 4

// confirmBoxWidth returns the modal's natural width, clamped
// against termWidth (composition rule 5): PadModalX insets both sides
// of whichever is widest among the question, the consequence, and the
// answer row, plus the two border columns.
func confirmBoxWidth(c Confirm, termWidth int) int {
	yes, no := confirmAnswerText(c)
	answerWidth := ansi.StringWidth(yes) + theme.GapControl + ansi.StringWidth(no)
	content := max(ansi.StringWidth(c.Question), ansi.StringWidth(c.Consequence), answerWidth)
	natural := content + 2*theme.PadModalX + 2
	return max(min(natural, termWidth), confirmFloorWidth)
}

// View implements Screen: the whole terminal, base ground, with the
// modal box centered on it (the 5a ruling's plain stack-top render;
// no dimmed backdrop this pass).
func (c Confirm) View() tea.View {
	width, height := c.layout.Width, c.layout.Height
	g := c.geometry()

	canvas := theme.NewCanvas(width, height)
	canvas.Paint(image.Rect(0, 0, width, height), c.theme.Blank(theme.GroundBase, width, height))
	canvas.Paint(image.Rect(g.x, g.y, g.x+g.width, g.y+confirmRows), renderConfirmBox(c, g.width))
	return tea.NewView(canvas.Render())
}

// renderConfirmBox renders Confirm's box, exactly width columns
// by confirmRows rows, joined by newlines.
func renderConfirmBox(c Confirm, width int) string {
	th := c.theme
	b := th.Border(theme.BorderModal)

	rows := []string{
		confirmEdge(th, b.TopLeft, b.Top, b.TopRight, width),
		confirmBlankRow(th, b.Left, b.Right, width),
		confirmTextRow(th, b.Left, b.Right, width, c.Question, theme.RoleFg, true),
		confirmTextRow(th, b.Left, b.Right, width, c.Consequence, theme.RoleFgMuted, false),
		confirmBlankRow(th, b.Left, b.Right, width),
		confirmAnswerRow(c, b.Left, b.Right, width),
		confirmBlankRow(th, b.Left, b.Right, width),
		confirmEdge(th, b.BottomLeft, b.Bottom, b.BottomRight, width),
	}
	return strings.Join(rows, "\n")
}

func confirmEdge(th theme.Theme, left, mid, right string, width int) string {
	line := left + strings.Repeat(mid, width-2) + right
	return th.Style(theme.RoleBorder, chromeGround).Render(line)
}

func confirmBlankRow(th theme.Theme, left, right string, width int) string {
	var out strings.Builder
	out.WriteString(th.Style(theme.RoleBorder, chromeGround).Render(left))
	writeSegs(&out, th, []rowSeg{{text: strings.Repeat(" ", width-2), role: theme.RoleFg}})
	out.WriteString(th.Style(theme.RoleBorder, chromeGround).Render(right))
	return out.String()
}

// confirmTextRow renders one PadModalX-inset text line (the question
// or the consequence), padded to width-2 interior cells.
func confirmTextRow(th theme.Theme, left, right string, width int, text string, role theme.Role, bold bool) string {
	interior := width - 2
	segs := []rowSeg{
		{text: strings.Repeat(" ", theme.PadModalX), role: theme.RoleFg},
		{text: text, role: role, bold: bold},
	}
	if pad := interior - theme.PadModalX - ansi.StringWidth(text); pad > 0 {
		segs = append(segs, rowSeg{text: strings.Repeat(" ", pad), role: theme.RoleFg})
	}
	var out strings.Builder
	out.WriteString(th.Style(theme.RoleBorder, chromeGround).Render(left))
	writeSegs(&out, th, segs)
	out.WriteString(th.Style(theme.RoleBorder, chromeGround).Render(right))
	return out.String()
}

// confirmAnswerOffsets returns the column, relative to the box's
// left edge (column 0 is the left border), of the y and n answer
// keys' characters within a boxWidth-wide answer row.
// confirmAnswerRow and ConfirmHitSpans both derive from it, so a hit
// span never depends on having rendered first.
func confirmAnswerOffsets(c Confirm, boxWidth int) (yesX, noX int) {
	yes, no := confirmAnswerText(c)
	interior := boxWidth - 2
	contentWidth := ansi.StringWidth(yes) + theme.GapControl + ansi.StringWidth(no)
	leftPad := max(0, (interior-contentWidth)/2)
	yesX = 1 + leftPad
	noX = yesX + ansi.StringWidth(yes) + theme.GapControl + confirmPillInset // the pill's inset precedes "n"
	return yesX, noX
}

// confirmAnswerRow renders the y/n answer row: the default answer (n)
// on its selectedBg pill (decision 6, the ratified exemplar),
// centered within the box.
func confirmAnswerRow(c Confirm, left, right string, width int) string {
	th := c.theme
	interior := width - 2
	yes, no := confirmAnswerText(c)
	contentWidth := ansi.StringWidth(yes) + theme.GapControl + ansi.StringWidth(no)
	leftPad := max(0, (interior-contentWidth)/2)
	rightPad := max(0, interior-contentWidth-leftPad)

	inset := strings.Repeat(" ", confirmPillInset)
	content := th.Style(theme.RoleFg, chromeGround).Render("y") +
		th.Style(theme.RoleFgMuted, chromeGround).Render(" "+c.YesLabel) +
		th.Style(theme.RoleFg, chromeGround).Render(strings.Repeat(" ", theme.GapControl)) +
		th.Style(theme.RoleFg, theme.GroundSelected).Render(inset+"n") +
		th.Style(theme.RoleFgMuted, theme.GroundSelected).Render(" "+c.NoLabel+inset)

	var out strings.Builder
	out.WriteString(th.Style(theme.RoleBorder, chromeGround).Render(left))
	out.WriteString(th.Style(theme.RoleFg, chromeGround).Render(strings.Repeat(" ", leftPad)))
	out.WriteString(content)
	out.WriteString(th.Style(theme.RoleFg, chromeGround).Render(strings.Repeat(" ", rightPad)))
	out.WriteString(th.Style(theme.RoleBorder, chromeGround).Render(right))
	return out.String()
}

// ConfirmHitSpans returns the y and n answer cells' HitSpans
// (ADR-0017's PointerModalAnswer, legal only at StateModal): each
// span's Rect is the exact column its key character occupies
// within the modal's answer row.
func ConfirmHitSpans(c Confirm) []HitSpan {
	g := c.geometry()
	yesX, noX := confirmAnswerOffsets(c, g.width)
	rowY := g.y + 5 // the answer row is index 5 of confirmRows, 0-based

	return []HitSpan{
		{Target: PointerModalAnswer, Verb: c.yesBinding(), Rect: image.Rect(g.x+yesX, rowY, g.x+yesX+1, rowY+1)},
		{Target: PointerModalAnswer, Verb: c.noBinding(), Rect: image.Rect(g.x+noX, rowY, g.x+noX+1, rowY+1)},
	}
}

// HitSpans implements the inline interface mouse.go's dispatchClick
// resolves a StateModal stack top through (task-10-findings-r2.md's
// F5 ruling: an anonymous interface{ HitSpans() []HitSpan }, never a
// named single-impl interface or a concrete Confirm type assertion).
func (c Confirm) HitSpans() []HitSpan { return ConfirmHitSpans(c) }
