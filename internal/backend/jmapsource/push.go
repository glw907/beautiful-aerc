package jmapsource

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/jmap"
)

// pingInterval is the keepalive cadence Listen asks the event source
// for, and so, doubled, the window a connection may go quiet in before
// the transport treats it as dropped, until a ping reports the cadence
// the server settled on. RFC 8620 section 7.3 lets the
// server hold the request to its own minimum and maximum and report
// what it chose, and that figure is what governs (JT-26), so this is a
// request rather than a setting. Thirty seconds is the longest minimum
// the section lets a server impose, so asking for it accepts every
// cadence a conformant server can grant. It is also the figure
// sync.Config asked for before the transport took the liveness check
// over.
const pingInterval = 30 * time.Second

// emailEvent and mailboxEvent name the RFC 8621 record types the mail
// source syncs, and so the only two the push stream subscribes to. A
// server pushing anything else has nothing poplar would pull for.
const (
	emailEvent   jmap.EventType = "Email"
	mailboxEvent jmap.EventType = "Mailbox"
)

// Push returns s's push transport, or nil when the session advertises
// no event source: the same absence Capabilities reports as
// backend.PushTransportNone, which is what sends the sync engine to
// its poll fallback rather than into a stream that cannot be opened.
//
// One session has one transport, and successive calls return it, since
// what stopped the last stream is what the next Listen has to report.
func (s *Session) Push() backend.Push {
	if s.caps.PushTransport == backend.PushTransportNone {
		return nil
	}
	return &s.push
}

// pushSource is s's backend.Push over RFC 8620 section 7.3's event
// source. stopped carries why the last stream it opened ended, from
// the goroutine that watched it end to the next Listen call.
type pushSource struct {
	session *Session

	mu      sync.Mutex
	stopped error
}

// noteStop records why a stream ended, classified and never nil, so
// the next Listen has something to report even for a server that
// closed the stream without saying anything was wrong.
func (p *pushSource) noteStop(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = fmt.Errorf("jmap: push: %w", classifyListen(cmp.Or(err, errStreamClosed)))
}

// takeStop returns why the last stream ended and forgets it, so the
// call after the one that reported it opens a stream instead of
// replaying a refusal that is now history.
func (p *pushSource) takeStop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.stopped
	p.stopped = nil
	return err
}

// Listen opens the event stream and reports every connection and every
// state change on the returned channel. It returns once the stream is
// open, or with the refusal that ended a stream, whether the one it
// was opening or the one before it. That refusal is classified as a
// backend.Failure and not logged: RunPush retries Listen in its own
// backoff loop and owns the surfacing decision (ADR-0013 revision 2).
//
// A refusal that ends a stream already open has no other way out.
// Listen has returned by then, so the call after it reports the
// refusal before opening anything, which is what lets the caller both
// wait longer before the next stream and tell the user why mail
// stopped. Against a server refusing every other connection, throwing
// it away instead leaves the caller reopening at its flat rate with
// nothing in the log.
//
// A server that cannot be reached is neither. Listen goes on trying at
// its own bounded pace and does not return, so an outage reaches its
// caller through the log instead (pushHealth).
//
// The channel survives a drop, because the transport under it
// reconnects on its own bounded schedule, and closes only when the
// stream stops for good: the server refusing a connection, or ctx
// ending.
func (p *pushSource) Listen(ctx context.Context) (<-chan backend.Notification, error) {
	if err := p.takeStop(); err != nil {
		return nil, err
	}

	// One slot, and a send that gives up rather than waits. The
	// callbacks run on the stream's own reading goroutine, so one that
	// blocks stalls the reader behind it, and a reader that stalls past
	// the ping cadence trips the transport's stall detector and aborts a
	// working connection. Dropping a notification because one is already
	// queued costs nothing: the flush it would have triggered reads
	// every change since the persisted token anyway.
	ch := make(chan backend.Notification, 1)
	notify := func() {
		select {
		case ch <- backend.Notification{Scope: string(p.session.accountID)}:
		default:
		}
	}

	opened := make(chan struct{})
	var once sync.Once
	stopped := make(chan struct{})

	var health pushHealth
	source := jmap.EventSource{
		Types: []jmap.EventType{emailEvent, mailboxEvent},
		Ping:  pingInterval,
		OnConnect: func() {
			once.Do(func() { close(opened) })
			health.connected()
			// ADR-0018: every connection is a gap, and the stream says
			// nothing about what happened while it was down, so each one
			// is a reason to pull Changes from the persisted token.
			notify()
		},
		OnChange: func(change *jmap.StateChange) {
			if _, ok := change.Changed[p.session.accountID]; ok {
				notify()
			}
		},
		OnDisconnect: health.dropped,
		OnPingClamped: func(reported, inForce time.Duration) {
			// Debug rather than a uerr.Error: an overridden cadence is
			// not a failure and the stream it protects is working. Not
			// silence either, because package jmap logs nothing and the
			// cadence in force is what decides how long a dead
			// connection goes unnoticed. It reports the first clamp and
			// then only a change in the advertised figure, so a server
			// that clamps every ping is one line, not one a ping.
			slog.Debug("jmap: push ping interval clamped", "reported", reported, "in_force", inForce)
		},
	}

	go func() {
		// The stop is recorded before either signal, so the report is
		// in hand by the time anyone can observe the stream ending.
		p.noteStop(p.session.client.Listen(ctx, source))
		close(ch)
		close(stopped)
	}()

	select {
	case <-opened:
		return ch, nil
	case <-stopped:
		return nil, p.takeStop()
	}
}

// errStreamClosed stands in when a stream ended with nothing said
// about why, so a report of it names a cause either way.
var errStreamClosed = errors.New("event source closed the stream")

// outageDrops is how many drops with no connection between them count
// as an outage rather than a stream the transport is about to get
// back.
const outageDrops = 2

// pushHealth counts one Listen call's consecutive drops, so an outage
// surfaces the way ADR-0013 revision 2 asks: once, on the transition,
// not once per attempt.
//
// A transport that owns its own retry never returns from Listen while
// it is still trying, so a server that cannot be reached at all
// reaches a caller only through these reports, and it produces one on
// every attempt, on a schedule that starts under 250ms and tops out at
// 30s. The first drop is not yet an outage: the transport reconnects
// on its own bounded schedule, and a stream it gets back in one
// attempt cost the user nothing. A second drop with no connection
// between them is the moment the sync engine's own reconnect loop used
// to classify and surface, back when it owned the retry, and it means
// the attempt to get back on failed too.
//
// A server that keeps accepting a connection and dropping it therefore
// never surfaces, by design: every one of those connections is a
// notification, so poplar pulls Changes on each and mail keeps
// arriving. That is push degraded to polling, not an outage.
//
// The callbacks all run on Listen's own reading goroutine, so this
// needs no lock.
type pushHealth struct {
	drops int
}

// dropped records a lost connection, reporting the first at debug
// level and the second, which is the outage, through uerr.
func (h *pushHealth) dropped(err error) {
	h.drops++
	switch h.drops {
	case 1:
		slog.Debug("jmap: push connection lost, the transport is reconnecting", "error", err)
	case outageDrops:
		_ = uerr.New("jmap.push", nil, uerr.ClassConnection, cmp.Or(err, errStreamClosed))
	}
}

// connected records a connection made, ending whatever run of drops
// came before it and logging the recovery when that run surfaced.
func (h *pushHealth) connected() {
	if h.drops >= outageDrops {
		slog.Info("jmap: push reconnected", "drops", h.drops)
	}
	h.drops = 0
}
