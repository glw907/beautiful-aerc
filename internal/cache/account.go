// Package cache is poplar's local mail store. Each account maps to
// a SQLite database. The cache is the UI-facing read/write layer;
// mail.Backend / mail.ChangeTracker handle protocol I/O. See
// docs/superpowers/specs/2026-05-02-cache-0-design.md.
package cache

import (
	"context"
	"database/sql"
	"fmt"
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

	db   *sql.DB
	dir  string
	name string

	maxSize           int64
	maxAttachmentSize int64

	events        chan CacheEvent
	droppedEvents atomic.Uint64

	drainSignal  chan struct{}
	stop         chan struct{}
	wg           sync.WaitGroup
	backfiller   *Backfiller
	backfillStop context.CancelFunc
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

// CacheEvent is the drainer→UI signal payload.
type CacheEvent struct {
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
	// MaxSize caps the body-cache in bytes. Zero (the new default) disables the cap; configurable via [cache] max-size in config.toml.
	MaxSize int64
	// MaxAttachmentSize caps attachment bytes, tracked separately from MaxSize.
	MaxAttachmentSize int64
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

// Open returns an Account ready for reads and writes. It opens (or
// creates) the per-account SQLite database under dir, applies
// pragmas, and runs schema migrations to the current version.
//
// dir is the cache base directory. The per-account subdirectory is
// created if absent. A leading ~ is expanded to the user's home.
func Open(accountName string, backend mail.Backend, ct mail.ChangeTracker, dir string, cfg Config) (*Account, error) {
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
	a := &Account{
		Backend:           backend,
		ChangeTracker:     ct,
		db:                db,
		dir:               acctDir,
		name:              accountName,
		maxSize:           cfg.MaxSize,
		maxAttachmentSize: cfg.MaxAttachmentSize,
		events:            make(chan CacheEvent, 32),
		drainSignal:       make(chan struct{}, 1),
		stop:              make(chan struct{}),
	}
	a.backfiller = newBackfiller(a)
	bfCtx, cancel := context.WithCancel(context.Background())
	a.backfillStop = cancel
	go a.backfiller.Run(bfCtx)
	return a, nil
}

func (a *Account) Name() string { return a.name }

func (a *Account) AccountName() string {
	if a.Backend == nil {
		return a.name
	}
	return a.Backend.AccountName()
}

func (a *Account) AccountEmail() string {
	if a.Backend == nil {
		return ""
	}
	return a.Backend.AccountEmail()
}

// DB is for tests and CLI introspection. Production callers go through
// the Account's typed methods.
func (a *Account) DB() *sql.DB { return a.db }

func (a *Account) Dir() string { return a.dir }

func (a *Account) Events() <-chan CacheEvent { return a.events }

// DroppedEvents counts CacheEvents the drainer dropped on a full
// buffer. A change since the last read means incremental Event
// handling has lost ground. Re-read the cache instead.
func (a *Account) DroppedEvents() uint64 { return a.droppedEvents.Load() }

// Close stops background goroutines and closes the database.
func (a *Account) Close() error {
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
