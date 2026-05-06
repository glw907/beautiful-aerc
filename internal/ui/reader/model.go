// SPDX-License-Identifier: MIT

package reader

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/humanize"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// Phase tracks whether the viewer is fetching the body or rendering
// it. The closed state is encoded by the open flag, not a phase, so
// phase transitions only run when the viewer is open.
type Phase int

const (
	PhaseLoading Phase = iota
	PhaseReady
)

// viewerKeys are handled by Model.handleKey. Body scrolling
// (j/k/space/b) is delegated to the embedded viewport's own KeyMap;
// only the keys Model consumes directly appear here. Links is a
// fixed-size array indexed by harvested-link position minus one
// (Links[0] is "1", Links[8] is "9").
type viewerKeys struct {
	Close            key.Binding
	OpenPicker       key.Binding
	OpenAttachPicker key.Binding
	BodyTop          key.Binding
	BodyBottom       key.Binding
	Links            [9]key.Binding
}

func newViewerKeys() viewerKeys {
	vk := viewerKeys{
		Close:            key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "close")),
		OpenPicker:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "links")),
		OpenAttachPicker: key.NewBinding(key.WithKeys("@"), key.WithHelp("@", "attachments")),
		BodyTop:          key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top of body")),
		BodyBottom:       key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom of body")),
	}
	for i := range vk.Links {
		d := string(rune('1' + i))
		vk.Links[i] = key.NewBinding(key.WithKeys(d), key.WithHelp(d, "link "+d))
	}
	return vk
}

// Model renders a single message in the right panel. It owns no
// backend reference. Body fetch and mark-read Cmds are constructed
// at the AccountTab level. The viewer is pure state + render, with
// scroll position tracked by an embedded bubbles/viewport.
type Model struct {
	open         bool
	phase        Phase
	msg          mail.MessageInfo
	accountEmail string
	blocks       []content.Block
	links        []string
	attachments  []mail.Attachment
	chipRow      string
	chipHeight   int
	icons        uicore.IconSet
	panel        string // headers rendered through ViewerHeader at v.width
	viewport     viewport.Model
	spinner      spinner.Model
	styles       Styles
	theme        *theme.CompiledTheme
	keys         viewerKeys
	width        int
	height       int
}

// New constructs an empty (closed) viewer. accountEmail populates the
// To: header in the rendered message view.
func New(styles Styles, t *theme.CompiledTheme, accountEmail string, icons uicore.IconSet) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(t.FgDim)
	return Model{
		styles:       styles,
		theme:        t,
		accountEmail: accountEmail,
		icons:        icons,
		spinner:      sp,
		keys:         newViewerKeys(),
	}
}

func (v Model) IsOpen() bool { return v.open }

// Phase reports the viewer's current load phase. Used by AccountTab
// to gate n/N during loading so a second fetch isn't queued.
func (v Model) Phase() Phase { return v.phase }

// CurrentUID returns the UID of the message in the viewer, or empty
// when closed. Used by AccountTab to drop stale BodyLoadedMsg events.
func (v Model) CurrentUID() mail.UID {
	if !v.open {
		return ""
	}
	return v.msg.UID
}

// Open transitions the viewer into the loading phase for msg. The
// caller fires the body-fetch Cmd in the same Update batch.
func (v Model) Open(msg mail.MessageInfo) Model {
	v.open = true
	v.phase = PhaseLoading
	v.msg = msg
	v.blocks = nil
	v.links = nil
	v.attachments = nil
	v.chipRow = ""
	v.chipHeight = 0
	v.panel = ""
	return v
}

// Close transitions the viewer out of view.
func (v Model) Close() Model {
	v.open = false
	v.phase = PhaseLoading
	return v
}

// SetBody installs parsed blocks and transitions to ready. Idempotent
// for stale UIDs. Callers should drop BodyLoadedMsg with a UID
// mismatch before invoking this.
func (v Model) SetBody(blocks []content.Block) Model {
	v.blocks = blocks
	v.phase = PhaseReady
	v.layout()
	return v
}

// SetSize updates dimensions. When ready, re-renders headers + body
// at the new width and recomputes the viewport height.
func (v Model) SetSize(width, height int) Model {
	v.width = width
	v.height = height
	if v.phase == PhaseReady && v.open {
		v.layout()
	}
	return v
}

// SpinnerTick returns the spinner's initial tick Cmd. Caller batches
// it with the body-fetch Cmd when opening.
func (v Model) SpinnerTick() tea.Cmd { return v.spinner.Tick }

func (v Model) Links() []string { return v.links }

// SetAttachments installs the attachment metadata list. Idempotent
// for stale UIDs. Caller drops stale messages before invoking.
func (v Model) SetAttachments(items []mail.Attachment) Model {
	v.attachments = items
	if v.phase == PhaseReady && v.open {
		v.layout()
	}
	return v
}

func (v Model) Attachments() []mail.Attachment { return v.attachments }

func (v Model) ScrollPct() int {
	if v.phase != PhaseReady {
		return 0
	}
	return int(v.viewport.ScrollPercent() * 100)
}

// Update handles spinner ticks and key events while open. Returns the
// updated viewer + any Cmds (link launch, viewer-closed signal,
// scroll-position broadcast). Caller is responsible for batching.
func (v Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !v.open {
		return v, nil
	}
	switch m := msg.(type) {
	case spinner.TickMsg:
		if v.phase == PhaseLoading {
			var c tea.Cmd
			v.spinner, c = v.spinner.Update(m)
			return v, c
		}
		return v, nil
	case tea.KeyMsg:
		return v.handleKey(m)
	}
	return v, nil
}

// handleKey runs the viewer's key dispatch. q/esc closes; 1-9 launch
// links. Tab emits OpenLinkPickerMsg for App. All other keys forward
// to the viewport, which is configured with a modifier-free keymap
// (j/k/space/b/g/G).
func (v Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, v.keys.Close):
		v = v.Close()
		return v, nil
	case key.Matches(msg, v.keys.OpenAttachPicker):
		if len(v.attachments) == 0 {
			return v, nil
		}
		uid := v.msg.UID
		items := append([]mail.Attachment(nil), v.attachments...)
		return v, func() tea.Msg { return OpenAttachPickerMsg{UID: uid, Items: items} }
	case key.Matches(msg, v.keys.OpenPicker):
		if len(v.links) == 0 {
			return v, nil
		}
		links := append([]string(nil), v.links...)
		return v, func() tea.Msg { return OpenLinkPickerMsg{Links: links} }
	}
	for i, b := range v.keys.Links {
		if key.Matches(msg, b) {
			if i < len(v.links) {
				url := v.links[i]
				return v, func() tea.Msg { return LaunchURLMsg{URL: url} }
			}
			return v, nil
		}
	}
	if v.phase != PhaseReady {
		return v, nil
	}
	switch {
	case key.Matches(msg, v.keys.BodyTop):
		v.viewport.GotoTop()
	case key.Matches(msg, v.keys.BodyBottom):
		v.viewport.GotoBottom()
	default:
		var c tea.Cmd
		v.viewport, c = v.viewport.Update(msg)
		return v, c
	}
	return v, nil
}

// View renders the viewer in its current phase. Returns "" when
// closed so AccountTab.View can fall through to the message list.
//
// The output is hard-clipped to v.width so the viewer cannot lie to
// its parent's JoinHorizontal. Content longer than v.width (e.g. a
// raw URL the body renderer's hardwrap missed) gets truncated rather
// than overflowing into the sidebar column. This is the bubbles-
// component idiom: each component owns its size contract.
func (v Model) View() string {
	if !v.open {
		return ""
	}
	bg := v.styles.ViewerBg
	if v.phase == PhaseLoading {
		text := v.spinner.View() + " Loading message…"
		placed := lipgloss.Place(
			v.width, v.height,
			lipgloss.Center, lipgloss.Center,
			v.styles.Dim.Render(text),
		)
		return clipPaneBg(placed, v.width, v.height, bg)
	}

	// Two stacked rectangles: BgElevated panel (with FgDim BorderBottom
	// drawn natively by lipgloss) above a BgBase body. clipPaneBg
	// per-line pads the body to v.width with bg-styled spaces, using
	// lipgloss's outer Width.Render directly leaves terminal-default
	// gaps in the right pad whenever the inner content ends a line
	// with `\x1b[0m` (lipgloss issue #209).
	// viewport.View() right-pads each line to its width with plain
	// (unstyled) spaces. We strip that pad before bg-padding ourselves.
	// otherwise FillRowToWidth sees the line at width and skips, leaving
	// the right side on terminal-default bg.
	leftPad := bg.Render(" ")
	bodyHeight := max(0, v.height-lipgloss.Height(v.panel)-v.chipHeight)
	bodyLines := strings.Split(v.viewport.View(), "\n")
	if len(bodyLines) > bodyHeight {
		bodyLines = bodyLines[:bodyHeight]
	}
	for i, l := range bodyLines {
		bodyLines[i] = uicore.FillRowToWidth(leftPad+strings.TrimRight(l, " "), v.width, bg)
	}
	if len(bodyLines) < bodyHeight {
		blank := bg.Render(strings.Repeat(" ", v.width))
		for len(bodyLines) < bodyHeight {
			bodyLines = append(bodyLines, blank)
		}
	}
	parts := []string{v.panel}
	if v.chipRow != "" {
		parts = append(parts, v.chipRow)
	}
	parts = append(parts, strings.Join(bodyLines, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// clipPaneBg fits s to exactly width × height. Each content line is
// padded to width with bgStyle. Missing rows get a bg-styled blank.
// Surface-baked leaves (theme styles carry their pane's bg) make the
// per-line right-pad sufficient. No SGR rewriting needed.
func clipPaneBg(s string, width, height int, bg lipgloss.Style) string {
	if width < 1 || height < 1 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = uicore.FillRowToWidth(line, width, bg)
	}
	blank := bg.Render(strings.Repeat(" ", width))
	for len(lines) < height {
		lines = append(lines, blank)
	}
	return strings.Join(lines, "\n")
}

// renderChipRow returns the wrapped chip block plus its row count,
// rendered at width. Returns ("", 0) when there are no attachments.
func (v Model) renderChipRow(width int) (string, int) {
	if len(v.attachments) == 0 || width < 1 {
		return "", 0
	}
	icon := v.icons.Attachment
	bg := v.styles.ViewerBg
	chips := make([]string, len(v.attachments))
	for i, a := range v.attachments {
		name := a.Filename
		if name == "" {
			name = "attachment"
		}
		chips[i] = fmt.Sprintf("%s %d. %s (%s)", icon, i+1, name, humanize.Bytes(int64(a.Size)))
	}
	var lines []string
	var cur string
	for _, c := range chips {
		if uicore.DisplayCells(c) > width {
			c = uicore.DisplayTruncate(c, width)
		}
		if cur == "" {
			cur = c
			continue
		}
		if uicore.DisplayCells(cur)+2+uicore.DisplayCells(c) > width {
			lines = append(lines, cur)
			cur = c
			continue
		}
		cur = cur + "  " + c
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	for i, l := range lines {
		lines[i] = uicore.FillRowToWidth(l, width, bg)
	}
	return strings.Join(lines, "\n"), len(lines)
}

// layout renders headers + body and populates the viewport. Called
// from SetBody and from SetSize when the viewer is already ready.
// Headers stay pinned above the viewport. Only the body scrolls.
//
// contentWidth is v.width - 1 to account for the panel's PaddingLeft.
// The body region fills the rest of v.height beneath the rendered
// panel (which includes its bottom border row).
func (v *Model) layout() {
	hdrs := content.ParsedHeaders{
		From:    []content.Address{{Name: v.msg.From}},
		To:      addressesFor(v.msg.To, v.accountEmail),
		Cc:      namesAsAddresses(v.msg.Cc),
		Bcc:     namesAsAddresses(v.msg.Bcc),
		Date:    viewerDateString(v.msg),
		Subject: v.msg.Subject,
	}
	contentWidth := max(1, v.width-1)
	headerStr := content.RenderHeaders(hdrs, v.theme, contentWidth)
	v.panel = v.styles.ViewerHeader.Width(v.width).Render(headerStr)
	v.chipRow, v.chipHeight = v.renderChipRow(v.width)
	body, urls := content.RenderBodyWithFootnotes(v.blocks, v.theme, contentWidth)
	v.links = urls
	bodyHeight := max(1, v.height-lipgloss.Height(v.panel)-v.chipHeight)
	vp := viewport.New(contentWidth, bodyHeight)
	// Modifier-free viewport bindings: j/k for line nav, space/b for
	// page nav. g/G are handled by the viewer wrapper itself.
	vp.KeyMap = viewport.KeyMap{
		Up:       key.NewBinding(key.WithKeys("k", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down")),
		PageDown: key.NewBinding(key.WithKeys(" ")),
		PageUp:   key.NewBinding(key.WithKeys("b")),
	}
	vp.SetContent(body)
	v.viewport = vp
}

// addressesFor returns the To: list to render in the viewer. Real
// recipient strings from the wire take precedence. Otherwise fall
// back to the user's own address (so the To: row is never empty).
func addressesFor(to, fallbackEmail string) []content.Address {
	if to != "" {
		return namesAsAddresses(to)
	}
	if fallbackEmail == "" {
		return nil
	}
	return []content.Address{{Email: fallbackEmail}}
}

// namesAsAddresses splits a flat "Name1, Name2, ..." MessageInfo
// string into Address values for the header renderer. The split is
// intentionally naive (commas inside quoted phrases are uncommon in
// already-formatted display strings). Refine if it bites.
func namesAsAddresses(s string) []content.Address {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ", ")
	out := make([]content.Address, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, content.Address{Name: p})
		}
	}
	return out
}

// viewerDateString returns the date string for the viewer's Date row.
// Prefers msg.Date when populated, otherwise formats msg.SentAt as a
// full unambiguous timestamp ("Mon, Jan 2 2006 3:04 PM"). Returns ""
// when neither is set so the row is omitted.
func viewerDateString(msg mail.MessageInfo) string {
	if msg.Date != "" {
		return msg.Date
	}
	if msg.SentAt.IsZero() {
		return ""
	}
	return msg.SentAt.Format("Mon, Jan 2 2006 3:04 PM")
}
