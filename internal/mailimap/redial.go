package mailimap

import (
	"context"
	"errors"

	"github.com/glw907/poplar/internal/mail"
)

// cmdClient returns the cached command connection, dialing a fresh one
// when the cache is empty. Re-selects the previously open folder on a
// fresh connection so UID-scoped commands still address the right
// mailbox.
func (b *Backend) cmdClient() (imapClient, error) {
	b.mu.Lock()
	if b.cmd != nil {
		c := b.cmd
		b.mu.Unlock()
		return c, nil
	}
	ctx := b.connCtx
	b.mu.Unlock()

	if ctx == nil {
		return nil, errors.New("cmd: backend not connected")
	}

	fresh, err := b.dialFn(ctx, "command")
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	if b.cmd != nil {
		// Lost a race. Keep the existing handle, drop the duplicate.
		duplicate := fresh
		c := b.cmd
		b.mu.Unlock()
		_ = duplicate.Logout()
		return c, nil
	}
	b.cmd = fresh
	current := b.current
	b.mu.Unlock()

	if current != "" {
		if _, err := fresh.Select(current, false); err != nil {
			b.log.Warn("imap cmd redial: re-select failed", "folder", current, "err", err)
		}
	}
	b.log.Info("imap cmd redialed")
	return fresh, nil
}

// dropCmd clears b.cmd iff it still points at c, so concurrent callers
// that already hold an older handle do not lose a redial done by
// another goroutine.
func (b *Backend) dropCmd(c imapClient) {
	b.mu.Lock()
	dropped := b.cmd == c
	if dropped {
		b.cmd = nil
	}
	b.mu.Unlock()
	if !dropped {
		return
	}
	b.log.Warn("imap cmd client dropped after connection error")
	_ = c.Logout()
}

// dialIdle opens a fresh idle connection and swaps it into b.idle.
// Caller is responsible for having dropped any prior handle first.
func (b *Backend) dialIdle(ctx context.Context) error {
	fresh, err := b.dialFn(ctx, "idle")
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.idle = fresh
	b.mu.Unlock()
	return nil
}

// dropIdle clears b.idle and logs the dead handle out.
func (b *Backend) dropIdle() {
	b.mu.Lock()
	dead := b.idle
	b.idle = nil
	b.mu.Unlock()
	if dead == nil {
		return
	}
	b.log.Warn("imap idle client dropped after connection error")
	_ = dead.Logout()
}

func (b *Backend) maybeDropOnConn(c imapClient, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, mail.ErrConnection) {
		b.dropCmd(c)
	}
}
