// Package cache is poplar's local mail store. Each account maps to
// a SQLite database. The cache is the UI-facing read/write layer;
// mail.Backend / mail.ChangeTracker handle protocol I/O. See
// docs/superpowers/specs/2026-05-02-cache-0-design.md.
package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	_ "modernc.org/sqlite"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/contacts"
	"github.com/glw907/poplar/internal/mail"
)

// Account is a single account's UI-facing read/write handle.
type Account struct {
	Backend        mail.Backend
	ChangeTracker  mail.ChangeTracker
	ContactsWriter contacts.Writer

	log   *slog.Logger
	db    *sql.DB
	dir   string
	name  string
	email string

	maxSize           int64
	maxAttachmentSize int64
	maxOutboxBytes    int64

	events        chan Event
	droppedEvents atomic.Uint64

	drainSignal   chan struct{}
	drainerPaused atomic.Bool
	drainerStop   context.CancelFunc
	stop          chan struct{}
	wg            sync.WaitGroup
	backfiller    *Backfiller
	backfillStop  context.CancelFunc

	burstTotal atomic.Int32
	burstDone  atomic.Int32

	attachInFlight atomic.Int32
	syncInFlight   atomic.Int32
}

// OpStatus is the lifecycle state of an outbox row.
type OpStatus string

const (
	OpPending   OpStatus = "pending"
	OpExecuting OpStatus = "executing"
	OpDone      OpStatus = "done"
	OpFailed    OpStatus = "failed"
	OpConflict  OpStatus = "conflict"
)

// OpKind is the discriminator for OpArgs.
type OpKind string

const (
	KindMove          OpKind = "move"
	KindFlag          OpKind = "flag"
	KindDestroy       OpKind = "destroy"
	KindSend          OpKind = "send"
	KindAppend        OpKind = "append"
	KindPushDraft     OpKind = "push-draft"
	KindContactPut    OpKind = "contact-put"
	KindContactDelete OpKind = "contact-delete"
)

// Event is the drainer→UI signal payload.
type Event struct {
	Account string
	OpID    int64
	Kind    OpKind
	Status  OpStatus
	Err     string
	// Note carries an out-of-band advisory the App surfaces as a
	// one-shot banner. The current consumer is draft-superseded
	// events where the server op succeeded but the local row was gone.
	Note string
}

// Config sets the cache size backstops. Zero values disable.
type Config struct {
	// MaxSize caps the body-cache in bytes. Zero disables the cap; configurable via [cache] max-size in config.toml.
	MaxSize int64
	// MaxAttachmentSize caps attachment bytes, tracked separately from MaxSize.
	MaxAttachmentSize int64
	// MaxOutboxBytes rejects an outbox INSERT whose payload exceeds this
	// many bytes. Zero disables the check; configurable via [cache]
	// max-outbox-bytes. Mirrors MaxSize's "0 disables" semantics.
	MaxOutboxBytes int64
}

// DBPath returns the on-disk SQLite path for accountName under dir.
// Empty dir defaults to $XDG_CACHE_HOME/poplar (or platform equivalent).
// A leading ~ in dir is expanded to the user's home directory.
func DBPath(accountName, dir string) (string, error) {
	var expanded string
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("user cache dir: %v", err)
		}
		expanded = filepath.Join(base, "poplar")
	} else {
		exp, err := config.ExpandHome(dir)
		if err != nil {
			return "", fmt.Errorf("expand %q: %v", dir, err)
		}
		expanded = exp
	}
	slug := Slugify(accountName)
	if slug == "" {
		return "", fmt.Errorf("account %q produces empty cache slug", accountName)
	}
	return filepath.Join(expanded, slug, "mail.db"), nil
}

// OpenDB opens a SQLite database at path with the standard poplar pragmas.
// Callers are responsible for setting pool sizes and running migrations.
func OpenDB(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", path)
	return sql.Open("sqlite", dsn)
}

// Open opens the per-account SQLite store and runs migrations. The
// returned Account has no backend; call WireBackend before any sync,
// backfill, or outbox work.
//
// dir is the cache base directory. The per-account subdirectory is
// created if absent. A leading ~ is expanded to the user's home.
// A nil log defaults to slog.Default() tagged with the package name.
func Open(accountName string, dir string, cfg Config, log *slog.Logger) (*Account, error) {
	if log == nil {
		log = slog.Default().With("component", "cache")
	}
	dbPath, err := DBPath(accountName, dir)
	if err != nil {
		return nil, err
	}
	acctDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(acctDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir cache: %w", err)
	}
	db, err := OpenDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// WAL is one writer, many readers. Keep the pool small so writers serialize.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	log.Debug("cache open", "schema", schemaVersion, "account", accountName)
	return &Account{
		log:               log,
		db:                db,
		dir:               acctDir,
		name:              accountName,
		maxSize:           cfg.MaxSize,
		maxAttachmentSize: cfg.MaxAttachmentSize,
		maxOutboxBytes:    cfg.MaxOutboxBytes,
		events:            make(chan Event, 32),
		drainSignal:       make(chan struct{}, 1),
		stop:              make(chan struct{}),
	}, nil
}

// WireBackend wires b and ct and starts background goroutines.
// Call exactly once.
func (a *Account) WireBackend(backend mail.Backend, ct mail.ChangeTracker) error {
	if a.Backend != nil {
		return errors.New("cache: backend already wired")
	}
	a.Backend = backend
	a.ChangeTracker = ct
	a.backfiller = newBackfiller(a)
	bfCtx, bfCancel := context.WithCancel(context.Background())
	a.backfillStop = bfCancel
	go a.backfiller.Run(bfCtx)
	if err := a.startDrainer(); err != nil {
		bfCancel()
		return err
	}
	return nil
}

// ErrNotConnected is returned by cache methods that require a live backend
// when no backend has been attached.
var ErrNotConnected = errors.New("cache: backend not yet wired")

func (a *Account) Name() string { return a.name }

// Connected reports whether a backend has been attached to this account.
func (a *Account) Connected() bool { return a.Backend != nil }

func (a *Account) AccountName() string { return a.name }

// SetEmail records the account's primary email so AccountEmail() can
// answer pre-wire. Callers set it once between Open and WireBackend
// from config; subsequent backend wiring does not override it.
func (a *Account) SetEmail(email string) { a.email = email }

func (a *Account) AccountEmail() string {
	if a.email != "" {
		return a.email
	}
	if a.Backend == nil {
		return ""
	}
	return a.Backend.AccountEmail()
}

// DB is for tests and CLI introspection. Production callers go through
// the Account's typed methods.
func (a *Account) DB() *sql.DB { return a.db }

func (a *Account) Dir() string { return a.dir }

func (a *Account) Events() <-chan Event { return a.events }

// DroppedEvents counts Events the drainer dropped on a full
// buffer. A change since the last read means incremental Event
// handling has lost ground. Re-read the cache instead.
func (a *Account) DroppedEvents() uint64 { return a.droppedEvents.Load() }

// BackfillProgress returns the count of cached bodies, the total
// message count, and whether the backfiller is currently in throttle
// backoff. The warn flag drives the status-bar `↓ ⚠` substate.
func (a *Account) BackfillProgress() (done, total int, warn bool, err error) {
	if err = a.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&total); err != nil {
		return 0, 0, false, err
	}
	if err = a.db.QueryRow(`SELECT COUNT(*) FROM bodies`).Scan(&done); err != nil {
		return 0, 0, false, err
	}
	if a.backfiller == nil {
		return done, total, false, nil
	}
	return done, total, a.backfiller.Throttling(), nil
}

// OutboxDrainProgress reports the current outbox drain burst's progress.
// Returns (pct, true) while the drainer is working a non-empty queue;
// (0, false) when idle.
func (a *Account) OutboxDrainProgress() (pct int, active bool) {
	total := a.burstTotal.Load()
	if total == 0 {
		return 0, false
	}
	done := a.burstDone.Load()
	return int(int64(done) * 100 / int64(total)), true
}

// AttachmentDownloadProgress reports whether an attachment fetch is in
// flight. pct is always 0 (indeterminate).
func (a *Account) AttachmentDownloadProgress() (pct int, active bool) {
	return 0, a.attachInFlight.Load() > 0
}

// SyncProgress is active while a folder sync runs. pct is unused.
func (a *Account) SyncProgress() (pct int, active bool) {
	return 0, a.syncInFlight.Load() > 0
}

// Bracket the long-running progress sources. UI sets the OSC 9;4 bar from
// the corresponding *Progress accessors.
func (a *Account) BeginAttachmentDownload() { a.attachInFlight.Add(1) }
func (a *Account) EndAttachmentDownload()   { a.attachInFlight.Add(-1) }
func (a *Account) BeginSync()               { a.syncInFlight.Add(1) }
func (a *Account) EndSync()                 { a.syncInFlight.Add(-1) }

// NotifyActivity signals recent user input, pausing backfill until
// the idle threshold elapses. No-op before WireBackend; a keystroke
// can land between cache.Open and the BackendReadyMsg fanout
// (ADR-0242).
func (a *Account) NotifyActivity() {
	if a.backfiller == nil {
		return
	}
	a.backfiller.NotifyActivity()
}

// NotifyConnState pauses backfill while offline. No-op pre-wire.
func (a *Account) NotifyConnState(online bool) {
	if a.backfiller == nil {
		return
	}
	a.backfiller.NotifyConnState(online)
}

// Close stops background goroutines and closes the database.
func (a *Account) Close() error {
	if a.drainerStop != nil {
		a.drainerStop()
	}
	if a.backfillStop != nil {
		a.backfillStop()
	}
	select {
	case <-a.stop:
	default:
		close(a.stop)
	}
	a.wg.Wait()
	return a.db.Close()
}

// tx runs fn inside a transaction, rolling back on error.
func (a *Account) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// signalDrainer wakes the drainer. Pending wakes coalesce.
func (a *Account) signalDrainer() {
	select {
	case a.drainSignal <- struct{}{}:
	default:
	}
}

// Slugify lowercases name and replaces non-[a-z0-9-] runs with a
// single dash. Leading and trailing dashes are stripped.
func Slugify(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
