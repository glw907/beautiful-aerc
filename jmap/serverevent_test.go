//go:build conformance || live

// The event framing both server suites read with. Neither of them may
// use this package's own reader: the question each asks is what the
// server put on the wire, and a reader that normalises the stream
// answers with its own behaviour instead.
package jmap_test

import (
	"bufio"
	"io"
	"strings"
)

// A serverEvent is one server-sent event read off the wire, before
// any type in this package sees it.
type serverEvent struct {
	name string
	data string
	id   string
}

// readServerEvents frames the first count events off body by the
// WHATWG server-sent events rules, and returns what it has when the
// stream ends first. A field with no colon is a name with an empty
// value, and one leading space after the colon belongs to the
// separator rather than to the value.
func readServerEvents(body io.Reader, count int) []serverEvent {
	var events []serverEvent
	var current serverEvent

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if current.name != "" || current.data != "" {
				events = append(events, current)
			}
			current = serverEvent{}
			if len(events) >= count {
				return events
			}
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			current.name = value
		case "data":
			current.data += value
		case "id":
			current.id = value
		}
	}
	return events
}
