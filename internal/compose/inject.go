package compose

import "strings"

// injectCcBcc inserts empty Cc: and Bcc: headers after the To: or Cc:
// block if they are not already present. Handles continuation lines
// (indented lines) that foldAddresses may have created.
func injectCcBcc(headers []string) []string {
	hasCc, hasBcc := false, false
	toEnd := -1
	ccEnd := -1

	for i, line := range headers {
		key, _, ok := splitHeader(line)
		if ok {
			switch strings.ToLower(key) {
			case "cc":
				hasCc = true
				ccEnd = i
			case "bcc":
				hasBcc = true
			case "to":
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

	// Insert after Cc if present, else after To. Skip past any
	// continuation lines (indented) that follow the insertion point.
	insertAfter := toEnd
	if hasCc && ccEnd >= 0 {
		insertAfter = ccEnd
	}
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
