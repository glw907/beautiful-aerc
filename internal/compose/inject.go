package compose

import "strings"

// injectCcBcc inserts empty Cc: and Bcc: headers after the To: block
// if they are not already present. If Cc already exists, Bcc is inserted
// after the Cc block instead.
func injectCcBcc(headers []string) []string {
	hasCc, hasBcc := false, false
	toEnd := -1
	ccEnd := -1
	lastHeaderEnd := -1 // tracks end of the most recently seen header block

	for i, line := range headers {
		key, _, ok := splitHeader(line)
		if ok {
			switch strings.ToLower(key) {
			case "cc":
				hasCc = true
				ccEnd = i
				lastHeaderEnd = i
			case "bcc":
				hasBcc = true
			case "to":
				toEnd = i
				lastHeaderEnd = i
			default:
				lastHeaderEnd = i
			}
		} else if lastHeaderEnd >= 0 && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			// Continuation line: extend the most recently seen header block
			lastHeaderEnd = i
			if ccEnd >= 0 && !hasBcc {
				ccEnd = i
			}
			if toEnd >= 0 && !hasCc {
				toEnd = i
			}
		}
	}

	if hasCc && hasBcc {
		return headers
	}
	if toEnd < 0 {
		return headers
	}

	// Determine insertion point: after Cc block if present, else after To block.
	insertAfter := toEnd
	if hasCc && ccEnd >= 0 {
		insertAfter = ccEnd
	}

	// Scan forward for any remaining continuation lines after insertAfter.
	for j := insertAfter + 1; j < len(headers); j++ {
		if len(headers[j]) > 0 && (headers[j][0] == ' ' || headers[j][0] == '\t') {
			insertAfter = j
		} else {
			break
		}
	}

	result := make([]string, 0, len(headers)+2)
	result = append(result, headers[:insertAfter+1]...)
	if !hasCc {
		result = append(result, "Cc:")
	}
	if !hasBcc {
		result = append(result, "Bcc:")
	}
	result = append(result, headers[insertAfter+1:]...)
	return result
}
