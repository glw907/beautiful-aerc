package reader

import (
	"fmt"
	"strings"
	"time"

	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/icalendar"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// renderInviteBlock formats inv into a fixed-width block placed between the
// viewer header panel and the chip row. Returns ("", 0) for nil or zero width.
func renderInviteBlock(m ansix.Measurer, inv *icalendar.Invite, icons uicore.IconSet, st Styles, width int) (string, int) {
	if inv == nil || width < 1 {
		return "", 0
	}

	bg := st.ViewerBg
	var rows []string

	if inv.Method == icalendar.MethodCancel {
		rows = append(rows, st.InviteCancelled.Render("[CANCELLED]"))
	}

	iconStr := st.InviteIcon.Render(icons.Calendar)
	summaryBudget := max(0, width-m.Width(iconStr)-2)
	rows = append(rows, iconStr+"  "+st.InviteSummary.Render(truncate(m, inv.Summary, summaryBudget)))

	rows = append(rows, st.InviteField.Render("    When: "+formatWhen(inv.Start, inv.End)))
	if inv.Location != "" {
		rows = append(rows, st.InviteField.Render("    Where: "+inv.Location))
	}
	if inv.Organizer != "" {
		rows = append(rows, st.InviteField.Render("    Organizer: "+inv.Organizer))
	}
	if inv.AttendeeCount > 0 {
		noun := "attendees"
		if inv.AttendeeCount == 1 {
			noun = "attendee"
		}
		rows = append(rows, st.InviteField.Render(fmt.Sprintf("    %d %s", inv.AttendeeCount, noun)))
	}
	if inv.Recurrence != "" {
		rows = append(rows, st.InviteField.Render("    Repeats: "+inv.Recurrence))
	}

	for i, r := range rows {
		if m.Width(r) > width {
			r = m.TruncateEllipsis(r, width)
		}
		rows[i] = uicore.FillRowToWidth(m, r, width, bg)
	}
	return strings.Join(rows, "\n"), len(rows)
}

func truncate(m ansix.Measurer, s string, w int) string {
	if m.Width(s) <= w {
		return s
	}
	return m.TruncateEllipsis(s, w)
}

func formatWhen(start, end time.Time) string {
	if start.IsZero() {
		return "(no start time)"
	}
	// All-day check must run before local-time conversion: iCalendar DATE
	// values arrive as UTC midnight; local conversion would obscure the signal.
	if !end.IsZero() && isAllDay(start, end) {
		return start.In(time.Local).Format("Mon 2006-01-02") + " (all day)"
	}
	start = start.In(time.Local)
	if end.IsZero() {
		return start.Format("Mon 2006-01-02, 3:04 PM")
	}
	end = end.In(time.Local)
	if sameDay(start, end) {
		return start.Format("Mon 2006-01-02, 3:04 PM") + " – " + end.Format("3:04 PM")
	}
	return start.Format("Mon 2006-01-02, 3:04 PM") + " – " + end.Format("Mon 2006-01-02, 3:04 PM")
}

func isAllDay(start, end time.Time) bool {
	return start.Hour() == 0 && start.Minute() == 0 && start.Second() == 0 &&
		end.Sub(start) == 24*time.Hour
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
