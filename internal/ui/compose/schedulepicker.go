package compose

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	mailcompose "github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// SchedulePicker picks a send time: three presets plus a free-form Custom row.
type SchedulePicker struct {
	now        time.Time
	cursor     int // 0..3 (3 = Custom)
	customOpen bool
	input      textinput.Model
	parseErr   string
	styles     Styles
	width      int
	height     int
}

// NewSchedulePicker builds the picker against now. Initial seeds the
// Custom-row textinput when non-empty (used by reschedule).
func NewSchedulePicker(t *theme.CompiledTheme, now time.Time, initial string) SchedulePicker {
	in := textinput.New()
	in.Prompt = "  "
	in.Placeholder = "tomorrow 3pm"
	in.SetVirtualCursor(false)
	in.SetValue(initial)
	in.CharLimit = 64
	p := SchedulePicker{
		now:    now,
		styles: NewStyles(t),
		input:  in,
	}
	if initial != "" {
		p.cursor = 3
		p.customOpen = true
		p.input.Focus()
	}
	return p
}

func (p SchedulePicker) presets() []presetRow {
	return []presetRow{
		{"Tomorrow morning", "8:00 AM", mailcompose.AtHM(p.now.AddDate(0, 0, 1), 8, 0)},
		{"Tomorrow afternoon", "1:00 PM", mailcompose.AtHM(p.now.AddDate(0, 0, 1), 13, 0)},
		{"Monday morning", "8:00 AM", mailcompose.AtHM(mailcompose.NextWeekday(p.now, time.Monday, false), 8, 0)},
	}
}

type presetRow struct {
	label string
	time  string
	when  time.Time
}

func (p SchedulePicker) Init() tea.Cmd { return nil }

func (p *SchedulePicker) moveUp() {
	if p.cursor > 0 {
		p.cursor--
		p.customOpen = false
	}
}

func (p *SchedulePicker) moveDown() {
	if p.cursor < 3 {
		p.cursor++
		if p.cursor != 3 {
			p.customOpen = false
		}
	}
}

func (p SchedulePicker) Update(msg tea.Msg) (SchedulePicker, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}

	if p.customOpen {
		switch km.Code {
		case tea.KeyEnter:
			when, err := mailcompose.ParseSchedule(p.input.Value(), p.now)
			if err != nil {
				p.parseErr = "not a recognized date — try \"tomorrow 3pm\" or \"2026-05-15 09:00\""
				return p, nil
			}
			return p, func() tea.Msg { return ScheduleAcceptedMsg{When: when} }
		case tea.KeyEsc:
			return p, func() tea.Msg { return ScheduleCancelledMsg{} }
		}
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		p.parseErr = ""
		return p, cmd
	}

	switch {
	case km.Code == tea.KeyUp || km.Code == 'k':
		p.moveUp()
	case km.Code == tea.KeyDown || km.Code == 'j':
		p.moveDown()
	case km.Code == tea.KeyEsc:
		return p, func() tea.Msg { return ScheduleCancelledMsg{} }
	case km.Code == tea.KeyEnter:
		if p.cursor == 3 {
			p.customOpen = true
			p.input.Focus()
			return p, nil
		}
		when := p.presets()[p.cursor].when
		return p, func() tea.Msg { return ScheduleAcceptedMsg{When: when} }
	}
	return p, nil
}

// Cursor returns the terminal cursor for the custom-time textinput in
// box-local coordinates (relative to the Box top-left corner returned by
// View). Returns nil when the custom row is not open.
//
// Box layout when customOpen:
//
//	row 0: ┌─ title ─┐
//	row 1: │preset 0│
//	row 2: │preset 1│
//	row 3: │preset 2│
//	row 4: │Custom…│
//	row 5: │textinput│
//
// The App adds the overlay origin (x, y) to produce global coordinates.
func (p SchedulePicker) Cursor() *tea.Cursor {
	if !p.customOpen {
		return nil
	}
	cur := p.input.Cursor()
	if cur == nil {
		return nil
	}
	// Row 5 in the box; left border "│" occupies column 0.
	cur.Position.X += 1 // shift past the "│" left border
	cur.Position.Y = 5
	return cur
}

func (p *SchedulePicker) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.input.SetWidth(w - 8)
}

func (p SchedulePicker) View() string {
	var b strings.Builder
	for i, r := range p.presets() {
		marker := "  "
		if i == p.cursor {
			marker = "▶ "
		}
		b.WriteString(marker + uicore.PadOrTruncate(r.label, 22) + r.time + "\n")
	}
	customMarker := "  "
	if p.cursor == 3 {
		customMarker = "▶ "
	}
	b.WriteString(customMarker + "Custom…\n")
	if p.customOpen {
		b.WriteString(p.input.View() + "\n")
		if p.parseErr != "" {
			b.WriteString(p.styles.PickerError.Render(p.parseErr))
		}
	}
	body := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	footer := []string{"j/k nav  ⏎ pick  Esc cancel"}
	contentW := 44
	shell := uicore.ModalShell{}
	return shell.Box("Schedule send", body, footer, contentW)
}
