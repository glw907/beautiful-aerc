// SPDX-License-Identifier: MIT

package ui

import (
	"strings"

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

// Phase reports the viewer's current load phase. Used by AccountTab
// to gate n/N during loading so a second fetch isn't queued.
func (v Viewer) Phase() viewerPhase { return v.phase }

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
		if len(v.links) == 0 {
			return v, nil
		}
		return v, linkPickerOpenCmd(v.links)
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
//
// The output is hard-clipped to v.width so the viewer cannot lie to
// its parent's JoinHorizontal — content longer than v.width (e.g. a
// raw URL the body renderer's hardwrap missed) gets truncated rather
// than overflowing into the sidebar column. This is the bubbles-
// component idiom: each component owns its size contract.
func (v Viewer) View() string {
	if !v.open {
		return ""
	}
	if v.phase == viewerLoading {
		text := v.spinner.View() + " Loading message…"
		placed := lipgloss.Place(
			v.width, v.height,
			lipgloss.Center, lipgloss.Center,
			v.styles.Dim.Render(text),
		)
		return clipPane(placed, v.width, v.height)
	}
	headers := padLeftLines(v.headerStr, 1)
	body := padLeftLines(v.viewport.View(), 1)
	out := lipgloss.JoinVertical(lipgloss.Left, headers, body)
	return clipPane(out, v.width, v.height)
}

// clipPane enforces the size contract every bubbletea component
// owes its parent: exactly height rows, each exactly width cells.
// Lines longer than width are truncated (ANSI-aware via
// displayTruncate); shorter lines are padded; the row count is
// enforced both ways. This is what bubbles components do
// internally when they call lipgloss.Place — the same idiom
// applied here so the viewer can never lie to JoinHorizontal.
func clipPane(s string, width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		w := displayCells(line)
		switch {
		case w > width:
			lines[i] = displayTruncate(line, width)
		case w < width:
			lines[i] = line + strings.Repeat(" ", width-w)
		}
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

// layout renders headers + body and populates the viewport. Called
// from SetBody and from SetSize when the viewer is already ready.
// Headers stay pinned above the viewport; only the body scrolls.
//
// contentWidth is one cell narrower than v.width. padLeftLines adds
// the leading space back in View(), so the total per-line cell count
// equals v.width after clipPane pads the remainder.
func (v *Viewer) layout() {
	hdrs := content.ParsedHeaders{
		From:    []content.Address{{Name: v.msg.From}},
		To:      []content.Address{{Email: v.accountName}},
		Date:    v.msg.Date,
		Subject: v.msg.Subject,
	}
	contentWidth := max(1, v.width-1)
	v.headerStr = content.RenderHeaders(hdrs, v.theme, contentWidth)
	body, urls := content.RenderBodyWithFootnotes(v.blocks, v.theme, contentWidth)
	v.links = urls
	headerHeight := lipgloss.Height(v.headerStr)
	bodyHeight := max(1, v.height-headerHeight)
	vp := viewport.New(contentWidth, bodyHeight)
	vp.KeyMap = viewerViewportKeymap()
	vp.SetContent(body)
	v.viewport = vp
}

// padLeftLines prepends n spaces to every newline-separated line in s.
func padLeftLines(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
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
