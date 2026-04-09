package compose

import "strings"

// unfoldHeaders joins RFC 2822 continuation lines (lines starting with
// space or tab) onto the preceding line with a single space.
func unfoldHeaders(headers []string) []string {
	var result []string
	for _, line := range headers {
		if len(result) > 0 && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			result[len(result)-1] += " " + strings.TrimLeft(line, " \t")
		} else {
			result = append(result, line)
		}
	}
	return result
}
