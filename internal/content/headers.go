package content

import (
	"net/mail"
	"strings"
)

// ParseHeaders parses raw RFC 2822 headers into structured fields.
// Handles continuation lines, CRLF, and bare email addresses.
func ParseHeaders(raw string) ParsedHeaders {
	var h ParsedHeaders

	// Unfold continuation lines and normalize CRLF
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")

	var unfolded []string
	for _, line := range lines {
		if line == "" {
			break // end of headers
		}
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			// Continuation line
			if len(unfolded) > 0 {
				unfolded[len(unfolded)-1] += " " + strings.TrimSpace(line)
			}
		} else {
			unfolded = append(unfolded, line)
		}
	}

	for _, line := range unfolded {
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		val := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "from":
			h.From = ParseAddressList(val)
		case "to":
			h.To = ParseAddressList(val)
		case "cc":
			h.Cc = ParseAddressList(val)
		case "bcc":
			h.Bcc = ParseAddressList(val)
		case "date":
			h.Date = val
		case "subject":
			h.Subject = val
		}
	}

	return h
}

// ParseAddressList parses a comma-separated list of RFC 5322 addresses.
func ParseAddressList(val string) []Address {
	addrs, err := mail.ParseAddressList(val)
	if err != nil {
		// Fall back to treating the whole value as a single bare email.
		email := strings.TrimSpace(val)
		email = strings.TrimPrefix(email, "<")
		email = strings.TrimSuffix(email, ">")
		if email != "" {
			return []Address{{Email: email}}
		}
		return nil
	}

	result := make([]Address, len(addrs))
	for i, a := range addrs {
		result[i] = Address{Name: a.Name, Email: a.Address}
	}
	return result
}
