package jmap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// stateEventBody is one RFC 8620 section 7.1.1 push, small enough to
// read inline: one account, one type, one state token.
func stateEventBody(state string) string {
	return fmt.Sprintf("event: state\ndata: {\"@type\":\"StateChange\",\"changed\":{\"A1\":{\"Email\":%q}}}\n\n", state)
}

// A streamRequest is what one connection to the event source arrived
// with, kept so a test can assert on the resume header and the query
// the URL template expanded to.
type streamRequest struct {
	lastEventID string
	query       url.Values
}

// An eventWriter is the scripted server's end of one connection.
type eventWriter struct {
	w    io.Writer
	f    http.Flusher
	done <-chan struct{}
}

// send writes raw stream bytes and flushes them, so the client sees
// them before the connection ends.
func (e *eventWriter) send(raw string) {
	_, _ = io.WriteString(e.w, raw)
	e.f.Flush()
}

// hold blocks until the client hangs up, which keeps the connection
// open the way a real event source does.
func (e *eventWriter) hold() { <-e.done }

// startEventFake serves the session resource and an event stream whose
// nth connection, counting from one, runs script. The script returns
// to close the connection.
func startEventFake(t *testing.T, script func(n int, w *eventWriter)) (*Client, func() []streamRequest) {
	t.Helper()
	client, mux := startFake(t)

	var mu sync.Mutex
	var seen []streamRequest
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, streamRequest{lastEventID: r.Header.Get("Last-Event-ID"), query: r.URL.Query()})
		n := len(seen)
		mu.Unlock()

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("the test server's ResponseWriter cannot flush")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		script(n, &eventWriter{w: w, f: flusher, done: r.Context().Done()})
	})

	return client, func() []streamRequest {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(seen)
	}
}

// A tracker records what a stream reported, so a test waits on a count
// rather than on a sleep.
type tracker struct {
	mu          sync.Mutex
	connects    int
	changes     []*StateChange
	disconnects []error
}

func (tr *tracker) connect() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.connects++
}

func (tr *tracker) change(c *StateChange) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.changes = append(tr.changes, c)
}

func (tr *tracker) disconnect(err error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.disconnects = append(tr.disconnects, err)
}

func (tr *tracker) connectCount() int {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.connects
}

func (tr *tracker) changesSeen() []*StateChange {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.changes)
}

func (tr *tracker) disconnectsSeen() []error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.disconnects)
}

// source returns an EventSource reporting into tr, with whatever else
// the caller set kept.
func (tr *tracker) source(s EventSource) EventSource {
	s.OnConnect, s.OnChange, s.OnDisconnect = tr.connect, tr.change, tr.disconnect
	return s
}

// listen runs Listen on its own goroutine and returns a func the test
// calls to stop it and read what it returned.
func listen(t *testing.T, c *Client, source EventSource) func() error {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- c.Listen(ctx, source) }()
	return func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			t.Fatal("Listen did not return after its context was cancelled")
			return nil
		}
	}
}

// listenOnce runs Listen on a stream that is supposed to end by
// itself, failing the test rather than hanging the suite when it does
// not.
func listenOnce(t *testing.T, c *Client, source EventSource) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- c.Listen(t.Context(), source) }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Listen did not return on a stream that should have ended")
		return nil
	}
}

// waitFor blocks until cond holds, failing the test rather than
// hanging when it never does.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestListenExpandsTheURLTemplate pins RFC 8620 section 7.3's three
// variables. go-jmap parsed the template as a URL and set query
// parameters on it, which leaves a server that spells the template
// anywhere but the query with the braces still in the path.
func TestListenExpandsTheURLTemplate(t *testing.T) {
	cases := []struct {
		name   string
		source EventSource
		want   url.Values
	}{
		{
			name:   "no types subscribes to everything",
			source: EventSource{},
			want:   url.Values{"types": {"*"}, "closeafter": {"no"}, "ping": {"0"}},
		},
		{
			name:   "named types are listed, and a ping is asked for in seconds",
			source: EventSource{Types: []EventType{"Email", "Mailbox"}, Ping: 300 * time.Second},
			want:   url.Values{"types": {"Email,Mailbox"}, "closeafter": {"no"}, "ping": {"300"}},
		},
		{
			name:   "a one-shot stream asks the server to close after a state event",
			source: EventSource{CloseAfterState: true},
			want:   url.Values{"types": {"*"}, "closeafter": {"state"}, "ping": {"0"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, requests := startEventFake(t, func(_ int, w *eventWriter) { w.hold() })
			dial(t, client)

			var tr tracker
			stop := listen(t, client, tr.source(c.source))
			waitFor(t, "the first connection", func() bool { return len(requests()) > 0 })
			_ = stop()

			got := requests()[0]
			for name, want := range c.want {
				if got.query.Get(name) != want[0] {
					t.Errorf("%s = %q, want %q", name, got.query.Get(name), want[0])
				}
			}
			if got.lastEventID != "" {
				t.Errorf("the first connection sent Last-Event-ID %q, want none", got.lastEventID)
			}
		})
	}
}

// TestListenDeliversStateChanges is the whole point of the stream: the
// server says an account moved, and the caller hears which types.
func TestListenDeliversStateChanges(t *testing.T) {
	client, _ := startEventFake(t, func(_ int, w *eventWriter) {
		w.send(stateEventBody("s1"))
		w.hold()
	})
	dial(t, client)

	var tr tracker
	stop := listen(t, client, tr.source(EventSource{}))
	waitFor(t, "the first state change", func() bool { return len(tr.changesSeen()) > 0 })
	_ = stop()

	change := tr.changesSeen()[0]
	if got := change.Changed["A1"]["Email"]; got != "s1" {
		t.Errorf("Email state = %q, want s1", got)
	}
	if tr.connectCount() != 1 {
		t.Errorf("connected %d times, want 1", tr.connectCount())
	}
}

// TestListenResumesFromTheLastEventID covers JT-22, both halves. RFC
// 8620 section 7.3 says a reconnect carries the last id seen and the
// server SHOULD send what the client missed, but it defines no way for
// a server to say it will not, and a grep of apache/james-project
// finds no server-side handling of the header at all. So poplar sends
// the id and assumes nothing: a reconnect is reported as a connection
// whatever the server does with it, and the caller closes the gap with
// /changes. The two rows are the two servers poplar may meet.
func TestListenResumesFromTheLastEventID(t *testing.T) {
	cases := []struct {
		name        string
		onReconnect string
		wantStates  []string
	}{
		{
			name:        "the server honours the header and replays what was missed",
			onReconnect: stateEventBody("s2"),
			wantStates:  []string{"s1", "s2"},
		},
		{
			name:        "the server ignores the header and replays nothing",
			onReconnect: "",
			wantStates:  []string{"s1"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, requests := startEventFake(t, func(n int, w *eventWriter) {
				if n == 1 {
					w.send("id: e1\n" + stateEventBody("s1"))
					return
				}
				w.send(c.onReconnect)
				w.hold()
			})
			dial(t, client)

			var tr tracker
			stop := listen(t, client, tr.source(EventSource{}))
			// The server records a connection before it writes a byte,
			// so waiting on the request count releases ahead of the
			// callbacks below. Wait on the client's own signal instead.
			waitFor(t, "the reconnect", func() bool { return tr.connectCount() > 1 })
			waitFor(t, "the replayed changes", func() bool { return len(tr.changesSeen()) >= len(c.wantStates) })
			_ = stop()

			if got := requests()[1].lastEventID; got != "e1" {
				t.Errorf("the reconnect sent Last-Event-ID %q, want e1", got)
			}
			var states []string
			for _, change := range tr.changesSeen() {
				states = append(states, change.Changed["A1"]["Email"])
			}
			if !slices.Equal(states, c.wantStates) {
				t.Errorf("delivered states %v, want %v; the client invented or dropped a change", states, c.wantStates)
			}
			// The gap signal: a reconnect is a connection like any
			// other, so the caller pulls /changes whether or not the
			// server chose to replay (ADR-0018).
			if tr.connectCount() < 2 {
				t.Errorf("reported %d connections, want one per connection made", tr.connectCount())
			}
			if len(tr.disconnectsSeen()) < 1 {
				t.Error("the drop that forced the reconnect was never reported")
			}
		})
	}
}

// TestListenKeepsTheResumeIDAcrossAConnection covers the half of
// JT-22 a single reconnect cannot see. The event stream rules keep the
// last id in force until another one replaces it, so neither a
// connection that dispatched nothing at all nor one carrying only
// idless events may disturb it. A client that rebuilt the resume point
// per connection would forget it after either, silently, and only on
// the connection after that.
func TestListenKeepsTheResumeIDAcrossAConnection(t *testing.T) {
	client, requests := startEventFake(t, func(n int, w *eventWriter) {
		switch n {
		case 1:
			w.send("id: e1\n" + stateEventBody("s1"))
		case 2:
			// Opened and dropped without dispatching anything.
		case 3:
			w.send(stateEventBody("s2"))
		default:
			w.hold()
		}
	})
	dial(t, client)

	var tr tracker
	stop := listen(t, client, tr.source(EventSource{}))
	waitFor(t, "the fourth connection", func() bool { return len(requests()) > 3 })
	_ = stop()

	for i, req := range requests()[1:4] {
		if req.lastEventID != "e1" {
			t.Errorf("connection %d sent Last-Event-ID %q, want e1", i+2, req.lastEventID)
		}
	}
}

// TestListenAdaptsToTheServersPingInterval covers JT-26. RFC 8620
// section 7.3 lets a server clamp the interval the client asked for
// and report what it chose in the ping payload. A client that keeps
// waiting on its own figure reads a dead stream as healthy for as long
// as the difference, which here is the five minutes it requested
// against the second the server granted.
//
// The elapsed bound pins the other half, ADR-0005 revision 2's rule
// that a drop is silence past twice the interval. The server here
// pings once, waits past one interval, pings again, and then stops. A
// client waiting only one interval would have hung up before that
// second ping, on a stream that was working.
func TestListenAdaptsToTheServersPingInterval(t *testing.T) {
	const granted = time.Second
	client, requests := startEventFake(t, func(n int, w *eventWriter) {
		if n == 1 {
			w.send("event: ping\ndata: {\"interval\":1}\n\n")
			time.Sleep(granted + granted/2)
			w.send("event: ping\ndata: {\"interval\":1}\n\n")
		}
		w.hold()
	})
	dial(t, client)

	var tr tracker
	started := time.Now()
	stop := listen(t, client, tr.source(EventSource{Ping: 300 * time.Second}))
	waitFor(t, "the stall detector to drop a silent stream", func() bool { return len(requests()) > 1 })
	elapsed := time.Since(started)
	_ = stop()

	if got := requests()[0].query.Get("ping"); got != "300" {
		t.Errorf("requested ping = %q, want the 300 the caller asked for", got)
	}
	if want := 2 * granted; elapsed < want {
		t.Errorf("dropped the stream after %v, want no sooner than %v", elapsed, want)
	}
	if len(tr.disconnectsSeen()) < 1 {
		t.Error("the stalled stream was never reported as a drop")
	}
}

// TestPingCadence is JT-26's other end: what the ping payload has to
// say before the stall detector will believe it, and what the detector
// computes from it. Every row asserts the stall window as well as the
// cadence, because the window is what the timer is set from and a
// cadence certified on its own is how a plausible number turns into an
// absurd timeout.
//
// The overflow row is the one with teeth. A figure that wraps a
// Duration lands as a negative timeout, which fires at once, so a
// stream would drop and reconnect on every ping it received.
//
// The Stalwart row is the divergence this clamps for: it reports 30000
// while pinging every 30 seconds, and believing that number as seconds
// leaves a dead connection unnoticed for 16h40m.
func TestPingCadence(t *testing.T) {
	cases := []struct {
		name       string
		requested  time.Duration
		interval   int64
		want       time.Duration
		wantWindow time.Duration
		wantOK     bool
	}{
		{
			name:       "the interval the server chose",
			requested:  300 * time.Second,
			interval:   42,
			want:       42 * time.Second,
			wantWindow: 84 * time.Second,
			wantOK:     true,
		},
		{
			name:       "an interval the server raised to its own minimum",
			requested:  5 * time.Second,
			interval:   30,
			want:       30 * time.Second,
			wantWindow: time.Minute,
			wantOK:     true,
		},
		{name: "no interval at all", requested: 300 * time.Second, interval: 0},
		{name: "a negative interval", requested: 300 * time.Second, interval: -1},
		{name: "an interval no Duration can hold", requested: 300 * time.Second, interval: math.MaxInt64},
		{
			name:       "Stalwart's milliseconds, read as the seconds the RFC defines",
			requested:  30 * time.Second,
			interval:   30000,
			want:       30 * time.Second,
			wantWindow: time.Minute,
			wantOK:     true,
		},
		{
			name:       "an interval above what the client asked for",
			requested:  120 * time.Second,
			interval:   3600,
			want:       120 * time.Second,
			wantWindow: 240 * time.Second,
			wantOK:     true,
		},
		{
			// RFC 8620 section 7.3 caps a server's minimum at 30
			// seconds, so a client asking for less than that can still
			// be given 30 and nothing longer.
			name:       "an absurd interval against a client that asked for none",
			interval:   math.MaxInt64 / int64(time.Second),
			want:       30 * time.Second,
			wantWindow: time.Minute,
			wantOK:     true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ping{Interval: c.interval}.cadence(c.requested)
			if ok != c.wantOK {
				t.Fatalf("cadence usable = %v, want %v", ok, c.wantOK)
			}
			if got != c.want {
				t.Errorf("cadence = %v, want %v", got, c.want)
			}
			if window := stallWindow(got); window != c.wantWindow {
				t.Errorf("the stall window this cadence sets is %v, want %v", window, c.wantWindow)
			}
		})
	}
}

// TestStallWindow guards the quantity the detector is actually set
// from. Bounding the cadence alone leaves the doubling unbounded, and
// a wrapped window is negative, which fires the timer the instant it
// is set: every connection aborted on its first event, on a stream
// that was working.
func TestStallWindow(t *testing.T) {
	cases := []struct {
		name    string
		cadence time.Duration
		want    time.Duration
	}{
		{name: "no ping asked for", cadence: 0, want: 0},
		{name: "a negative cadence", cadence: -time.Second, want: 0},
		{name: "a plain cadence doubles", cadence: 30 * time.Second, want: time.Minute},
		{name: "the largest cadence that doubles cleanly", cadence: math.MaxInt64 / 2, want: math.MaxInt64 - 1},
		{name: "a cadence whose double overflows saturates", cadence: math.MaxInt64/2 + 1, want: math.MaxInt64},
		{name: "the largest cadence a ping can advertise", cadence: math.MaxInt64, want: math.MaxInt64},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stallWindow(c.cadence)
			if got != c.want {
				t.Errorf("stallWindow(%v) = %v, want %v", c.cadence, got, c.want)
			}
			if got < 0 {
				t.Errorf("stallWindow(%v) = %v, which fires a timer at once", c.cadence, got)
			}
		})
	}
}

// TestListenSurvivesAnAbsurdPingInterval is the end of the same
// thread, on a live stream rather than on the arithmetic. The interval
// advertised here is the largest one a Duration can hold as seconds,
// and the clamp brings it back to the 300 the caller asked for. The
// server never goes silent, so the stream must stay up on one
// connection: a clamp that overshot the other way would abort it.
func TestListenSurvivesAnAbsurdPingInterval(t *testing.T) {
	client, requests := startEventFake(t, func(_ int, w *eventWriter) {
		w.send("event: ping\ndata: {\"interval\":9223372036}\n\n")
		w.send(stateEventBody("s1"))
		w.hold()
	})
	dial(t, client)

	var tr tracker
	stop := listen(t, client, tr.source(EventSource{Ping: 300 * time.Second}))
	waitFor(t, "the state event behind the ping", func() bool { return len(tr.changesSeen()) > 0 })
	time.Sleep(500 * time.Millisecond)
	_ = stop()

	if got := len(requests()); got != 1 {
		t.Errorf("made %d connections to a server that never went silent, want 1", got)
	}
	if got := tr.disconnectsSeen(); len(got) != 0 {
		t.Errorf("reported %d drops, want none: %v", len(got), got)
	}
}

// TestListenReportsAClampedPingInterval is what a caller can see of
// the clamp. Package jmap logs nothing, so without this report a
// server whose advertised cadence poplar overrides looks
// indistinguishable from one poplar agreed with, and the operator
// chasing a stream that feels dead has no evidence either way. The
// first stream advertises Stalwart's milliseconds against a client
// asking for 300 seconds, so the cadence in force is 300; the second
// advertises a figure the RFC allows, so nothing is reported.
func TestListenReportsAClampedPingInterval(t *testing.T) {
	cases := []struct {
		name     string
		interval string
		want     []clampReport
	}{
		{
			name:     "an interval above what the client asked for",
			interval: "30000",
			want:     []clampReport{{reported: 30000 * time.Second, inForce: 300 * time.Second}},
		},
		{
			name:     "an interval the RFC allows",
			interval: "42",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, _ := startEventFake(t, func(_ int, w *eventWriter) {
				w.send("event: ping\ndata: {\"interval\":" + c.interval + "}\n\n")
				w.send(stateEventBody("s1"))
				w.hold()
			})
			dial(t, client)

			var mu sync.Mutex
			var got []clampReport
			var tr tracker
			source := tr.source(EventSource{Ping: 300 * time.Second})
			source.OnPingClamped = func(reported, inForce time.Duration) {
				mu.Lock()
				defer mu.Unlock()
				got = append(got, clampReport{reported: reported, inForce: inForce})
			}

			stop := listen(t, client, source)
			waitFor(t, "the state event behind the ping", func() bool { return len(tr.changesSeen()) > 0 })
			_ = stop()

			mu.Lock()
			defer mu.Unlock()
			if !slices.Equal(got, c.want) {
				t.Errorf("clamp reports = %v, want %v", got, c.want)
			}
		})
	}
}

// A clampReport is one OnPingClamped call's two arguments.
type clampReport struct {
	reported time.Duration
	inForce  time.Duration
}

// TestListenStopsAfterOneStateEvent covers JT-27. RFC 8620 section 7.3
// gives closeafter=state for a client that wants one notification and
// no connection, and a client that reconnected after it would hold the
// stream open forever against a server that keeps closing it.
func TestListenStopsAfterOneStateEvent(t *testing.T) {
	client, requests := startEventFake(t, func(_ int, w *eventWriter) {
		w.send(stateEventBody("s1"))
		w.hold()
	})
	dial(t, client)

	var tr tracker
	if err := listenOnce(t, client, tr.source(EventSource{CloseAfterState: true})); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if len(tr.changesSeen()) != 1 {
		t.Errorf("delivered %d changes, want exactly 1", len(tr.changesSeen()))
	}
	if len(requests()) != 1 {
		t.Errorf("made %d connections, want exactly 1", len(requests()))
	}
}

// TestListenHonoursTheContext covers half of JT-25. go-jmap's Listen
// took no context, so the only way to stop a stream was to close the
// body out from under the reading goroutine.
func TestListenHonoursTheContext(t *testing.T) {
	client, requests := startEventFake(t, func(_ int, w *eventWriter) { w.hold() })
	dial(t, client)

	var tr tracker
	stop := listen(t, client, tr.source(EventSource{}))
	waitFor(t, "the first connection", func() bool { return len(requests()) > 0 })

	if err := stop(); !errors.Is(err, context.Canceled) {
		t.Errorf("Listen error = %v, want context.Canceled", err)
	}
}

// closeReporter wraps a transport so every response body reports an
// error when closed, which no server can be made to do.
type closeReporter struct {
	base http.RoundTripper
	err  error
}

func (c closeReporter) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.base.RoundTrip(req)
	if resp != nil {
		resp.Body = reportingBody{ReadCloser: resp.Body, err: c.err}
	}
	return resp, err
}

type reportingBody struct {
	io.ReadCloser
	err error
}

func (b reportingBody) Close() error {
	_ = b.ReadCloser.Close()
	return b.err
}

// TestListenSurfacesTheCloseError covers the rest of JT-25. go-jmap's
// Close dropped the body's error on the floor, so a stream that failed
// to release its connection looked like a clean shutdown.
func TestListenSurfacesTheCloseError(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("GET /session", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, sessionTemplate, srv.URL, "s0")
	})
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, stateEventBody("s1"))
	})

	stubborn := errors.New("release the connection")
	client := NewClient(srv.URL+"/session", &http.Client{
		Transport: closeReporter{base: srv.Client().Transport, err: stubborn},
	})
	dial(t, client)

	var tr tracker
	err := listenOnce(t, client, tr.source(EventSource{CloseAfterState: true}))
	if !errors.Is(err, stubborn) {
		t.Errorf("Listen error = %v, want the body's close error", err)
	}
}

// TestListenStopsOnAServerRefusal draws the line the reconnect loop
// respects. A transport failure is the protocol's to retry; a server
// that answered and said no is poplar's to decide about, and a client
// that reconnected through an expired token would hammer the server
// with a credential it already knows is dead.
func TestListenStopsOnAServerRefusal(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		check   func(*testing.T, error)
	}{
		{
			name: "problem details",
			handler: serveJSON(http.StatusUnauthorized, "application/problem+json",
				`{"type":"about:blank","detail":"token expired"}`),
			check: func(t *testing.T, err error) {
				var reqErr *RequestError
				if !errors.As(err, &reqErr) || reqErr.Detail != "token expired" {
					t.Errorf("Listen error = %v (%T), want the server's problem details", err, err)
				}
			},
		},
		{
			name:    "a bare status",
			handler: serveJSON(http.StatusServiceUnavailable, "text/html", "<html>down</html>"),
			check: func(t *testing.T, err error) {
				var httpErr *HTTPError
				if !errors.As(err, &httpErr) || httpErr.Status != http.StatusServiceUnavailable {
					t.Errorf("Listen error = %v (%T), want a 503 HTTPError", err, err)
				}
			},
		},
		{
			name:    "a body that is not an event stream",
			handler: serveJSON(http.StatusOK, "text/html", "<html>sign in</html>"),
			check: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "text/event-stream") {
					t.Errorf("Listen error = %v, want it to name the content type it wanted", err)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, mux := startFake(t)
			var mu sync.Mutex
			var attempts int
			mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				attempts++
				mu.Unlock()
				c.handler(w, r)
			})
			dial(t, client)

			var tr tracker
			c.check(t, listenOnce(t, client, tr.source(EventSource{})))

			mu.Lock()
			defer mu.Unlock()
			if attempts != 1 {
				t.Errorf("made %d attempts, want exactly 1; a refusal is not the protocol's to retry", attempts)
			}
		})
	}
}

// TestConnectTimesTheOpenStream pins what the backoff schedule
// measures, from both ends. Timing from before the dial counts a
// black-holing server's connect timeout as time connected, so every
// attempt looks healthy and the schedule never escalates. Reporting
// nothing is the same defect from the other side: no connection ever
// looks healthy, so a drop after hours of a good stream waits the
// fully escalated delay the reset exists to avoid.
//
// The two rows are the same server spending its time in the two
// places it can: before the stream opens, and with the stream open.
func TestConnectTimesTheOpenStream(t *testing.T) {
	const spent = 300 * time.Millisecond
	cases := []struct {
		name string

		// handler takes the test so the second case can fail loudly
		// on a ResponseWriter it cannot flush. Without the flush the
		// client waits for the handler to return, and the stream is
		// never open while the server holds it.
		handler  func(*testing.T) http.HandlerFunc
		wantLess bool
	}{
		{
			name: "time spent answering is not time connected",
			handler: func(*testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					time.Sleep(spent)
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
				}
			},
			wantLess: true,
		},
		{
			name: "time spent with the stream open is",
			handler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					flusher, ok := w.(http.Flusher)
					if !ok {
						t.Error("the test server's ResponseWriter cannot flush")
						return
					}
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					flusher.Flush()
					time.Sleep(spent)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, mux := startFake(t)
			mux.HandleFunc("GET /events", c.handler(t))
			session := dial(t, client)

			l := &listener{
				client: client,
				url: expandTemplate(session.EventSourceURL,
					"{types}", "*", "{closeafter}", "no", "{ping}", "0"),
			}
			uptime, retry, err := l.connect(t.Context())
			if err != nil {
				t.Fatalf("connect: %v", err)
			}
			if !retry {
				t.Fatal("a stream the server closed is not a refusal")
			}
			if uptime < 0 {
				t.Fatalf("uptime = %v", uptime)
			}
			if c.wantLess && uptime >= spent {
				t.Errorf("uptime = %v, want under the %v the server spent before the stream opened", uptime, spent)
			}
			if !c.wantLess && uptime < spent {
				t.Errorf("uptime = %v, want at least the %v the stream stayed open", uptime, spent)
			}
		})
	}
}

// TestListenRefusesWithoutASession keeps the push stream on the same
// footing as every other call: the URL comes out of the session, so
// there is nowhere to connect before one is in hand.
func TestListenRefusesWithoutASession(t *testing.T) {
	client, _ := startFake(t)
	if err := client.Listen(t.Context(), EventSource{}); !errors.Is(err, ErrNoSession) {
		t.Errorf("Listen error = %v, want ErrNoSession", err)
	}
}

// TestReconnectBackoffSchedule covers JT-25's bounded backoff. The
// reset row is the one that matters: without it the counter only ever
// grows, so a single drop after hours of health waits the same
// escalated delay as the sixth drop in a row, and ADR-0005's 30s p95
// push recovery goes with it.
func TestReconnectBackoffSchedule(t *testing.T) {
	var b backoff
	steps := []struct {
		connectedFor time.Duration
		want         time.Duration
	}{
		{connectedFor: 0, want: reconnectMin},
		{connectedFor: time.Millisecond, want: 2 * reconnectMin},
		{connectedFor: time.Second, want: 4 * reconnectMin},
		{connectedFor: 0, want: 8 * reconnectMin},
		{connectedFor: reconnectMax - time.Millisecond, want: 16 * reconnectMin},
		{connectedFor: reconnectMax, want: reconnectMin},
		{connectedFor: 0, want: 2 * reconnectMin},
	}
	for i, step := range steps {
		if got := b.bound(step.connectedFor); got != step.want {
			t.Errorf("step %d after %v connected: bound = %v, want %v", i, step.connectedFor, got, step.want)
		}
	}

	var capped backoff
	var last time.Duration
	for range 20 {
		last = capped.bound(0)
		if last > reconnectMax {
			t.Fatalf("bound = %v, want no more than %v", last, reconnectMax)
		}
	}
	if last != reconnectMax {
		t.Errorf("bound after 20 failures = %v, want it held at %v", last, reconnectMax)
	}
}

// TestJitterStaysUnderItsBound proves the delay a bound produces is
// both random and inside it. A jitter that could return the bound
// itself would let every stream in a fleet reconnect in lockstep.
func TestJitterStaysUnderItsBound(t *testing.T) {
	const bound = time.Second
	seen := make(map[time.Duration]bool)
	for range 100 {
		d := jitter(bound)
		if d < 0 || d >= bound {
			t.Fatalf("jitter = %v, want it in [0, %v)", d, bound)
		}
		seen[d] = true
	}
	if len(seen) < 2 {
		t.Error("jitter returned one value 100 times, so the delay is not jittered at all")
	}
}
