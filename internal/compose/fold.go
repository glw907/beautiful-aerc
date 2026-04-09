package compose

import (
	"net/mail"
	"strings"

	"github.com/mattn/go-runewidth"
)

// foldableHeaders lists headers that should be folded at recipient boundaries.
// Excludes From (single address, never folded).
var foldableHeaders = map[string]bool{"to": true, "cc": true, "bcc": true}

// foldAddresses wraps To, Cc, and Bcc headers at recipient boundaries
// to fit within 72 columns. Continuation lines are indented to align
// under the first address. Single-recipient and non-address headers
// pass through unchanged.
func foldAddresses(headers []string) []string {
	var result []string
	for _, line := range headers {
		key, value, ok := splitHeader(line)
		if !ok || !foldableHeaders[strings.ToLower(key)] || strings.TrimSpace(value) == "" {
			result = append(result, line)
			continue
		}
		addrs, err := mail.ParseAddressList(value)
		if err != nil || len(addrs) < 2 {
			result = append(result, line)
			continue
		}

		indent := strings.Repeat(" ", len(key)+2)
		formatted := make([]string, len(addrs))
		for i, a := range addrs {
			formatted[i] = formatAddr(a)
		}

		cur := key + ": " + formatted[0]
		for j := 1; j < len(formatted); j++ {
			candidate := cur + ", " + formatted[j]
			if runewidth.StringWidth(candidate) <= maxWidth {
				cur = candidate
			} else {
				result = append(result, cur+",")
				cur = indent + formatted[j]
			}
		}
		result = append(result, cur)
	}
	return result
}
