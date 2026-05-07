package uicore

import (
	"regexp"
)

var sgrRe = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// DimANSI returns s with SGR faint (ESC[2m) injected throughout: a leading
// faint, and a faint re-applied after every reset so the dim attribute
// survives the spans lipgloss emits.
func DimANSI(s string) string {
	return "\x1b[2m" + sgrRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := sgrRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		params := sub[1]
		if params == "" || params == "0" {
			return "\x1b[0;2m"
		}
		return "\x1b[2;" + params + "m"
	})
}
