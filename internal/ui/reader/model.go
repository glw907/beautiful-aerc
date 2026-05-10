package reader

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/humanize"
	"github.com/glw907/poplar/internal/icalendar"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// Phase tracks whether the viewer is fetching the body or rendering it.
// Closed is encoded by the open flag instead of a phase, so phase
// transitions only run while the viewer is open.
type Phase int

const (
	PhaseLoading Phase = iota
	PhaseReady
)

// viewerKeys covers the keys Model consumes directly. Body scrolling
// (j/k/space/b) delegates to the viewport's own KeyMap. Links is indexed
// by harvested-link position minus one (Links[0] is "1", Links[8] is "9").
type viewerKeys struct {
	Close            key.Binding
	OpenPicker       key.Binding
	OpenAttachPicker key.Binding
	BodyTop          key.Binding
	BodyBottom       key.Binding
	Unsubscribe      key.Binding
	Links            [9]key.Binding
}

func newViewerKeys() viewerKeys {
	vk := viewerKeys{
		Close:            key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "close")),
		OpenPicker:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "links")),
		OpenAttachPicker: key.NewBinding(key.WithKeys("@"), key.WithHelp("@", "attachments")),
		BodyTop:          key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top of body")),
		BodyBottom:       key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom of body")),
		Unsubscribe:      key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "unsubscribe")),
	}
	for i := range vk.Links {
		d := string(rune('1' + i))
		vk.Links[i] = key.NewBinding(key.WithKeys(d), key.WithHelp(d, "link "+d))
	}
	return vk
}

// Model renders one message in the right panel. It holds no backend
// reference; body fetch and mark-read Cmds are built at the AccountTab
// level. Pure state + render, with scroll tracked by the embedded
// bubbles/viewport.
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
	invite       *icalendar.Invite
	inviteRow    string
	inviteHeight int
	unsub        content.Unsubscribe
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
// rendered "To" header.
func New(styles Styles, t *theme.CompiledTheme, accountEmail string, icons uicore.IconSet) Model {
	sp := uicore.NewSpinner(t)
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
func (v Model) Phase() Phase { return v.phase }

// CurrentUID returns the UID in the viewer, or "" when closed. AccountTab
// uses it to drop stale BodyLoadedMsg events.
func (v Model) CurrentUID() mail.UID {
	if !v.open {
		return ""
	}
	return v.msg.UID
}

// Open transitions the viewer into the loading phase for msg. The caller
// fires the body-fetch Cmd in the same Update batch.
func (v Model) Open(msg mail.MessageInfo) Model {
	v.open = true
	v.phase = PhaseLoading
	v.msg = msg
	v.blocks = nil
	v.links = nil
	v.attachments = nil
	v.chipRow = ""
	v.chipHeight = 0
	v.invite = nil
	v.inviteRow = ""
	v.inviteHeight = 0
	v.panel = ""
	v.unsub = content.Unsubscribe{}
	return v
}

func (v Model) Close() Model {
	v.open = false
	v.phase = PhaseLoading
	return v
}

// SetBody installs parsed blocks, unsubscribe data, and invite (nil when
// absent), then transitions to ready. Callers must drop BodyLoadedMsg
// with a UID mismatch before invoking.
func (v Model) SetBody(blocks []content.Block, unsub content.Unsubscribe, invite *icalendar.Invite) Model {
	v.blocks = blocks
	v.unsub = unsub
	v.invite = invite
	v.phase = PhaseReady
	v.layout()
	return v
}

// Unsubscribe returns the harvested List-Unsubscribe data for the
// current message. Zero when the viewer is closed or the message
// has no List-Unsubscribe headers.
func (v Model) Unsubscribe() content.Unsubscribe { return v.unsub }

// Invite returns the parsed calendar invite for the current message,
// or nil when absent.
func (v Model) Invite() *icalendar.Invite { return v.invite }

// SetSize updates dimensions and re-runs layout when the viewer is ready.
func (v Model) SetSize(width, height int) Model {
	v.width = width
	v.height = height
	if v.phase == PhaseReady && v.open {
		v.layout()
	}
	return v
}

// SpinnerTick returns the spinner's initial tick Cmd; the caller batches
// it with the body-fetch Cmd when opening.
func (v Model) SpinnerTick() tea.Cmd { return v.spinner.Tick }

func (v Model) Links() []string { return v.links }

// SetAttachments installs the attachment metadata list. Caller drops
// stale messages before invoking.
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

// Update handles spinner ticks and key events while open and returns
// any emitted Cmd (link launch, viewer-closed signal, scroll broadcast).
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
	case tea.KeyPressMsg:
		return v.handleKey(m)
	}
	return v, nil
}

// handleKey runs the viewer's key dispatch. q/esc closes, 1-9 launch
// links, Tab emits OpenLinkPickerMsg. Everything else falls through to
// the viewport's modifier-free keymap (j/k/space/b/g/G).
func (v Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
	case key.Matches(msg, v.keys.Unsubscribe):
		if !v.unsub.Available() {
			return v, nil
		}
		uid := v.msg.UID
		unsub := v.unsub
		return v, func() tea.Msg { return OpenUnsubscribeConfirmMsg{UID: uid, Unsub: unsub} }
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

// View renders the viewer at the current phase, returning "" when closed
// so AccountTab.View can fall through to the message list. Output is
// hard-clipped to v.width so a stray oversized line (e.g. a raw URL the
// body renderer's hardwrap missed) cannot bleed into the sidebar column.
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

	// Body is two stacked rects: BgElevated panel (FgDim BorderBottom drawn
	// by lipgloss) above a BgBase body. lipgloss's outer Width.Render
	// leaves terminal-default gaps in the right pad whenever inner content
	// ends with \x1b[0m (lipgloss #209), so per-line bg-padding is needed.
	// viewport.View() right-pads with plain unstyled spaces; strip that
	// before re-padding, otherwise FillRowToWidth sees full width and
	// skips, leaving the right edge unstyled.
	leftPad := bg.Render(" ")
	bodyHeight := max(0, v.height-lipgloss.Height(v.panel)-v.inviteHeight-v.chipHeight)
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
	if v.inviteRow != "" {
		parts = append(parts, v.inviteRow)
	}
	if v.chipRow != "" {
		parts = append(parts, v.chipRow)
	}
	parts = append(parts, strings.Join(bodyLines, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// clipPaneBg fits s to exactly width × height, padding each content line
// to width with bgStyle and filling missing rows with a bg-styled blank.
// Surface-baked theme styles already carry the pane's bg, so per-line
// right-padding is sufficient and no SGR rewriting is needed.
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

// renderChipRow returns the wrapped chip block and its row count.
// Returns ("", 0) when there are no attachments.
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
		if ansix.Width(c) > width {
			c = ansix.Truncate(c, width)
		}
		if cur == "" {
			cur = c
			continue
		}
		if ansix.Width(cur)+2+ansix.Width(c) > width {
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

// layout renders headers + body and populates the viewport. Headers stay
// pinned above the viewport, only the body scrolls. contentWidth is
// v.width - 1 to account for the panel's PaddingLeft.
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
	v.inviteRow, v.inviteHeight = renderInviteBlock(v.invite, v.icons, v.styles, v.width)
	body, urls := content.RenderBodyWithFootnotes(v.blocks, v.theme, contentWidth)
	v.links = urls
	bodyHeight := max(1, v.height-lipgloss.Height(v.panel)-v.inviteHeight-v.chipHeight)
	vp := viewport.New(viewport.WithWidth(contentWidth), viewport.WithHeight(bodyHeight))
	// Modifier-free viewport bindings. g/G are handled by the wrapper.
	vp.KeyMap = viewport.KeyMap{
		Up:       key.NewBinding(key.WithKeys("k", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down")),
		PageDown: key.NewBinding(key.WithKeys("space")),
		PageUp:   key.NewBinding(key.WithKeys("b")),
	}
	vp.SetContent(body)
	v.viewport = vp
}

// addressesFor returns the To: list to render. Real recipient strings
// from the wire take precedence. The fallback to the user's own address
// keeps the To: row from rendering empty.
func addressesFor(to, fallbackEmail string) []content.Address {
	if to != "" {
		return namesAsAddresses(to)
	}
	if fallbackEmail == "" {
		return nil
	}
	return []content.Address{{Email: fallbackEmail}}
}

// namesAsAddresses splits a flat "Name1, Name2, ..." string into Address
// values. The split is intentionally naive (commas inside quoted phrases
// are rare in already-formatted display strings). Refine if it bites.
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

// viewerDateString returns the Date-row string. msg.Date wins when
// populated, otherwise SentAt is formatted as an unambiguous timestamp
// ("Mon, Jan 2 2006 3:04 PM"). Returns "" so the row is omitted when
// neither is set.
func viewerDateString(msg mail.MessageInfo) string {
	if msg.Date != "" {
		return msg.Date
	}
	if msg.SentAt.IsZero() {
		return ""
	}
	return msg.SentAt.Format("Mon, Jan 2 2006 3:04 PM")
}
