package mailimap

import (
	"context"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

const (
	idleRefreshInterval = 9 * time.Minute // under Gmail's 10-min cap
	defaultPollFallback = 60 * time.Second
	reconnectInitial    = 1 * time.Second
	reconnectMax        = 30 * time.Second
)

// expBackoff returns initial * 2^(attempts-1) capped at max.
// attempts <= 1 returns initial.
func expBackoff(attempts int, initial, max time.Duration) time.Duration {
	if attempts <= 1 {
		return initial
	}
	d := initial << (attempts - 1)
	if d > max {
		return max
	}
	return d
}

// idleLoop runs until ctx is cancelled, redialing on session error
// with exponential backoff. Clean refresh returns reuse the existing
// handle.
func (b *Backend) idleLoop(ctx context.Context) {
	defer close(b.idleDone)

	attempts := 0
	for {
		if ctx.Err() != nil {
			return
		}
		b.mu.Lock()
		haveIdle := b.idle != nil
		b.mu.Unlock()
		if !haveIdle {
			if err := b.dialIdle(ctx); err != nil {
				attempts++
				delay := expBackoff(attempts, reconnectInitial, reconnectMax)
				b.log.Warn("imap idle redial failed", "attempt", attempts, "delay", delay, "err", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
				}
				continue
			}
		}
		err := b.runIdleSession(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			attempts = 0
			continue
		}
		b.emit(mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnReconnecting})
		attempts++
		delay := expBackoff(attempts, reconnectInitial, reconnectMax)
		b.log.Warn("imap idle session ended, will redial",
			"attempt", attempts, "delay", delay, "err", err)
		b.dropIdle()
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// runIdleSession selects the current folder, runs IDLE or poll, and
// listens for folder-switch signals. Returns nil on clean refresh-
// cycle completion (caller re-loops) or an error on connection failure.
func (b *Backend) runIdleSession(ctx context.Context) error {
	b.mu.Lock()
	idle := b.idle
	current := b.current
	switchCh := b.switchCh
	hasIDLE := b.caps.IDLE
	b.mu.Unlock()

	if current == "" {
		// Wait for the first OpenFolder.
		select {
		case <-ctx.Done():
			return nil
		case f := <-switchCh:
			b.mu.Lock()
			b.current = f
			current = f
			b.mu.Unlock()
		}
	}

	if _, err := idle.Select(current, true); err != nil {
		return err
	}

	b.emit(mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnConnected})

	if !hasIDLE {
		return b.pollLoop(ctx, current, switchCh)
	}

	timer := time.NewTimer(idleRefreshInterval)
	defer timer.Stop()

	idleErrCh := make(chan error, 1)
	go func() {
		idleErrCh <- idle.Idle(b.emit)
	}()

	for {
		select {
		case <-ctx.Done():
			idle.IdleStop()
			<-idleErrCh
			return nil
		case <-timer.C:
			idle.IdleStop()
			if err := <-idleErrCh; err != nil {
				return err
			}
			return nil
		case f := <-switchCh:
			idle.IdleStop()
			if err := <-idleErrCh; err != nil {
				return err
			}
			b.mu.Lock()
			b.current = f
			b.mu.Unlock()
			return nil
		case err := <-idleErrCh:
			return err
		}
	}
}

// pollLoop is the IDLE-less fallback. UpdateFolderInfo fires every
// pollFallbackInterval. The UI re-fetches on receipt.
func (b *Backend) pollLoop(ctx context.Context, folder string, switchCh chan string) error {
	interval := b.pollInterval
	if interval == 0 {
		interval = defaultPollFallback
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case f := <-switchCh:
			b.mu.Lock()
			b.current = f
			b.mu.Unlock()
			return nil
		case <-t.C:
			b.emit(mail.Update{Type: mail.UpdateFolderInfo, Folder: folder})
		}
	}
}

// emit drops u when the buffer is full.
func (b *Backend) emit(u mail.Update) {
	b.mu.Lock()
	ch := b.updates
	b.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- u:
	default:
		b.log.Warn("dropped update, channel full", "type", u.Type, "cap", cap(ch))
	}
}
