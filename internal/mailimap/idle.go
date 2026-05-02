// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

const (
	idleRefreshInterval  = 9 * time.Minute // under Gmail's 10-min cap
	pollFallbackInterval = 60 * time.Second
	reconnectInitial     = 1 * time.Second
	reconnectMax         = 30 * time.Second
)

// idleLoop runs until ctx is cancelled. It selects the current
// folder on the idle connection, runs IDLE (or poll fallback),
// honors folder-switch signals from OpenFolder, and reconnects
// with exponential backoff on failure.
func (b *Backend) idleLoop(ctx context.Context) {
	defer close(b.idleDone)

	backoff := reconnectInitial
	for {
		if ctx.Err() != nil {
			return
		}
		err := b.runIdleSession(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			b.emit(mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnReconnecting})
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > reconnectMax {
				backoff = reconnectMax
			}
			continue
		}
		backoff = reconnectInitial
	}
}

// runIdleSession selects the current folder on the idle connection,
// runs IDLE (or poll), and listens for folder-switch signals. It
// returns nil on clean refresh-cycle completion (caller re-loops),
// or an error on connection failure.
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
		idleErrCh <- idle.Idle(b.handleUnilateral)
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
			return nil // re-enter loop, fresh IDLE
		case f := <-switchCh:
			idle.IdleStop()
			if err := <-idleErrCh; err != nil {
				return err
			}
			b.mu.Lock()
			b.current = f
			b.mu.Unlock()
			return nil // re-enter loop with new folder
		case err := <-idleErrCh:
			return err
		}
	}
}

// pollLoop runs when the server lacks IDLE. STATUS every 60s and
// emit UpdateFolderInfo on change. Honors folder-switch signals.
func (b *Backend) pollLoop(ctx context.Context, folder string, switchCh chan string) error {
	t := time.NewTicker(pollFallbackInterval)
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
			// Fire UpdateFolderInfo unconditionally; UI re-fetches on receipt.
			b.emit(mail.Update{Type: mail.UpdateFolderInfo, Folder: folder})
		}
	}
}

// handleUnilateral receives unilateral IDLE responses (translated by
// the realClient adapter into mail.Update values) and forwards them.
func (b *Backend) handleUnilateral(u mail.Update) {
	b.emit(u)
}

// emit sends u to the updates channel, dropping if buffer is full.
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
	}
}
