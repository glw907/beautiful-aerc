// Package mailimap implements mail.Backend over IMAP4rev1 using
// emersion/go-imap. Capabilities are negotiated at Connect; UIDPLUS
// is required, MOVE / SPECIAL-USE / IDLE are used opportunistically.
//
// A Backend owns two physical IMAP connections: a synchronous
// "command" connection used by every mail.Backend method, and an
// "idle" connection that runs in a goroutine emitting mail.Update
// values. Both share the auth path defined in auth.go.
package mailimap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailauth"
)

// Backend is one IMAP account.
type Backend struct {
	cfg   config.AccountConfig
	log   *slog.Logger
	oauth *mailauth.Client // non-nil for xoauth2 accounts using mailauth

	mu       sync.Mutex
	cmd      imapClient // command connection, nil before Connect
	idle     imapClient
	smtp     smtpClient // dialed lazily on first Send
	caps     capSet
	current  string // selected folder on cmd
	trash    string // resolved on first Delete
	password string // cached PasswordCmd result
	updates  chan mail.Update

	idleCancel context.CancelFunc
	idleDone   chan struct{}
	switchCh   chan string

	// connCtx is the parent context for redials. Set in Connect,
	// cancelled in Disconnect.
	connCtx    context.Context
	connCancel context.CancelFunc

	// dialFn opens a fresh IMAP connection for the given role.
	// Defaults to dial(ctx, b, role); tests swap in a fake.
	dialFn func(ctx context.Context, role string) (imapClient, error)
}

type capSet struct {
	UIDPLUS    bool
	MOVE       bool
	IDLE       bool
	SpecialUse bool
}

// New constructs an unconnected Backend. A nil logger defaults to
// slog.Default() tagged with the package name.
func New(cfg config.AccountConfig, log *slog.Logger) *Backend {
	if log == nil {
		log = slog.Default().With("component", "mailimap")
	}
	b := &Backend{cfg: cfg, log: log}
	b.dialFn = func(ctx context.Context, role string) (imapClient, error) { return dial(ctx, b, role) }
	return b
}

// NewWithOAuth returns a Backend that obtains XOAUTH2 access tokens via c
// rather than running password-cmd on every dial. c must be non-nil.
func NewWithOAuth(cfg config.AccountConfig, c *mailauth.Client, log *slog.Logger) *Backend {
	if log == nil {
		log = slog.Default().With("component", "mailimap")
	}
	b := &Backend{cfg: cfg, log: log, oauth: c}
	b.dialFn = func(ctx context.Context, role string) (imapClient, error) { return dial(ctx, b, role) }
	return b
}

func (b *Backend) AccountName() string {
	if b.cfg.Display != "" {
		return b.cfg.Display
	}
	if b.cfg.Email != "" {
		return b.cfg.Email
	}
	return b.cfg.Name
}

func (b *Backend) AccountEmail() string {
	if b.cfg.From != nil && b.cfg.From.Address != "" {
		return b.cfg.From.Address
	}
	return b.cfg.Email
}

func (b *Backend) IsJMAP() bool { return false }

// Updates returns a nil channel before Connect succeeds.
func (b *Backend) Updates() <-chan mail.Update { return b.updates }

const updatesBuffer = 64

// Connect dials both connections, authenticates, negotiates capabilities,
// and starts the idle goroutine. UIDPLUS is required. Tests bypass by
// setting b.cmd / b.idle directly and calling finishConnect.
func (b *Backend) Connect(ctx context.Context) error {
	cmd, err := b.dialFn(ctx, "command")
	if err != nil {
		return fmt.Errorf("connect cmd: %w", err)
	}
	idle, err := b.dialFn(ctx, "idle")
	if err != nil {
		_ = cmd.Logout()
		return fmt.Errorf("connect idle: %w", err)
	}
	b.mu.Lock()
	b.cmd = cmd
	b.idle = idle
	b.mu.Unlock()

	return b.finishConnect(ctx)
}

// finishConnect runs the post-dial bringup. Split out so unit tests
// can drive it with fakes.
func (b *Backend) finishConnect(ctx context.Context) error {
	caps, err := b.cmd.Capabilities()
	if err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}
	cs := capSet{
		UIDPLUS:    caps["UIDPLUS"],
		MOVE:       caps["MOVE"],
		IDLE:       caps["IDLE"],
		SpecialUse: caps["SPECIAL-USE"],
	}
	if !cs.UIDPLUS {
		return errors.New("server does not advertise UIDPLUS, required for safe deletion")
	}
	if b.cfg.GmailQuirks && !caps["X-GM-EXT-1"] {
		return errors.New("gmail account but server does not advertise X-GM-EXT-1")
	}

	updates := make(chan mail.Update, updatesBuffer)

	connCtx, connCancel := context.WithCancel(ctx)
	idleCtx, idleCancel := context.WithCancel(connCtx)

	b.mu.Lock()
	b.caps = cs
	b.updates = updates
	b.switchCh = make(chan string, 1)
	b.connCtx = connCtx
	b.connCancel = connCancel
	b.idleCancel = idleCancel
	b.idleDone = make(chan struct{})
	b.mu.Unlock()

	go b.idleLoop(idleCtx)

	return nil
}

// Disconnect tears down the idle goroutine then logs out both
// connections. Returns the first non-nil error.
func (b *Backend) Disconnect() error {
	b.mu.Lock()
	idleCancel := b.idleCancel
	connCancel := b.connCancel
	done := b.idleDone
	cmd := b.cmd
	idle := b.idle
	smtp := b.smtp
	b.smtp = nil
	b.mu.Unlock()

	if idleCancel != nil {
		idleCancel()
	}
	if done != nil {
		<-done
	}
	if connCancel != nil {
		connCancel()
	}

	var firstErr error
	if cmd != nil {
		if err := cmd.Logout(); err != nil {
			firstErr = err
		}
	}
	if idle != nil {
		if err := idle.Logout(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if smtp != nil {
		if err := smtp.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
