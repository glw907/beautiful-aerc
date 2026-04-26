package ui

import (
	"os/exec"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// viewerPhase tracks whether the viewer is fetching the body or
// rendering it. The closed state is encoded by the open flag, not a
// phase, so phase transitions only run when the viewer is open.
type viewerPhase int

const (
	viewerLoading viewerPhase = iota
	viewerReady
)

// openURL is the URL launcher hook. Tests swap it to capture the URL
// instead of executing xdg-open.
var openURL = func(url string) error {
	return exec.Command("xdg-open", url).Start()
}

// Viewer renders a single message in the right panel. It owns no
// backend reference — body fetch and mark-read Cmds are constructed
// at the AccountTab level. The viewer is pure state + render, with
// scroll position tracked by an embedded bubbles/viewport.
type Viewer struct {
	open        bool
	phase       viewerPhase
	msg         mail.MessageInfo
	accountName string
	blocks      []content.Block
	links       []string
	headerStr   string
	viewport    viewport.Model
	spinner     spinner.Model
	styles      Styles
	theme       *theme.CompiledTheme
	width       int
	height      int
}

// NewViewer constructs an empty (closed) viewer. accountName is used
// to synthesize a To: header until Pass 3 wires real backend headers.
func NewViewer(styles Styles, t *theme.CompiledTheme, accountName string) Viewer {
	return Viewer{
		styles:      styles,
		theme:       t,
		accountName: accountName,
		spinner:     NewSpinner(t),
	}
}

// IsOpen reports whether the viewer is currently displayed.
func (v Viewer) IsOpen() bool { return v.open }

// CurrentUID returns the UID of the message in the viewer, or empty
// when closed. Used by AccountTab to drop stale bodyLoadedMsg events.
func (v Viewer) CurrentUID() mail.UID {
	if !v.open {
		return ""
	}
	return v.msg.UID
}

// Open transitions the viewer into the loading phase for msg. The
// caller fires the body-fetch Cmd in the same Update batch.
func (v Viewer) Open(msg mail.MessageInfo) Viewer {
	v.open = true
	v.phase = viewerLoading
	v.msg = msg
	v.blocks = nil
	v.links = nil
	v.headerStr = ""
	return v
}

// Close transitions the viewer out of view. The caller emits a
// ViewerClosedMsg so chrome (footer, status bar) can revert context.
func (v Viewer) Close() Viewer {
	v.open = false
	v.phase = viewerLoading
	return v
}

// SetBody installs parsed blocks and transitions to ready. Idempotent
// for stale UIDs — callers should drop bodyLoadedMsg with a UID
// mismatch before invoking this.
func (v Viewer) SetBody(blocks []content.Block) Viewer {
	v.blocks = blocks
	v.phase = viewerReady
	v.layout()
	return v
}

// SetSize updates dimensions. When ready, re-renders headers + body
// at the new width and recomputes the viewport height.
func (v Viewer) SetSize(width, height int) Viewer {
	v.width = width
	v.height = height
	if v.phase == viewerReady && v.open {
		v.layout()
	}
	return v
}

// SpinnerTick returns the spinner's initial tick Cmd. Caller batches
// it with the body-fetch Cmd when opening.
func (v Viewer) SpinnerTick() tea.Cmd { return v.spinner.Tick }

// Links returns the harvested URL list. Exposed for tests.
func (v Viewer) Links() []string { return v.links }

// ScrollPct returns the current scroll position as 0..100 percent.
func (v Viewer) ScrollPct() int {
	if v.phase != viewerReady {
		return 0
	}
	return int(v.viewport.ScrollPercent() * 100)
}

// Update handles spinner ticks and key events while open. Returns the
// updated viewer + any Cmds (link launch, viewer-closed signal,
// scroll-position broadcast). Caller is responsible for batching.
func (v Viewer) Update(msg tea.Msg) (Viewer, tea.Cmd) {
	if !v.open {
		return v, nil
	}
	switch m := msg.(type) {
	case spinner.TickMsg:
		if v.phase == viewerLoading {
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
// links; tab is reserved for a link-picker overlay and is a no-op
// here. All other keys forward to the viewport, which is configured
// with a modifier-free keymap (j/k/space/b/g/G).
func (v Viewer) handleKey(msg tea.KeyMsg) (Viewer, tea.Cmd) {
	s := msg.String()
	switch s {
	case "q", "esc":
		v = v.Close()
		return v, viewerClosedCmd()
	case "tab":
		return v, nil
	}
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		idx := int(s[0] - '1')
		if idx < len(v.links) {
			return v, launchURLCmd(v.links[idx])
		}
		return v, nil
	}
	if v.phase != viewerReady {
		return v, nil
	}
	prevPct := v.ScrollPct()
	switch s {
	case "g":
		v.viewport.GotoTop()
	case "G":
		v.viewport.GotoBottom()
	default:
		var c tea.Cmd
		v.viewport, c = v.viewport.Update(msg)
		if pct := v.ScrollPct(); pct != prevPct {
			return v, tea.Batch(c, viewerScrollCmd(pct))
		}
		return v, c
	}
	if pct := v.ScrollPct(); pct != prevPct {
		return v, viewerScrollCmd(pct)
	}
	return v, nil
}

// View renders the viewer in its current phase. Returns "" when
// closed so AccountTab.View can fall through to the message list.
func (v Viewer) View() string {
	if !v.open {
		return ""
	}
	if v.phase == viewerLoading {
		text := v.spinner.View() + " Loading message…"
		return lipgloss.Place(
			v.width, v.height,
			lipgloss.Center, lipgloss.Center,
			v.styles.Dim.Render(text),
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left, v.headerStr, v.viewport.View())
}

// layout renders headers + body and populates the viewport. Called
// from SetBody and from SetSize when the viewer is already ready.
// Headers stay pinned above the viewport; only the body scrolls.
func (v *Viewer) layout() {
	hdrs := content.ParsedHeaders{
		From:    []content.Address{{Name: v.msg.From}},
		To:      []content.Address{{Email: v.accountName}},
		Date:    v.msg.Date,
		Subject: v.msg.Subject,
	}
	width := max(1, v.width)
	v.headerStr = content.RenderHeaders(hdrs, v.theme, width)
	body, urls := content.RenderBodyWithFootnotes(v.blocks, v.theme, width)
	v.links = urls
	headerHeight := lipgloss.Height(v.headerStr)
	bodyHeight := max(1, v.height-headerHeight)
	vp := viewport.New(width, bodyHeight)
	vp.KeyMap = viewerViewportKeymap()
	vp.SetContent(body)
	v.viewport = vp
}

// viewerViewportKeymap configures the viewport with modifier-free
// bindings: j/k for line nav, space/b for page nav. g/G are handled
// by the viewer wrapper itself (not the viewport).
func viewerViewportKeymap() viewport.KeyMap {
	return viewport.KeyMap{
		Up:       key.NewBinding(key.WithKeys("k", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down")),
		PageDown: key.NewBinding(key.WithKeys(" ")),
		PageUp:   key.NewBinding(key.WithKeys("b")),
	}
}
