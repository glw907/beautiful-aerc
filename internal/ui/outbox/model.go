package outbox

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// Model is a read-only view over scheduled outbox rows.
type Model struct {
	rows   []cache.OutboxRow
	cursor int
	now    time.Time
	width  int
	height int
	styles Styles
}

// New builds the outbox view bound to the given theme.
func New(t *theme.CompiledTheme) Model {
	return Model{styles: NewStyles(t), now: time.Now()}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetRows replaces the row set and preserves the cursor by op ID,
// clamping to the nearest valid index on a miss.
func (m *Model) SetRows(rows []cache.OutboxRow) {
	prevID := int64(-1)
	if m.cursor < len(m.rows) {
		prevID = m.rows[m.cursor].ID
	}
	m.rows = rows
	m.cursor = 0
	for i, r := range rows {
		if r.ID == prevID {
			m.cursor = i
			break
		}
	}
	if m.cursor >= len(m.rows) && len(m.rows) > 0 {
		m.cursor = len(m.rows) - 1
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case km.Type == tea.KeyEsc:
		return m, func() tea.Msg { return CloseMsg{} }
	case km.Type == tea.KeyRunes && len(km.Runes) == 1:
		switch km.Runes[0] {
		case 'q':
			return m, func() tea.Msg { return CloseMsg{} }
		case 'j':
			if len(m.rows) > 0 && m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case 'k':
			if len(m.rows) > 0 && m.cursor > 0 {
				m.cursor--
			}
		case 'c':
			if len(m.rows) == 0 {
				return m, nil
			}
			id := m.rows[m.cursor].ID
			return m, func() tea.Msg { return CancelMsg{OpID: id} }
		case 's':
			if len(m.rows) == 0 {
				return m, nil
			}
			r := m.rows[m.cursor]
			return m, func() tea.Msg {
				return RescheduleMsg{OpID: r.ID, Initial: r.ScheduledFor.Format("2006-01-02 15:04")}
			}
		case 'e':
			if len(m.rows) == 0 {
				return m, nil
			}
			r := m.rows[m.cursor]
			if r.Draft == nil {
				return m, nil
			}
			return m, func() tea.Msg { return EditAsDraftMsg{OpID: r.ID, Draft: r.Draft} }
		}
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	header := fmt.Sprintf("Outbox (%d)", len(m.rows))
	b.WriteString(m.styles.Header.Render(header) + "\n\n")
	if len(m.rows) == 0 {
		fill := strings.Repeat("\n", m.height/2-2)
		return b.String() + fill + m.styles.Empty.Render("Outbox is empty")
	}
	for i, r := range m.rows {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		row := fmt.Sprintf("%s%s  %-22s  %-30s  %s",
			marker,
			uicore.PadOrTruncate(formatWhen(r.ScheduledFor, m.now), 18),
			ansix.TruncateEllipsis(firstAddr(r.To), 22),
			ansix.TruncateEllipsis(r.Subject, 30),
			r.Status,
		)
		b.WriteString(row + "\n")
	}
	b.WriteString("\n  c cancel  s reschedule  e edit-as-draft  q close\n")
	return b.String()
}

func firstAddr(to []string) string {
	if len(to) == 0 {
		return ""
	}
	return to[0]
}

func formatWhen(t, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := t.Sub(now)
	switch {
	case d < 0:
		return t.Format("Mon Jan 2 3:04 PM")
	case d < 24*time.Hour:
		return t.Format("Today 3:04 PM")
	case d < 48*time.Hour:
		return t.Format("Tomorrow 3:04 PM")
	default:
		return t.Format("Mon Jan 2 3:04 PM")
	}
}
