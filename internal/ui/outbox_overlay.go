package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// OpenConflictsFromOutboxMsg is emitted by the outbox overlay when
// the user presses ! to transition to the conflict overlay.
type OpenConflictsFromOutboxMsg struct{}

// OutboxOverlay is the modal opened by Q. Read-only summary of the
// drainer's queue, grouped by (kind, folder, status). App owns the
// load → SetGroups → render lifecycle.
type OutboxOverlay struct {
	shell  uicore.ModalShell
	styles Styles
	groups []cache.OutboxGroup
	keys   outboxKeys
	// nowSec is a test seam. Returns "now" in unix seconds for the
	// "retrying in Ns" rendering. Defaults to wall clock.
	nowSec func() int64
}

type outboxKeys struct {
	Close         key.Binding
	OpenConflicts key.Binding
	Swallow       key.Binding
}

// NewOutboxOverlay constructs a closed OutboxOverlay with default keys.
func NewOutboxOverlay(styles Styles) OutboxOverlay {
	return OutboxOverlay{
		styles: styles,
		keys: outboxKeys{
			Close:         key.NewBinding(key.WithKeys("esc", "Q")),
			OpenConflicts: key.NewBinding(key.WithKeys("!")),
			Swallow:       key.NewBinding(key.WithKeys("q")),
		},
		nowSec: func() int64 { return time.Now().Unix() },
	}
}

func (o OutboxOverlay) IsOpen() bool { return o.shell.IsOpen() }

func (o OutboxOverlay) Open(groups []cache.OutboxGroup) OutboxOverlay {
	o.shell = o.shell.WithOpen(true)
	o.groups = groups
	return o
}

func (o OutboxOverlay) Close() OutboxOverlay {
	o.shell = o.shell.WithOpen(false)
	o.groups = nil
	return o
}

// SetGroups replaces the displayed groups without toggling open state.
func (o OutboxOverlay) SetGroups(groups []cache.OutboxGroup) OutboxOverlay {
	o.groups = groups
	return o
}

func (o OutboxOverlay) SetSize(w, h int) OutboxOverlay {
	o.shell = o.shell.SetSize(w, h)
	return o
}

func (o OutboxOverlay) Update(msg tea.Msg) (OutboxOverlay, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch {
	case key.Matches(km, o.keys.OpenConflicts):
		return o.Close(), func() tea.Msg { return OpenConflictsFromOutboxMsg{} }
	case key.Matches(km, o.keys.Close):
		return o.Close(), nil
	case key.Matches(km, o.keys.Swallow):
		return o, nil // overlay does not quit the app
	}
	return o, nil
}

func (o OutboxOverlay) View() string {
	if !o.shell.IsOpen() {
		return ""
	}
	contentW := outboxContentWidth(o.shell.Width())
	bodyRows := o.bodyRows(contentW)
	footerRows := o.footerRows(contentW)
	return o.shell.Box("Outbox", bodyRows, footerRows, contentW)
}

// outboxContentWidth caps the modal at 56 cells, clamped for narrow terminals.
func outboxContentWidth(termW int) int {
	w := 56
	if max := termW - 6; max < w {
		w = max
	}
	if w < 20 {
		w = 20
	}
	return w
}

func (o OutboxOverlay) bodyRows(contentW int) []string {
	if len(o.groups) == 0 {
		return []string{uicore.PadOrTruncate(" Outbox is empty.", contentW)}
	}
	now := o.nowSec()
	rows := make([]string, 0, len(o.groups))
	for _, g := range o.groups {
		rows = append(rows, uicore.PadOrTruncate(" "+formatOutboxGroup(g, now), contentW))
	}
	return rows
}

func (o OutboxOverlay) footerRows(contentW int) []string {
	return []string{uicore.PadOrTruncate(" ! conflicts  ·  q close", contentW)}
}

// formatOutboxGroup renders a single group line. Examples:
//
//	Move → Archive · 23 pending
//	Flag · 1 executing
//	Delete · 1 failed, retrying in 12s
func formatOutboxGroup(g cache.OutboxGroup, nowSec int64) string {
	verb := outboxKindVerb(g.Kind)
	dest := ""
	if g.Kind == cache.KindMove && g.Folder != "" {
		dest = " → " + g.Folder
	}
	suffix := ""
	if g.Status == cache.OpFailed && g.NextAt.Valid {
		secs := g.NextAt.Int64/int64(1e9) - nowSec
		if secs > 0 {
			suffix = fmt.Sprintf(", retrying in %ds", secs)
		}
	}
	return fmt.Sprintf("%s%s · %d %s%s", verb, dest, g.Count, g.Status, suffix)
}

func outboxKindVerb(k cache.OpKind) string {
	switch k {
	case cache.KindMove:
		return "Move"
	case cache.KindFlag:
		return "Flag"
	case cache.KindDestroy:
		return "Delete"
	case cache.KindSend:
		return "Send"
	case cache.KindAppend:
		return "Append"
	}
	return string(k)
}
