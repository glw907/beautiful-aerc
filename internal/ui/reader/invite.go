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
// Each row is padded to width via FillRowToWidth; rows exceeding width truncate
// with an ellipsis. The block is at most 7 rows tall.
func renderInviteBlock(inv *icalendar.Invite, icons uicore.IconSet, st Styles, width int) (string, int) {
	if inv == nil || width < 1 {
		return "", 0
	}

	bg := st.ViewerBg
	var rows []string

	if strings.EqualFold(inv.Method, "CANCEL") {
		row := st.InviteCancelled.Render("[CANCELLED]")
		rows = append(rows, uicore.FillRowToWidth(row, width, bg))
	}

	iconStr := st.InviteIcon.Render(icons.Calendar)
	iconW := ansix.Width(iconStr)
	summaryBudget := max(0, width-iconW-2)
	summary := inv.Summary
	if ansix.Width(summary) > summaryBudget {
		summary = ansix.TruncateEllipsis(summary, summaryBudget)
	}
	summaryRow := iconStr + "  " + st.InviteSummary.Render(summary)
	rows = append(rows, uicore.FillRowToWidth(summaryRow, width, bg))

	when := formatWhen(inv.Start, inv.End)
	whenRow := st.InviteField.Render("    When: " + when)
	rows = append(rows, uicore.FillRowToWidth(whenRow, width, bg))

	if inv.Location != "" {
		row := st.InviteField.Render("    Where: " + inv.Location)
		rows = append(rows, uicore.FillRowToWidth(row, width, bg))
	}
	if inv.Organizer != "" {
		row := st.InviteField.Render("    Organizer: " + inv.Organizer)
		rows = append(rows, uicore.FillRowToWidth(row, width, bg))
	}
	if inv.AttendeeCount > 0 {
		label := fmt.Sprintf("    %d attendees", inv.AttendeeCount)
		if inv.AttendeeCount == 1 {
			label = "    1 attendee"
		}
		rows = append(rows, uicore.FillRowToWidth(st.InviteField.Render(label), width, bg))
	}
	if inv.Recurrence != "" {
		row := st.InviteField.Render("    Repeats: " + inv.Recurrence)
		rows = append(rows, uicore.FillRowToWidth(row, width, bg))
	}

	for i, r := range rows {
		if ansix.Width(r) > width {
			rows[i] = ansix.TruncateEllipsis(r, width)
		}
	}

	return strings.Join(rows, "\n"), len(rows)
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
