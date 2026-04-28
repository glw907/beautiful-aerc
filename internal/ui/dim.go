// SPDX-License-Identifier: MIT

package ui

import (
	"regexp"
)

// sgrRe matches SGR escape sequences: ESC [ <params> m.
// Capturing group 1 holds the parameter string (may be empty for ESC[m).
var sgrRe = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// DimANSI returns s with SGR faint (ESC[2m) injected throughout so that the
// rendered output appears perceptibly dimmer without losing hue information.
//
// Strategy:
//   - Prepend ESC[2m so any leading unstyled text is faint.
//   - For each existing SGR sequence:
//   - If params are empty or "0" (a reset), rewrite to ESC[0;2m so the
//     faint attribute is re-applied after every reset.
//   - Otherwise prepend "2;" to the params so the faint bit rides along
//     with any existing color/bold attributes.
func DimANSI(s string) string {
	// Prepend unconditional faint so plain leading content is dimmed.
	result := "\x1b[2m" + sgrRe.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the params from the capturing group.
		sub := sgrRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		params := sub[1]
		if params == "" || params == "0" {
			// Reset: restore faint after the reset so dim persists.
			return "\x1b[0;2m"
		}
		// Non-reset: inject faint alongside the existing attributes.
		return "\x1b[2;" + params + "m"
	})
	return result
}
