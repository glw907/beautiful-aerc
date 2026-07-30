package jmapsource

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
)

// eventStream serves the event source, running script once per
// connection. hold blocks the handler until the client hangs up, which
// is how a real stream stays open; returning instead drops it.
type eventStream struct {
	mu          sync.Mutex
	connections int
	script      func(n int, send func(string), hold func())
}

func (e *eventStream) handle(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	e.connections++
	n := e.connections
	e.mu.Unlock()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no flusher", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	send := func(raw string) {
		_, _ = fmt.Fprint(w, raw)
		flusher.Flush()
	}
	e.script(n, send, func() { <-r.Context().Done() })
}

func (e *eventStream) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.connections
}

// stateEvent is one RFC 8620 section 7.1.1 push naming u1, the account
// sessionTemplate's primaryAccounts assigns mail.
func stateEvent(state string) string {
	return fmt.Sprintf("event: state\ndata: {\"@type\":\"StateChange\",\"changed\":{\"u1\":{\"Email\":%q}}}\n\n", state)
}

// newPushSession dials a Session against a server whose event source
// runs script, and returns the session and the stream behind it.
func newPushSession(t *testing.T, script func(n int, send func(string), hold func())) (*Session, *eventStream) {
	t.Helper()

	stream := &eventStream{script: script}
	mux, srv := newFakeServer(t)
	mux.HandleFunc("/events", stream.handle)
	return dialTestSession(t, srv), stream
}

// gate returns a channel a scripted connection blocks on and the func
// that releases it, safe to call more than once. A test registers that
// func as cleanup after dialing, so it runs before the server closes:
// httptest.Server.Close waits out its handlers, and one still blocked
// on the gate hangs the suite there rather than reporting the failure
// that got it stuck.
func gate() (<-chan struct{}, func()) {
	ch := make(chan struct{})
	var once sync.Once
	return ch, func() { once.Do(func() { close(ch) }) }
}

// listenPush opens the transport, failing t when it will not open, and
// stops it on cleanup.
func listenPush(t *testing.T, session *Session) <-chan backend.Notification {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	ch, err := session.Push().Listen(ctx)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	return ch
}

// nextNotification returns the next notification on ch, failing t when
// none arrives or when the channel closed instead.
func nextNotification(t *testing.T, ch <-chan backend.Notification, what string) backend.Notification {
	t.Helper()

	select {
	case n, ok := <-ch:
		if !ok {
			t.Fatalf("the notification channel closed while waiting for %s", what)
		}
		return n
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		return backend.Notification{}
	}
}

// TestPushNotifiesOnConnect is ADR-0018's obligation on the adapter.
// The stream says nothing about what happened while it was down, and
// resumption is a courtesy no server owes, so a connection that
// replays nothing must still produce a notification. The sync engine
// flushes only on one, and without this a reconnect against a server
// that ignores Last-Event-ID drops the whole disconnected window in
// silence.
func TestPushNotifiesOnConnect(t *testing.T) {
	session, _ := newPushSession(t, func(_ int, _ func(string), hold func()) {
		hold() // a connection that replays nothing at all
	})

	ch := listenPush(t, session)
	if got := nextNotification(t, ch, "the connect notification"); got.Scope != "u1" {
		t.Errorf("Scope = %q, want the mail account id u1", got.Scope)
	}
}

// TestPushNotifiesOnStateChange covers the other source of one, and
// its filter: a state event for the account poplar syncs produces a
// notification, and one naming a different account does not. The
// script waits for the connect notification to be read first, since
// the channel holds one and a send that would block is dropped.
func TestPushNotifiesOnStateChange(t *testing.T) {
	proceed, open := gate()
	session, _ := newPushSession(t, func(_ int, send func(string), hold func()) {
		<-proceed
		send("event: state\ndata: {\"@type\":\"StateChange\",\"changed\":{\"other\":{\"Email\":\"x\"}}}\n\n")
		send(stateEvent("s1"))
		hold()
	})

	t.Cleanup(open)

	ch := listenPush(t, session)
	nextNotification(t, ch, "the connect notification")
	open()

	if got := nextNotification(t, ch, "the state change notification"); got.Scope != "u1" {
		t.Errorf("Scope = %q, want the mail account id u1", got.Scope)
	}
	select {
	case extra := <-ch:
		t.Errorf("a third notification arrived (%v): a change to another account produced one", extra)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestPushSurvivesADroppedStream is the reconnect-ownership ruling,
// demonstrated. The server drops the first connection; the transport
// reconnects on its own schedule and reports the new connection, and
// the caller's channel never closes, so the sync engine's own backoff
// never runs. Two schedules in series is what puts ADR-0005's 30s p95
// recovery bound out of reach.
func TestPushSurvivesADroppedStream(t *testing.T) {
	proceed, open := gate()
	session, stream := newPushSession(t, func(n int, _ func(string), hold func()) {
		if n == 1 {
			<-proceed
			return // a drop: the handler returns, closing the stream
		}
		hold()
	})

	t.Cleanup(open)

	ch := listenPush(t, session)
	nextNotification(t, ch, "the first connection's notification")
	open()
	nextNotification(t, ch, "the reconnection's notification")

	if got := stream.count(); got < 2 {
		t.Fatalf("connections = %d, want at least 2: the transport did not reconnect", got)
	}
	select {
	case _, ok := <-ch:
		if !ok {
			t.Fatal("the channel closed on a drop, which hands the reconnect back to the caller")
		}
	default:
	}
}

// TestPushClosesTheChannelWhenTheStreamStopsForGood is the other side
// of that contract. A refusal is not a drop: the transport stops, so
// the channel closes, which is the signal the caller's own backoff
// loop waits for.
func TestPushClosesTheChannelWhenTheStreamStopsForGood(t *testing.T) {
	stream := &eventStream{script: func(int, func(string), func()) {}}
	mux, srv := newFakeServer(t)
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if stream.count() == 0 {
			stream.handle(w, r) // opens, then drops at once
			return
		}
		http.Error(w, "no", http.StatusForbidden)
	})
	session := dialTestSession(t, srv)

	ch := listenPush(t, session)
	deadline := time.After(10 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("the channel stayed open after the server refused the reconnection")
		}
	}
}

// TestPushReportsARefusalWithoutLoggingIt pins how a rejected
// credential crosses the seam. The class has to cross it, or the one
// push failure waiting cannot fix reaches the user as a connectivity
// problem; and this package must not log it, because the caller
// retries Listen in its own backoff loop and one line per attempt is
// the flood ADR-0013 revision 2 forbids.
func TestPushReportsARefusalWithoutLoggingIt(t *testing.T) {
	buf := uerrtest.Capture(t)

	mux, srv := newFakeServer(t)
	mux.HandleFunc("/events", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	})
	session := dialTestSession(t, srv)

	ch, err := session.Push().Listen(t.Context())
	if ch != nil {
		t.Error("Listen returned a channel alongside an error")
	}
	var failure backend.Failure
	if !errors.As(err, &failure) {
		t.Fatalf("Listen error = %v, want a backend.Failure", err)
	}
	if failure.Class != uerr.ClassAuth {
		t.Errorf("Class = %v, want ClassAuth", failure.Class)
	}
	if lines := uerrtest.Lines(t, buf); len(lines) != 0 {
		t.Errorf("Listen wrote %d uerr lines, want none: the caller's retry loop owns the surfacing decision", len(lines))
	}
}

// TestPushIsAbsentWithoutAnEventSource keeps Push and Capabilities
// telling one story. A session advertising no event source has no push
// transport, and a non-nil one here would send the sync engine into a
// stream that cannot be opened instead of into the poll fallback.
func TestPushIsAbsentWithoutAnEventSource(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/session", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"capabilities":{},"accounts":{},"primaryAccounts":{},"apiUrl":"%s/api","state":"s1"}`, srv.URL)
	})
	session := dialTestSession(t, srv)

	if got := session.Capabilities().PushTransport; got != backend.PushTransportNone {
		t.Fatalf("PushTransport = %v, want PushTransportNone", got)
	}
	if got := session.Push(); got != nil {
		t.Errorf("Push() = %v, want nil to match the capability", got)
	}
}

// TestPushLogsAClampedPingIntervalOnce covers the clamp seam. Package
// jmap logs nothing, so a server whose advertised cadence poplar
// overrides is otherwise invisible to whoever is chasing a stream that
// feels dead; and a clamping server clamps on every ping, so the line
// is deduped rather than written once per ping for as long as the
// stream is up.
func TestPushLogsAClampedPingIntervalOnce(t *testing.T) {
	buf := captureDebugLog(t)

	proceed, open := gate()
	session, _ := newPushSession(t, func(_ int, send func(string), hold func()) {
		<-proceed
		// Stalwart's milliseconds, three times over, then a state event
		// to prove the stream ran on past them.
		for range 3 {
			send("event: ping\ndata: {\"interval\":30000}\n\n")
		}
		send(stateEvent("s1"))
		hold()
	})

	t.Cleanup(open)

	ch := listenPush(t, session)
	nextNotification(t, ch, "the connect notification")
	open()
	nextNotification(t, ch, "the state change behind the pings")

	var clamps []string
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if strings.Contains(line, "ping interval clamped") {
			clamps = append(clamps, line)
		}
	}
	if len(clamps) != 1 {
		t.Fatalf("clamp lines = %d across three clamping pings, want exactly 1: %v", len(clamps), clamps)
	}
	if !strings.Contains(clamps[0], "reported=8h20m0s") || !strings.Contains(clamps[0], "in_force=30s") {
		t.Errorf("clamp line = %q, want the advertised figure and the one held to", clamps[0])
	}
}

// captureDebugLog points slog's default logger at a buffer for t's
// duration. uerrtest.CaptureDefault would drop the line under test:
// its handler runs at slog's own default level, and the clamp is a
// debug-level report.
func captureDebugLog(t *testing.T) *logBuffer {
	t.Helper()

	buf := &logBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

// logBuffer is captureDebugLog's destination, guarded because the
// stream's own goroutine writes to it while the test goroutine reads.
type logBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestPushSurfacesAnUnreachableServerOnce is the failure the
// reconnect-ownership ruling moved rather than removed. A transport
// that owns its own retry never returns from Listen while it is still
// trying, so a server poplar cannot reach at all reaches the caller
// only through the drop reports, and it produces one per attempt, from
// 250ms apart up to 30s. One line for the outage, not one per attempt:
// undeduped that is thousands a day for one standing failure.
func TestPushSurfacesAnUnreachableServerOnce(t *testing.T) {
	buf := uerrtest.Capture(t)

	dead := httptest.NewServer(http.NewServeMux())
	deadURL := dead.URL
	dead.Close() // nothing listens on deadURL from here on

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/session", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"capabilities":{},"accounts":{},"primaryAccounts":{"urn:ietf:params:jmap:mail":"u1"},"apiUrl":%q,"eventSourceUrl":%q,"state":"s1"}`,
			srv.URL+"/api", deadURL+"/events")
	})
	session := dialTestSession(t, srv)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _, _ = session.Push().Listen(ctx) }()

	waitFor(t, "the outage to surface", func() bool { return len(uerrtest.Lines(t, buf)) > 0 })
	// Long enough for several more attempts on the transport's own
	// schedule, which starts at 250ms.
	time.Sleep(2 * time.Second)

	lines := uerrtest.Lines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("uerr lines = %d over a run of failed connections, want exactly 1: %v", len(lines), lines)
	}
	if got := lines[0]["class"]; got != "connection" {
		t.Errorf("class = %v, want connection", got)
	}
}

// TestPushLogsRecoveryAfterAnOutage closes the pair: the outage
// surfaced, so the connection that ends it is a state transition of
// its own and says so, rather than leaving the last word on the
// account a failure the transport has already fixed. The server hangs
// up without answering at all, which is what an unreachable server
// looks like to the client, rather than the refusal a status would be.
// It does so four times for two drops, because net/http retries an
// idempotent request itself when a connection is hung up on before the
// response, so server-side hangups and client-visible drops are not
// one for one.
func TestPushLogsRecoveryAfterAnOutage(t *testing.T) {
	buf := uerrtest.Capture(t)
	logs := captureDebugLog(t)

	stream := &eventStream{script: func(_ int, _ func(string), hold func()) { hold() }}
	mux, srv := newFakeServer(t)
	var mu sync.Mutex
	attempts := 0
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n > 4 {
			stream.handle(w, r)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("the test server's ResponseWriter cannot hijack")
			return
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		_ = conn.Close() // hang up with no response at all
	})
	session := dialTestSession(t, srv)

	ch := listenPush(t, session)
	nextNotification(t, ch, "the notification from the connection that recovered")

	if lines := uerrtest.Lines(t, buf); len(lines) != 1 {
		t.Fatalf("uerr lines = %d across a run of failed connections, want exactly 1: %v", len(lines), lines)
	}
	if !strings.Contains(logs.String(), "push reconnected") {
		t.Errorf("no recovery line after an outage that surfaced; log was:\n%s", logs.String())
	}
}

// TestPushDoesNotSurfaceARecoveredDrop is the other half. A stream the
// transport gets back in one attempt cost the user nothing, and
// reporting it would put a failure in front of them for a blip they
// never saw.
func TestPushDoesNotSurfaceARecoveredDrop(t *testing.T) {
	buf := uerrtest.Capture(t)

	proceed, open := gate()
	session, _ := newPushSession(t, func(n int, _ func(string), hold func()) {
		if n == 1 {
			<-proceed
			return
		}
		hold()
	})
	t.Cleanup(open)

	ch := listenPush(t, session)
	nextNotification(t, ch, "the first connection's notification")
	open()
	nextNotification(t, ch, "the reconnection's notification")

	if lines := uerrtest.Lines(t, buf); len(lines) != 0 {
		t.Errorf("a drop the transport recovered from in one attempt surfaced %d uerr lines, want none: %v", len(lines), lines)
	}
}

// waitFor blocks until cond holds, failing t rather than hanging the
// suite when it never does.
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

// TestPushReportsARefusalThatEndedAnOpenStream is the refusal that had
// nowhere to go. It arrives after Listen has already returned its
// channel, so returning it is not available; discarding it leaves the
// caller reopening a stream against a server that keeps refusing, at
// whatever flat rate the caller reopens at, and with nothing in the
// log. It is both the ground for waiting longer and the failure the
// user is owed, so the next Listen reports it before opening anything.
func TestPushReportsARefusalThatEndedAnOpenStream(t *testing.T) {
	buf := uerrtest.Capture(t)

	stream := &eventStream{script: func(int, func(string), func()) {}}
	mux, srv := newFakeServer(t)
	var mu sync.Mutex
	requests := 0
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		first := requests == 1
		mu.Unlock()
		if first {
			stream.handle(w, r) // opens, then the stream ends
			return
		}
		http.Error(w, "no", http.StatusUnauthorized)
	})
	hits := func() int {
		mu.Lock()
		defer mu.Unlock()
		return requests
	}
	session := dialTestSession(t, srv)
	push := session.Push()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ch, err := push.Listen(ctx)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	for range ch { // draining until the transport stops for good is the point
	}

	// The stream is over and the caller is about to reopen. That call
	// is where the refusal has to surface, and it needs no request of
	// its own to do it.
	before := hits()
	next, err := push.Listen(ctx)
	if next != nil {
		t.Error("Listen returned a channel alongside the refusal that ended the last stream")
	}
	var failure backend.Failure
	if !errors.As(err, &failure) {
		t.Fatalf("Listen error = %v, want the backend.Failure that ended the previous stream", err)
	}
	if failure.Class != uerr.ClassAuth {
		t.Errorf("Class = %v, want ClassAuth", failure.Class)
	}
	if got := hits(); got != before {
		t.Errorf("Listen made %d more requests before reporting a refusal it already held", got-before)
	}
	if lines := uerrtest.Lines(t, buf); len(lines) != 0 {
		t.Errorf("the adapter logged %d uerr lines for a refusal its caller surfaces", len(lines))
	}

	// And only once: the report is consumed, so the Listen after it
	// tries the server again rather than replaying history.
	if _, err := push.Listen(ctx); err == nil {
		t.Error("a Listen after the report opened a stream against a server that refuses")
	}
	if hits() <= before {
		t.Error("the Listen after the report never reached the server")
	}
}

// TestPushDropsANotificationRatherThanStallingTheStream pins the
// policy backend.Push states: a send the caller has not kept up with
// is dropped, not waited on. Waiting stalls the goroutine reading the
// stream, and a reader stalled past the ping cadence trips the
// transport's own stall detector and aborts a working connection,
// which a sync flush taking seconds is enough to cause. Nothing is
// lost, because the Changes call the queued notification triggers
// reads everything since the persisted token either way.
//
// The evidence has to come from the reader, not the server, which can
// write five events into a socket whether or not anyone is parsing
// them. The clamping ping behind them is that evidence: the line it
// produces is written by the reader itself, after it has been through
// every event in front of it.
func TestPushDropsANotificationRatherThanStallingTheStream(t *testing.T) {
	logs := captureDebugLog(t)

	proceed, open := gate()
	session, _ := newPushSession(t, func(_ int, send func(string), hold func()) {
		<-proceed
		for i := range 5 {
			send(stateEvent(fmt.Sprintf("s%d", i)))
		}
		send("event: ping\ndata: {\"interval\":30000}\n\n")
		hold()
	})
	t.Cleanup(open)

	// Never read the channel, so every notification after the first has
	// nowhere to go.
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	ch, err := session.Push().Listen(ctx)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	open()

	waitFor(t, "the reader to get past five undeliverable notifications", func() bool {
		return strings.Contains(logs.String(), "ping interval clamped")
	})

	// The stream is still live, and what it queued is still there.
	if len(ch) != cap(ch) {
		t.Errorf("queued notifications = %d, want the channel full at %d", len(ch), cap(ch))
	}
	select {
	case _, ok := <-ch:
		if !ok {
			t.Fatal("the channel closed while the caller was not reading it")
		}
	default:
		t.Fatal("nothing was queued for a caller that read nothing")
	}
}
