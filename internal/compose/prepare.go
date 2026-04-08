package compose

import (
	"log"
	"strings"
)

const maxWidth = 72

// Options controls compose-prep behavior.
type Options struct {
	InjectCcBcc bool
}

// Prepare normalizes an aerc compose buffer. On any processing error,
// the original input is returned unchanged.
func Prepare(input []byte, opts Options) []byte {
	text := strings.ReplaceAll(string(input), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	// strings.Split on trailing \n produces empty last element
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Find header/body boundary (first blank line)
	boundary := -1
	for i, line := range lines {
		if line == "" {
			boundary = i
			break
		}
	}

	if boundary < 0 {
		log.Println("no header/body boundary found, passing through")
		return input
	}

	headers := lines[:boundary]
	body := lines[boundary+1:]

	headers = unfoldHeaders(headers)
	headers = stripBrackets(headers)
	headers = foldAddresses(headers)

	var result []string
	result = append(result, headers...)
	result = append(result, "")
	result = append(result, body...)

	return []byte(strings.Join(result, "\n") + "\n")
}
