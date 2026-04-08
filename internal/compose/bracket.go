package compose

import (
	"net/mail"
	"strings"
)

// addressHeaders lists headers that contain email addresses.
var addressHeaders = map[string]bool{
	"from": true, "to": true, "cc": true, "bcc": true,
}

// splitHeader splits "Key: value" into key, value, ok.
func splitHeader(line string) (string, string, bool) {
	idx := strings.Index(line, ":")
	if idx < 1 {
		return "", "", false
	}
	return line[:idx], strings.TrimSpace(line[idx+1:]), true
}

// formatAddr formats a mail.Address for display. Bare addresses (no
// name) are returned without angle brackets. Named addresses use the
// standard RFC 5322 format.
func formatAddr(a *mail.Address) string {
	if a.Name == "" {
		return a.Address
	}
	return a.String()
}

// stripBrackets removes angle brackets from bare email addresses on
// address headers (From, To, Cc, Bcc). Named addresses are untouched.
// If parsing fails, the line passes through unchanged.
func stripBrackets(headers []string) []string {
	result := make([]string, len(headers))
	for i, line := range headers {
		key, value, ok := splitHeader(line)
		if !ok || !addressHeaders[strings.ToLower(key)] || strings.TrimSpace(value) == "" {
			result[i] = line
			continue
		}
		addrs, err := mail.ParseAddressList(value)
		if err != nil {
			result[i] = line
			continue
		}
		parts := make([]string, len(addrs))
		for j, a := range addrs {
			parts[j] = formatAddr(a)
		}
		result[i] = key + ": " + strings.Join(parts, ", ")
	}
	return result
}
