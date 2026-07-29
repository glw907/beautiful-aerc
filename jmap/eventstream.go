package jmap

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"strings"
)

// An event is one server-sent event the stream dispatched.
type event struct {
	// name is the event field's value, or "message" when the server
	// sent none.
	name string

	// data is the event's payload, with the newlines that separated a
	// multi-line field kept and the trailing one removed.
	data string
}

// maxEventBytes bounds one line of a stream and the data one event
// accumulates across its lines. go-jmap took bufio.Scanner's 64 KB
// default for the first, never read the error, and had no notion of
// the second, so a longer line ended the read and reported success. A
// bound stays, since the alternative offers a server unbounded
// allocation from either direction, but it sits far above any
// StateChange, which carries a state token per type per account and no
// message data at all. Overrunning it is an error the caller sees.
const maxEventBytes = 1 << 24

// errEventTooLong reports data accumulated past maxEventBytes across
// an event's lines, the counterpart to bufio.ErrTooLong for one line.
var errEventTooLong = errors.New("event data too long")

// An eventReader assembles server-sent events from a byte stream,
// following the WHATWG event stream interpretation that RFC 8620
// section 7.3 defers to.
//
// started tracks the one place a byte order mark may be dropped, the
// stream's first line.
//
// idBuffer and lastID are that specification's two id slots. An id
// field writes idBuffer; dispatching an event copies idBuffer to
// lastID, and lastID is what a reconnect resends. Keeping them apart
// is what stops an event abandoned at the end of a broken stream from
// advancing the resume point past changes the client never saw.
type eventReader struct {
	lines    *bufio.Scanner
	started  bool
	idBuffer string
	lastID   string
}

func newEventReader(r io.Reader) *eventReader {
	lines := bufio.NewScanner(r)
	lines.Buffer(nil, maxEventBytes)
	lines.Split(scanEventLines)
	return &eventReader{lines: lines}
}

// next returns the next event the stream dispatches, io.EOF at the end
// of a stream, or whatever the read failed with. Fields accumulate
// until a blank line, and a blank line with no data behind it commits
// the id and dispatches nothing.
func (r *eventReader) next() (event, error) {
	var name string
	var data strings.Builder
	for r.lines.Scan() {
		line := r.lines.Text()
		if !r.started {
			// One leading byte order mark belongs to the encoding, not
			// to the first field name. Left in, it makes an "event"
			// field unrecognisable, so the stream's first event
			// degrades to an unnamed one and the caller drops it.
			//
			// The styling analyzer decodes a literal before scanning
			// its runes, so no string spelling of the mark gets past
			// the rule. string([]byte{0xEF, 0xBB, 0xBF}) does get
			// past it, and a reader of that line cannot see which
			// character is meant.
			line = strings.TrimPrefix(line, "\ufeff") //poplar:allow-unicode the analyzer decodes the literal, and the byte-slice form that escapes it hides the character
			r.started = true
		}
		if line == "" {
			r.lastID = r.idBuffer
			if data.Len() == 0 {
				name = ""
				continue
			}
			return event{
				name: cmp.Or(name, "message"),
				data: strings.TrimSuffix(data.String(), "\n"),
			}, nil
		}

		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "":
			// A line opening with a colon is a comment.
		case "event":
			name = value
		case "data":
			data.WriteString(value)
			data.WriteByte('\n')
			if data.Len() > maxEventBytes {
				return event{}, errEventTooLong
			}
		case "id":
			// A NUL means the server garbled the id, and resuming from
			// a garbled id asks for changes since nowhere.
			if !strings.ContainsRune(value, 0) {
				r.idBuffer = value
			}
		case "retry":
			// Ignored by choice. The field lets a server set the
			// client's reconnection delay, and poplar's delay is its
			// own: a server naming an hour would take push down for an
			// hour with no way for the client to disagree.
		}
	}
	if err := r.lines.Err(); err != nil {
		return event{}, fmt.Errorf("read event stream: %w", err)
	}
	return event{}, io.EOF
}

// scanEventLines splits on the three terminators an event stream may
// use: CRLF, CR, and LF. bufio.ScanLines covers the first and the
// last, so a server that ends its lines with a bare CR would hand it
// one line holding the whole stream.
//
// An unterminated remainder at the end of a stream is not a line, and
// the event it belongs to goes with it.
func scanEventLines(data []byte, atEOF bool) (int, []byte, error) {
	lf := bytes.IndexByte(data, '\n')
	cr := bytes.IndexByte(data, '\r')
	switch {
	case lf < 0 && cr < 0:
		return 0, nil, nil
	case cr < 0 || (lf >= 0 && lf < cr):
		return lf + 1, data[:lf], nil
	case cr == len(data)-1 && !atEOF:
		// A CR at the edge of the buffer is either a terminator or the
		// first half of a CRLF, and only the next read says which.
		return 0, nil, nil
	case lf == cr+1:
		return cr + 2, data[:cr], nil
	default:
		return cr + 1, data[:cr], nil
	}
}
