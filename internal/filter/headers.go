// Package filter implements aerc email content filters.
package filter

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ColorSet holds pre-computed ANSI escape sequences for output.
type ColorSet struct {
	HdrKey string // bold accent for header keys
	HdrFG  string // foreground for header values
	HdrDim string // dim for angle brackets and separator
	Reset  string
}

// headerBlock holds parsed RFC 2822 headers preserving original key casing.
type headerBlock struct {
	values      map[string]string // lowercase key -> raw value
	originalKey map[string]string // lowercase key -> original case key
}

// parseHeaders reads RFC 2822 headers from r, handling CRLF line endings and
// continuation lines (lines starting with whitespace). Stops at the blank line.
func parseHeaders(r io.Reader) *headerBlock {
	b := &headerBlock{
		values:      make(map[string]string),
		originalKey: make(map[string]string),
	}

	scanner := bufio.NewScanner(r)
	current := ""

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")

		// Blank line marks end of headers.
		if line == "" {
			break
		}

		// Continuation line (folded header).
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			if current != "" {
				b.values[current] = b.values[current] + "\n" + line
			}
			continue
		}

		// New header field.
		colon := strings.IndexByte(line, ':')
		if colon < 1 {
			continue
		}
		key := line[:colon]
		lkey := strings.ToLower(key)
		val := line[colon+1:]

		b.values[lkey] = val
		b.originalKey[lkey] = key
		current = lkey
	}

	return b
}

// stripBareAngles removes angle brackets around email addresses that have no
// display name before them. Matches the AWK: strips <email> at start or after
// ", " (bare angle case).
func stripBareAngles(val string) string {
	// Strip bare <email> at the very start of the value.
	for strings.HasPrefix(val, "<") {
		close := strings.IndexByte(val, '>')
		if close < 0 {
			break
		}
		val = val[1:close] + val[close+1:]
	}

	// Strip bare <email> that appears after ", " (comma-separated list where
	// a recipient is just <email> with no name).
	for {
		idx := strings.Index(val, ", <")
		if idx < 0 {
			break
		}
		after := val[idx+2:] // starts with "<..."
		close := strings.IndexByte(after, '>')
		if close < 0 {
			break
		}
		email := after[1:close]
		rest := after[close+1:]
		val = val[:idx+2] + email + rest
	}

	return val
}

// colorizeValue applies ANSI colors: angle-bracket content gets dim color,
// the rest gets foreground color. If ColorSet has empty strings, output is plain.
func colorizeValue(val string, cs *ColorSet) string {
	if cs.HdrFG == "" && cs.HdrDim == "" {
		return val
	}

	var sb strings.Builder
	for {
		open := strings.IndexByte(val, '<')
		if open < 0 {
			break
		}
		close := strings.IndexByte(val[open:], '>')
		if close < 0 {
			break
		}
		close += open

		sb.WriteString(cs.HdrFG)
		sb.WriteString(val[:open])
		sb.WriteString(cs.HdrDim)
		sb.WriteString(val[open : close+1])
		val = val[close+1:]
	}
	sb.WriteString(cs.HdrFG)
	sb.WriteString(val)
	sb.WriteString(cs.Reset)
	return sb.String()
}

// wrapAddresses splits a comma-separated address list and wraps lines at cols.
// The key (e.g. "To:") appears on the first line; continuation lines are
// indented to align with the first address.
func wrapAddresses(key, addrs string, cols int) []string {
	// Split on comma, trimming spaces.
	parts := strings.Split(addrs, ",")
	rcpts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			rcpts = append(rcpts, p)
		}
	}

	if len(rcpts) == 0 {
		return []string{key}
	}

	indent := strings.Repeat(" ", len(key)+1)
	var lines []string

	cur := key + " " + rcpts[0]
	for i := 1; i < len(rcpts); i++ {
		candidate := cur + ", " + rcpts[i]
		if len(candidate) <= cols {
			cur = candidate
		} else {
			lines = append(lines, cur+",")
			cur = indent + rcpts[i]
		}
	}
	lines = append(lines, cur)
	return lines
}

// Headers formats RFC 2822 headers from r to w using cs for ANSI colors.
// It outputs headers in the fixed order: from, to, cc, bcc, date, subject,
// skipping any that are absent, then draws a separator of cols "─" characters.
func Headers(r io.Reader, w io.Writer, cs *ColorSet, cols int) error {
	b := parseHeaders(r)

	if cols < 1 {
		cols = 80
	}

	order := []string{"from", "to", "cc", "bcc", "date", "subject"}
	addrHeaders := map[string]bool{"from": true, "to": true, "cc": true, "bcc": true}

	for _, k := range order {
		val, ok := b.values[k]
		if !ok {
			continue
		}

		// Unfold continuation lines.
		val = strings.ReplaceAll(val, "\n\t", " ")
		val = strings.ReplaceAll(val, "\n ", " ")

		// Strip leading space (RFC 2822 puts a space after the colon).
		val = strings.TrimPrefix(val, " ")

		origKey := b.originalKey[k] + ":"

		if addrHeaders[k] {
			val = stripBareAngles(val)
			lines := wrapAddresses(origKey, val, cols)
			for _, line := range lines {
				// Extract trailing comma if present (from wrapped lines).
				// The AWK prints the comma after colorize(), i.e. after the reset code.
				trailer := ""
				body := line
				if strings.HasSuffix(line, ",") {
					trailer = ","
					body = line[:len(line)-1]
				}

				colon := strings.IndexByte(body, ':')
				if colon > 0 && colon <= len(origKey) {
					// Line starts with "Key:" — colorize key and value separately.
					rest := body[colon+1:]
					fmt.Fprintln(w, cs.HdrKey+body[:colon+1]+cs.Reset+colorizeValue(rest, cs)+trailer)
				} else {
					// Continuation line — colorize entire line as value.
					fmt.Fprintln(w, colorizeValue(body, cs)+trailer)
				}
			}
		} else {
			fmt.Fprintln(w, cs.HdrKey+origKey+cs.Reset+colorizeValue(" "+val, cs))
		}
	}

	// Draw separator.
	sep := cs.HdrDim + strings.Repeat("─", cols) + cs.Reset
	fmt.Fprintln(w, sep)

	return nil
}
