package contacts

import (
	"strings"

	"github.com/glw907/poplar/internal/ui/uicore"
)

// RenderDetailCard renders a contact as a formatted card. Lines wider
// than width are truncated with an ellipsis. Layout: name, optional
// title·org, blank, email rows, blank+phone rows when present, optional
// rule + note.
func RenderDetailCard(c Contact, width int, s Styles) string {
	var lines []string

	lines = append(lines, uicore.TruncateToWidth(s.Name.Render(c.Name), width))

	if c.Kind != KindOrg && (c.Title != "" || c.Org != "") {
		titleOrg := buildTitleOrg(c.Title, c.Org)
		lines = append(lines, uicore.TruncateToWidth(s.TitleOrg.Render(titleOrg), width))
	}

	lines = append(lines, "")

	for i, e := range c.Emails {
		lines = append(lines, renderLabelRow(e.Address, e.Label, i == 0, s, width))
	}

	if len(c.Phones) > 0 {
		lines = append(lines, "")
		for i, p := range c.Phones {
			lines = append(lines, renderLabelRow(formatPhone(p.E164), p.Label, i == 0, s, width))
		}
	}

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

// renderLabelRow renders "<value>  (<label>, primary)" with a dim suffix.
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

// labelSuffix builds the parenthetical suffix. The primary row gets
// "(primary)" or "(label, primary)"; later rows omit the suffix when
// the label is empty.
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

// formatPhone pretty-prints US E.164 numbers as "+1 555-0100"; other
// formats pass through. Replaced by phonenumbers.Format in 9.2.
func formatPhone(e164 string) string {
	if len(e164) == 12 && e164[0] == '+' && e164[1] == '1' {
		digits := e164[2:]
		return "+1 " + digits[3:6] + "-" + digits[6:]
	}
	return e164
}
