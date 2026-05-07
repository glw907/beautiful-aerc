package contacts

import (
	"strings"

	"github.com/glw907/poplar/internal/ui/uicore"
)

// RenderDetailCard renders a contact as a formatted card of the given
// cell width. Lines exceeding width are truncated with an ellipsis.
// Shared by the i-popover and the Contacts-mode right column.
//
// Layout:
//   - Name (bold)
//   - Title · Org (omitted for KindOrg or when both blank)
//   - blank line
//   - Email rows: address  (label, primary) / (label) / (primary)
//   - blank line (only when phones are present)
//   - Phone rows: pretty(e164)  (label, primary) / (label) / (primary)
//   - separator rule (────) only when note is non-empty
//   - Note lines (embedded \n preserved)
func RenderDetailCard(c Contact, width int, s Styles) string {
	var lines []string

	// Name line.
	lines = append(lines, uicore.TruncateToWidth(s.Name.Render(c.Name), width))

	// Title · Org line. Skip for KindOrg and when both are blank.
	if c.Kind != KindOrg && (c.Title != "" || c.Org != "") {
		titleOrg := buildTitleOrg(c.Title, c.Org)
		lines = append(lines, uicore.TruncateToWidth(s.TitleOrg.Render(titleOrg), width))
	}

	lines = append(lines, "")

	// Email rows.
	for i, e := range c.Emails {
		lines = append(lines, renderLabelRow(e.Address, e.Label, i == 0, s, width))
	}

	// Blank between emails and phones, only when phones present.
	if len(c.Phones) > 0 {
		lines = append(lines, "")
		for i, p := range c.Phones {
			lines = append(lines, renderLabelRow(formatPhone(p.E164), p.Label, i == 0, s, width))
		}
	}

	// Separator + note.
	note := strings.TrimSpace(c.Note)
	if note != "" {
		rule := s.Rule.Render(strings.Repeat("─", width))
		lines = append(lines, rule)
		for _, l := range strings.Split(note, "\n") {
			lines = append(lines, uicore.TruncateToWidth(s.Body.Render(l), width))
		}
	}

	return strings.Join(lines, "\n")
}

// renderLabelRow renders "<value>  (<label>, primary)" with the label
// suffix dimmed. Truncates to width cells.
func renderLabelRow(value, label string, primary bool, s Styles, width int) string {
	v := s.Body.Render(value)
	suffix := labelSuffix(label, primary)
	if suffix != "" {
		return uicore.TruncateToWidth(v+" "+s.Dim.Render(suffix), width)
	}
	return uicore.TruncateToWidth(v, width)
}

func buildTitleOrg(title, org string) string {
	switch {
	case title != "" && org != "":
		return title + " · " + org
	case title != "":
		return title
	default:
		return org
	}
}

// labelSuffix builds the parenthetical suffix for an email or phone row.
// primary is true for index 0. An empty label on index 0 yields "(primary)";
// an empty label on later rows yields "" (no parens).
func labelSuffix(label string, primary bool) string {
	if label == "" && primary {
		return "(primary)"
	}
	if label == "" {
		return ""
	}
	if primary {
		return "(" + label + ", primary)"
	}
	return "(" + label + ")"
}

// formatPhone pretty-prints an E.164 number. US numbers (+1XXXXXXXXXX,
// 12 chars total) become "+1 NXX-XXXX" style using the 7-digit local
// number (e.g. "+15555550100" → "+1 555-0100"). Everything else is
// returned unchanged.
// TODO(glw907): replace with phonenumbers.Format once libphonenumber lands in 9.2.
func formatPhone(e164 string) string {
	if len(e164) == 12 && e164[0] == '+' && e164[1] == '1' {
		digits := e164[2:] // 10-digit NANP number
		return "+1 " + digits[3:6] + "-" + digits[6:]
	}
	return e164
}
