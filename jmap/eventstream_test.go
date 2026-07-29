package jmap

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// drain reads every event the stream dispatches and reports them with
// the resume id left in force and the error that ended the read.
func drain(t *testing.T, r io.Reader) ([]event, string, error) {
	t.Helper()
	reader := newEventReader(r)
	var events []event
	for {
		ev, err := reader.next()
		if err != nil {
			return events, reader.lastID, err
		}
		events = append(events, ev)
	}
}

// TestEventReaderFraming covers JT-23. Every row is one rule of the
// WHATWG event stream interpretation that RFC 8620 section 7.3 defers
// to. go-jmap dispatched on the data line rather than the blank line,
// so a multi-line payload was lost in silence, and it never read an
// id field at all, so a reconnect could not name where it left off.
func TestEventReaderFraming(t *testing.T) {
	cases := []struct {
		name   string
		stream string
		want   []event
		wantID string
	}{
		{
			name:   "an event dispatches on the blank line",
			stream: "event: state\ndata: {}\n\n",
			want:   []event{{name: "state", data: "{}"}},
		},
		{
			name:   "an event with no blank line after it is discarded",
			stream: "event: state\ndata: {}\n",
		},
		{
			name:   "a data line unterminated at end of stream is discarded",
			stream: "event: state\ndata: {}",
		},
		{
			name:   "multi-line data joins with newlines",
			stream: "data: first\ndata: second\n\n",
			want:   []event{{name: "message", data: "first\nsecond"}},
		},
		{
			name:   "one leading byte order mark belongs to the encoding",
			stream: "\ufeffevent: state\ndata: {}\n\n",
			want:   []event{{name: "state", data: "{}"}},
		},
		{
			name:   "a second byte order mark is a field name like any other",
			stream: "\ufeff\ufeffevent: state\ndata: {}\n\n",
			want:   []event{{name: "message", data: "{}"}},
		},
		{
			name:   "a byte order mark on a later line is a field name like any other",
			stream: "data: a\n\ufeffdata: b\n\n",
			want:   []event{{name: "message", data: "a"}},
		},
		{
			name:   "an absent event field names the event message",
			stream: "data: hi\n\n",
			want:   []event{{name: "message", data: "hi"}},
		},
		{
			name:   "a comment line is ignored",
			stream: ": keepalive\ndata: hi\n\n",
			want:   []event{{name: "message", data: "hi"}},
		},
		{
			name:   "a line with no colon is a field with an empty value",
			stream: "data\ndata: hi\n\n",
			want:   []event{{name: "message", data: "\nhi"}},
		},
		{
			name:   "an unknown field is ignored",
			stream: "vendor: whatever\ndata: hi\n\n",
			want:   []event{{name: "message", data: "hi"}},
		},
		{
			name:   "a retry field is ignored",
			stream: "retry: 60000\ndata: hi\n\n",
			want:   []event{{name: "message", data: "hi"}},
		},
		{
			name:   "one leading space is stripped and a second is kept",
			stream: "data:  hi\n\n",
			want:   []event{{name: "message", data: " hi"}},
		},
		{
			name:   "a value needs no leading space",
			stream: "data:hi\n\n",
			want:   []event{{name: "message", data: "hi"}},
		},
		{
			name:   "an id is captured and retained across later events",
			stream: "id: e1\ndata: a\n\ndata: b\n\n",
			want: []event{
				{name: "message", data: "a"},
				{name: "message", data: "b"},
			},
			wantID: "e1",
		},
		{
			name:   "an id with no data commits without dispatching",
			stream: "id: e7\n\ndata: a\n\n",
			want:   []event{{name: "message", data: "a"}},
			wantID: "e7",
		},
		{
			name:   "an id holding a NUL is ignored",
			stream: "id: e1\n\nid: e\x002\ndata: a\n\n",
			want:   []event{{name: "message", data: "a"}},
			wantID: "e1",
		},
		{
			name:   "an id whose event never dispatches never becomes the resume point",
			stream: "id: e1\n\nid: e2\ndata: a",
			wantID: "e1",
		},
		{
			name:   "CRLF terminates a line",
			stream: "data: a\r\n\r\n",
			want:   []event{{name: "message", data: "a"}},
		},
		{
			name:   "a bare CR terminates a line",
			stream: "data: a\r\r",
			want:   []event{{name: "message", data: "a"}},
		},
		{
			name:   "an event field with no data dispatches nothing and does not leak into the next event",
			stream: "event: ping\n\ndata: a\n\n",
			want:   []event{{name: "message", data: "a"}},
		},
		{
			name:   "a blank line with nothing pending dispatches nothing",
			stream: "\n\ndata: a\n\n",
			want:   []event{{name: "message", data: "a"}},
		},
		{
			name:   "two events arrive in order",
			stream: "event: state\ndata: one\n\nevent: ping\ndata: two\n\n",
			want: []event{
				{name: "state", data: "one"},
				{name: "ping", data: "two"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, gotID, err := drain(t, strings.NewReader(c.stream))
			if !errors.Is(err, io.EOF) {
				t.Fatalf("read ended with %v, want io.EOF", err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("dispatched %v, want %v", got, c.want)
			}
			for i, want := range c.want {
				if got[i] != want {
					t.Errorf("event %d = %+v, want %+v", i, got[i], want)
				}
			}
			if gotID != c.wantID {
				t.Errorf("resume id = %q, want %q", gotID, c.wantID)
			}
		})
	}
}

// chunkReader hands out one scripted chunk per Read, so a test can put
// a line boundary anywhere it likes.
type chunkReader struct {
	chunks []string
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if len(c.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.chunks[0])
	c.chunks[0] = c.chunks[0][n:]
	if c.chunks[0] == "" {
		c.chunks = c.chunks[1:]
	}
	return n, nil
}

// TestEventReaderJoinsALineSplitAcrossReads is JT-23's boundary case.
// A CR arriving at the end of one read is either a line terminator or
// the first half of a CRLF, and nothing in the buffer says which. A
// reader that guesses splits one line into two, which turns the CR's
// own line into a spurious dispatch and cuts the event short.
func TestEventReaderJoinsALineSplitAcrossReads(t *testing.T) {
	stream := &chunkReader{chunks: []string{"da", "ta: a\r", "\ndata: ", "b\r", "\n\r", "\n"}}

	got, _, err := drain(t, stream)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("read ended with %v, want io.EOF", err)
	}
	want := event{name: "message", data: "a\nb"}
	if len(got) != 1 || got[0] != want {
		t.Errorf("dispatched %v, want one %+v", got, want)
	}
}

// TestEventReaderCarriesALargeEvent covers JT-24. go-jmap left
// bufio.Scanner on its 64 KB default token and never read the error,
// so a longer line ended the stream and reported success: a push
// connection that looked healthy and delivered nothing.
func TestEventReaderCarriesALargeEvent(t *testing.T) {
	const size = 128 * 1024
	payload := strings.Repeat("x", size)

	got, _, err := drain(t, strings.NewReader("data: "+payload+"\n\n"))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("read ended with %v, want io.EOF", err)
	}
	if len(got) != 1 {
		t.Fatalf("dispatched %d events, want 1", len(got))
	}
	if len(got[0].data) != size {
		t.Errorf("event carries %d bytes, want %d", len(got[0].data), size)
	}
}

// TestEventReaderRefusesAnOversizedEvent is the claim the ceiling
// rests on: overrunning it is an error, never a truncation. Both
// halves need saying, because a server can overrun a line or overrun
// an event a legal line at a time, and only the first is
// bufio.Scanner's to notice.
func TestEventReaderRefusesAnOversizedEvent(t *testing.T) {
	line := "data: " + strings.Repeat("x", maxEventBytes+1) + "\n\n"

	oneLine := strings.Repeat("data: "+strings.Repeat("x", 64*1024)+"\n", maxEventBytes/(64*1024)+2)
	cases := []struct {
		name   string
		stream string
		want   error
	}{
		{name: "one line past the ceiling", stream: line, want: bufio.ErrTooLong},
		{name: "legal lines accumulating past the ceiling", stream: oneLine + "\n", want: errEventTooLong},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, err := drain(t, strings.NewReader(c.stream))
			if !errors.Is(err, c.want) {
				t.Fatalf("read ended with %v, want %v", err, c.want)
			}
			if len(got) != 0 {
				t.Errorf("dispatched %d events, want none; the stream was truncated rather than refused", len(got))
			}
		})
	}
}

// TestEventReaderSurfacesAReadError is the other half of JT-24: a
// stream that breaks mid-event says so. Reporting the end of the
// stream instead would hand the caller a clean shutdown it cannot
// distinguish from the server closing politely.
func TestEventReaderSurfacesAReadError(t *testing.T) {
	broken := errors.New("stream reset")
	stream := io.MultiReader(strings.NewReader("data: a\n"), iotest.ErrReader(broken))

	got, _, err := drain(t, stream)
	if !errors.Is(err, broken) {
		t.Fatalf("read ended with %v, want the reader's own error", err)
	}
	if len(got) != 0 {
		t.Errorf("dispatched %v, want nothing from a half-read event", got)
	}
}
