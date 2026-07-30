package jmap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// An EventSource is a subscription to the server's push stream (RFC
// 8620 section 7.3), passed to [Client.Listen]. It is not section
// 7.2's PushSubscription object, which asks a server to post to a URL
// of the client's and which this package does not model.
//
// The three callbacks run on Listen's own goroutine, in the order the
// stream produced them, so a slow one stalls the stream. A nil one is
// skipped.
type EventSource struct {
	// Types names the record types to subscribe to. Empty subscribes
	// to every type the server offers.
	Types []EventType

	// Ping asks the server to send a ping event on this cadence, which
	// it may clamp and reports back in the event's payload. Silence
	// past twice the cadence in force counts as a dropped connection.
	// Zero asks for no ping and leaves the stream with no liveness
	// check at all.
	//
	// This is also the ceiling on the cadence a reported interval can
	// establish, or 30 seconds when it is shorter than that, since a
	// server has no room under the RFC to return anything longer.
	Ping time.Duration

	// CloseAfterState ends the stream after one state event, RFC 8620
	// section 7.3's one-shot mode. Listen returns rather than
	// reconnecting.
	CloseAfterState bool

	// OnConnect reports that a connection is open, once per connection
	// and so once per reconnect. Every one of them is a gap: the
	// stream says nothing about what happened while it was down, and
	// resumption is a courtesy no server owes (ADR-0018). Treat it as
	// the moment to pull /changes.
	OnConnect func()

	// OnChange reports one state event.
	OnChange func(*StateChange)

	// OnDisconnect reports a connection lost, before the delay ahead
	// of the next one. A nil error means the server closed the stream
	// without saying anything was wrong.
	OnDisconnect func(error)

	// OnPingClamped reports a ping event advertising an interval
	// above what RFC 8620 section 7.3 permits: what the server said,
	// and the cadence held to instead (cadence's own doc comment). A
	// conformant server never triggers it, and a clamping one
	// triggers it on every ping, so a caller that logs this dedups
	// what it logs. This package logs nothing, so a stream whose
	// liveness expectation is not the advertised one is otherwise
	// indistinguishable from one that is.
	OnPingClamped func(reported, inForce time.Duration)
}

// A ping is the payload of RFC 8620 section 7.3's ping event, which
// reports the cadence the server settled on rather than the one the
// client asked for.
type ping struct {
	// Interval is the cadence in seconds.
	Interval int64 `json:"interval"`
}

// pingMinimumCeiling is the highest minimum RFC 8620 section 7.3 lets
// a server impose on a requested ping interval, and so the highest
// cadence a client that asked for less can be given.
const pingMinimumCeiling = 30 * time.Second

// cadence returns the interval to expect the next event within, given
// the interval the client requested, and whether it is usable. A ping
// carries whatever the server put in it, and neither a figure at or
// below zero nor one too large for a Duration to hold says anything
// about how often to expect the next one, so both leave the cadence
// already in force alone.
//
// A figure above what the RFC allows is clamped rather than believed.
// Section 7.3 lets a server hold a requested interval to its own
// minimum and maximum and forbids a minimum above 30 seconds, so a
// conformant server never reports more than the client asked for, or
// 30 seconds when the client asked for less. Believing a larger one
// costs the liveness check outright: the stall window is twice the
// cadence, and Stalwart reports 30000 while pinging every 30 seconds,
// which taken as seconds is a window of 16h40m. ADR-0005 puts push
// recovery at 30 seconds p95, and a dropped connection nobody notices
// has no recovery time at all.
func (p ping) cadence(requested time.Duration) (time.Duration, bool) {
	if p.Interval <= 0 || p.Interval > int64(math.MaxInt64/time.Second) {
		return 0, false
	}
	return min(time.Duration(p.Interval)*time.Second, max(requested, pingMinimumCeiling)), true
}

// stallWindow returns how long a stream may say nothing before it
// counts as dropped, twice the ping cadence in force (ADR-0005
// revision 2). A cadence at or below zero returns zero, which is the
// caller asking for no liveness check.
//
// A cadence whose double no Duration can hold saturates instead of
// wrapping. This is the quantity the timer is set from, so a wrapped
// negative window fires the detector the instant it is set, and a
// server advertising an absurd interval would then have every one of
// its connections aborted on its first event.
func stallWindow(cadence time.Duration) time.Duration {
	switch {
	case cadence <= 0:
		return 0
	case cadence > math.MaxInt64/2:
		return math.MaxInt64
	}
	return 2 * cadence
}

const (
	stateEvent      = "state"
	pingEvent       = "ping"
	eventStreamType = "text/event-stream"
)

// Listen streams push notifications from the session's event source
// until the caller cancels ctx, the server refuses the connection, or
// a one-shot stream ends. It returns ctx.Err() for the first, the
// server's error for the second, and nil for the third.
//
// A connection lost to the transport is not a failure Listen reports
// by returning. The server-sent events standard makes reconnecting
// part of the protocol, so Listen reconnects on a bounded jittered
// schedule and resends the last event id it saw, and the caller hears
// about each drop and each new connection through the callbacks. What
// Listen never decides is when to stop trying or when to poll
// instead: that is policy, and it stays with the caller, which is
// also why a server that answered and refused ends the call rather
// than starting a retry loop against a credential the server has
// already rejected.
func (c *Client) Listen(ctx context.Context, source EventSource) error {
	session := c.Session()
	if session == nil {
		return ErrNoSession
	}

	l := &listener{
		client: c,
		source: source,
		url: expandTemplate(session.EventSourceURL,
			"{types}", subscribedTypes(source.Types),
			"{closeafter}", closeAfter(source.CloseAfterState),
			"{ping}", strconv.FormatInt(int64(source.Ping/time.Second), 10),
		),
	}

	var schedule backoff
	for {
		uptime, retry, err := l.connect(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !retry {
			return err
		}
		if source.OnDisconnect != nil {
			source.OnDisconnect(err)
		}
		if !sleep(ctx, jitter(schedule.bound(uptime))) {
			return ctx.Err()
		}
	}
}

// A listener holds one Listen call's state across the connections it
// makes. lastID is the resume point, which outlives the connection
// that produced it.
type listener struct {
	client *Client
	source EventSource
	url    string
	lastID string
}

// connect makes one connection and reads it to its end. It reports how
// long the connection stayed open, zero when it never opened at all,
// and whether the failure is one a reconnect could fix.
func (l *listener) connect(ctx context.Context) (time.Duration, bool, error) {
	// The stall detector cancels this context rather than the caller's,
	// so a silent server drops one connection instead of ending Listen.
	connCtx, abort := context.WithCancel(ctx)
	defer abort()

	req, err := http.NewRequestWithContext(connCtx, http.MethodGet, l.url, nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("Accept", eventStreamType)
	if l.lastID != "" {
		req.Header.Set("Last-Event-ID", l.lastID)
	}

	resp, err := l.client.httpClient.Do(req)
	if err != nil {
		return 0, true, err
	}
	if err := refusal(resp); err != nil {
		return 0, false, errors.Join(err, resp.Body.Close())
	}
	if err := checkEventStream(resp.Header.Get("Content-Type")); err != nil {
		return 0, false, errors.Join(err, resp.Body.Close())
	}

	// The clock starts where the stream does. Timing from before the
	// dial would count a black-holing server's connect timeout as time
	// connected, which resets the backoff on every attempt and leaves
	// the schedule flat.
	openedAt := time.Now()
	if l.source.OnConnect != nil {
		l.source.OnConnect()
	}
	retry, err := l.consume(abort, resp.Body)
	return time.Since(openedAt), retry, errors.Join(err, resp.Body.Close())
}

// consume reads one open connection to its end. abort is the stall
// detector's only lever: cancelling the connection's context unblocks
// the read that is waiting on a server which has gone quiet.
func (l *listener) consume(abort context.CancelFunc, body io.Reader) (bool, error) {
	reader := newEventReader(body)
	reader.idBuffer, reader.lastID = l.lastID, l.lastID
	defer func() { l.lastID = reader.lastID }()

	window := stallWindow(l.source.Ping)
	var stall *time.Timer
	if window > 0 {
		stall = time.NewTimer(window)
		defer stall.Stop()
		done := make(chan struct{})
		defer close(done)
		go func() {
			select {
			case <-stall.C:
				abort()
			case <-done:
			}
		}()
	}

	for {
		ev, err := reader.next()
		if errors.Is(err, io.EOF) {
			return true, nil
		}
		if err != nil {
			return true, err
		}

		switch ev.name {
		case stateEvent:
			change := &StateChange{}
			if err := json.Unmarshal([]byte(ev.data), change); err != nil {
				return true, fmt.Errorf("decode state event: %v", err)
			}
			if l.source.OnChange != nil {
				l.source.OnChange(change)
			}
			if l.source.CloseAfterState {
				return false, nil
			}
		case pingEvent:
			p := ping{}
			if err := json.Unmarshal([]byte(ev.data), &p); err != nil {
				return true, fmt.Errorf("decode ping event: %v", err)
			}
			if cadence, ok := p.cadence(l.source.Ping); ok {
				// cadence reporting ok is what bounds this product:
				// it rejects an interval too large for a Duration to
				// hold as seconds, so the only figures reaching here
				// convert without wrapping.
				if reported := time.Duration(p.Interval) * time.Second; reported > cadence && l.source.OnPingClamped != nil {
					l.source.OnPingClamped(reported, cadence)
				}
				window = stallWindow(cadence)
			}
		}

		if stall != nil {
			// Reset after the switch, so a ping that clamped the
			// cadence is already in window. Go 1.23 retired the
			// drain-then-reset idiom: Stop is enough, and the old
			// receive after a false Stop can now block forever.
			stall.Stop()
			stall.Reset(window)
		}
	}
}

// checkEventStream fails when the server answered with something that
// is not a stream. A 200 carrying a sign-in page parses as a stream
// that never dispatches anything, which is indistinguishable from a
// healthy but quiet server.
func checkEventStream(contentType string) error {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != eventStreamType {
		return fmt.Errorf("event source answered with %q, want %s", contentType, eventStreamType)
	}
	return nil
}

func subscribedTypes(types []EventType) string {
	if len(types) == 0 {
		return string(AllEvents)
	}
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return strings.Join(names, ",")
}

func closeAfter(oneShot bool) string {
	if oneShot {
		return "state"
	}
	return "no"
}

const (
	reconnectMin = 250 * time.Millisecond
	reconnectMax = 30 * time.Second
)

// A backoff is the reconnection schedule across one run of drops.
type backoff struct {
	attempt int
}

// bound returns the ceiling on the delay before the next attempt,
// doubling per consecutive failure up to reconnectMax.
//
// A connection that stayed up at least reconnectMax proved itself, so
// its drop starts the schedule over. Without that, the count only ever
// grows for the life of a Listen call: it saturates after a handful of
// drops, and one drop after hours of health then waits the same
// escalated delay as the sixth drop in a row, which is the wrong side
// of ADR-0005's 30s push recovery bound.
func (b *backoff) bound(connectedFor time.Duration) time.Duration {
	if connectedFor >= reconnectMax {
		b.attempt = 0
	}
	d := reconnectMin
	for range b.attempt {
		d = min(d*2, reconnectMax)
	}
	b.attempt++
	return d
}

// jitter returns a delay uniformly distributed below bound, so a
// server coming back up does not meet every client it dropped at the
// same instant.
func jitter(bound time.Duration) time.Duration {
	return time.Duration(rand.Int64N(int64(bound))) //nolint:gosec // G404: jitter timing, not a security decision
}

// sleep waits d and reports whether it finished; false means ctx ended
// first.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
