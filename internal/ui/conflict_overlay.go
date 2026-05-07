package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// RetryConflictMsg / DiscardConflictMsg fire on r / d; App issues the
// cache cmd in response.
type RetryConflictMsg struct{ OpID int64 }
type DiscardConflictMsg struct{ OpID int64 }

// conflictCache is the heap-allocated render cache (ADR-0130 escape
// hatch). The pointer is shared across value copies; dirty flips on any
// rows / cursor / size change.
type conflictCache struct {
	dirty bool
	view  string
}

// ConflictOverlay is the modal opened by `!`: a list of conflicted ops
// with per-row retry / discard. App owns the load → SetRows → render
// lifecycle.
type ConflictOverlay struct {
	shell  uicore.ModalShell
	styles Styles
	rows   []cache.ConflictRow
	cursor int
	keys   conflictKeys
	cache  *conflictCache
	nowSec func() int64
}

type conflictKeys struct {
	Up      key.Binding
	Down    key.Binding
	Retry   key.Binding
	Discard key.Binding
	Close   key.Binding
	Swallow key.Binding
}

func NewConflictOverlay(styles Styles) ConflictOverlay {
	return ConflictOverlay{
		styles: styles,
		keys: conflictKeys{
			Up:      key.NewBinding(key.WithKeys("k", "up")),
			Down:    key.NewBinding(key.WithKeys("j", "down")),
			Retry:   key.NewBinding(key.WithKeys("r")),
			Discard: key.NewBinding(key.WithKeys("d")),
			Close:   key.NewBinding(key.WithKeys("esc", "!")),
			Swallow: key.NewBinding(key.WithKeys("q")),
		},
		cache:  &conflictCache{dirty: true},
		nowSec: func() int64 { return time.Now().Unix() },
	}
}

func (c ConflictOverlay) IsOpen() bool { return c.shell.IsOpen() }

func (c ConflictOverlay) Open(rows []cache.ConflictRow) ConflictOverlay {
	c.shell = c.shell.WithOpen(true)
	c.rows = rows
	c.cursor = 0
	c.cache.dirty = true
	return c
}

func (c ConflictOverlay) Close() ConflictOverlay {
	c.shell = c.shell.WithOpen(false)
	c.rows = nil
	c.cursor = 0
	c.cache.dirty = true
	return c
}

// SetRows replaces the displayed rows and clamps cursor.
func (c ConflictOverlay) SetRows(rows []cache.ConflictRow) ConflictOverlay {
	c.rows = rows
	if c.cursor >= len(rows) {
		c.cursor = max(0, len(rows)-1)
	}
	c.cache.dirty = true
	return c
}

func (c ConflictOverlay) RowCount() int { return len(c.rows) }

func (c ConflictOverlay) SetSize(w, h int) ConflictOverlay {
	c.shell = c.shell.SetSize(w, h)
	c.cache.dirty = true
	return c
}

func (c ConflictOverlay) Update(msg tea.Msg) (ConflictOverlay, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return c, nil
	}
	switch {
	case key.Matches(km, c.keys.Down):
		if c.cursor < len(c.rows)-1 {
			c.cursor++
			c.cache.dirty = true
		}
		return c, nil
	case key.Matches(km, c.keys.Up):
		if c.cursor > 0 {
			c.cursor--
			c.cache.dirty = true
		}
		return c, nil
	case key.Matches(km, c.keys.Retry):
		if len(c.rows) == 0 {
			return c, nil
		}
		opID := c.rows[c.cursor].ID
		return c, func() tea.Msg { return RetryConflictMsg{OpID: opID} }
	case key.Matches(km, c.keys.Discard):
		if len(c.rows) == 0 {
			return c, nil
		}
		opID := c.rows[c.cursor].ID
		return c, func() tea.Msg { return DiscardConflictMsg{OpID: opID} }
	case key.Matches(km, c.keys.Close):
		return c.Close(), nil
	case key.Matches(km, c.keys.Swallow):
		return c, nil
	}
	return c, nil
}

// View caches the rendered string via the heap-allocated *conflictCache
// pointer (ADR-0130 escape hatch).
func (c ConflictOverlay) View() string {
	if !c.shell.IsOpen() {
		return ""
	}
	if !c.cache.dirty {
		return c.cache.view
	}
	contentW := conflictContentWidth(c.shell.Width())
	bodyRows := c.bodyRows(contentW)
	footerRows := c.footerRows(contentW)
	c.cache.view = c.shell.Box("Conflicts", bodyRows, footerRows, contentW)
	c.cache.dirty = false
	return c.cache.view
}

func conflictContentWidth(termW int) int {
	w := 76
	if max := termW - 6; max < w {
		w = max
	}
	if w < 30 {
		w = 30
	}
	return w
}

func (c ConflictOverlay) bodyRows(contentW int) []string {
	if len(c.rows) == 0 {
		return []string{uicore.PadOrTruncate(" No conflicts.", contentW)}
	}
	maxRows := conflictMaxBodyLines(c.shell.Height())
	visible := c.rows
	overflow := 0
	if 2*len(visible) > maxRows {
		visible = visible[:maxRows/2]
		overflow = len(c.rows) - len(visible)
	}
	now := c.nowSec()
	rows := make([]string, 0, 2*len(visible)+1)
	for i, r := range visible {
		marker := "│ "
		if i == c.cursor {
			marker = "┃ "
		}
		rows = append(rows, uicore.PadOrTruncate(marker+formatConflictHeader(r), contentW))
		rows = append(rows, uicore.PadOrTruncate("│   "+formatConflictDetail(r, now), contentW))
	}
	if overflow > 0 {
		rows = append(rows, uicore.PadOrTruncate(fmt.Sprintf("│ +%d more (resolve to see)", overflow), contentW))
	}
	return rows
}

func (c ConflictOverlay) footerRows(contentW int) []string {
	if len(c.rows) == 0 {
		return []string{uicore.PadOrTruncate(" q close", contentW)}
	}
	return []string{uicore.PadOrTruncate(" r retry  ·  d discard  ·  q close", contentW)}
}

func conflictMaxBodyLines(termH int) int {
	avail := termH - 9
	if avail < 2 {
		avail = 2
	}
	if avail%2 == 1 {
		avail--
	}
	return avail
}

// formatConflictHeader returns a one-line summary. Folder is the source.
// Move-op destinations live in args (decoded view is out of scope here).
func formatConflictHeader(r cache.ConflictRow) string {
	verb := outboxKindVerb(r.Kind)
	id := shortPID(r.ProtocolID)
	if id != "" && r.Folder != "" {
		return fmt.Sprintf("%s %s in %s", verb, id, r.Folder)
	}
	if id != "" {
		return verb + " " + id
	}
	return verb
}

func formatConflictDetail(r cache.ConflictRow, nowSec int64) string {
	age := humanizeAge(nowSec - r.EnqueuedAt.Unix())
	kind := r.ErrorKind
	if kind == "" {
		kind = "error"
	}
	return fmt.Sprintf("%s: %s  (%d attempts, %s ago)", kind, r.ErrorMessage, r.Attempts, age)
}

func shortPID(pid string) string {
	if len(pid) <= 8 {
		return pid
	}
	return pid[:8]
}

func humanizeAge(secs int64) string {
	if secs < 0 {
		secs = 0
	}
	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		return fmt.Sprintf("%dm", secs/60)
	case secs < 86400:
		return fmt.Sprintf("%dh", secs/3600)
	default:
		return fmt.Sprintf("%dd", secs/86400)
	}
}
