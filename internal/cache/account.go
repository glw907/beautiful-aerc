// SPDX-License-Identifier: MIT

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
	"github.com/glw907/poplar/internal/mail"
)

// Account exposes a single account's handle. The UI reads/writes
// through this type. The backend pointer is held so the syncer
// and drainer can talk to the protocol layer.
type Account struct {
	Backend       mail.Backend
	ChangeTracker mail.ChangeTracker

	db   *sql.DB
	dir  string
	name string

	// maxSize is the body-cache size cap in bytes. 0 disables. When an insert
	// would push total over maxSize, evict by messages.sent_at ASC until under cap.
	maxSize int64
	// maxAttachmentSize is the attachment-bytes-cache size cap. 0 disables.
	maxAttachmentSize int64

	events        chan CacheEvent
	droppedEvents atomic.Uint64

	// Lifecycle handles. The drainer wakes on signal. The syncer
	// runs per-folder when wired up by the App.
	drainSignal chan struct{}
	stop        chan struct{}
	wg          sync.WaitGroup
}

// OpStatus is the lifecycle state of an outbox row. Stored as the
// underlying string in outbox.status and emitted on CacheEvent.Status.
type OpStatus string

const (
	OpPending   OpStatus = "pending"
	OpExecuting OpStatus = "executing"
	OpDone      OpStatus = "done"
	OpFailed    OpStatus = "failed"
	OpConflict  OpStatus = "conflict"
)

// OpKind is the discriminator for OpArgs. The string value is
// stored in outbox.kind and returned by OpArgs.opKind().
type OpKind string

const (
	KindMove      OpKind = "move"
	KindFlag      OpKind = "flag"
	KindDestroy   OpKind = "destroy"
	KindSend      OpKind = "send"
	KindAppend    OpKind = "append"
	KindPushDraft OpKind = "push-draft"
)

// CacheEvent is the drainer→UI signal channel payload. App's
// pumpCacheCmd ranges (*Account).Events() and re-emits these as
// tea.Msg.
type CacheEvent struct {
	Account string
	OpID    int64
	Kind    OpKind
	Status  OpStatus
	Err     string // populated for conflict/failed
	// Note carries an out-of-band advisory the App should surface as
	// a one-shot banner. Currently used for draft-superseded events
	// where the op succeeded server-side but the local row was gone.
	// See the drainer's KindPushDraft handler.
	Note string
}

// The zero Config disables the size backstops.
type Config struct {
	// MaxSize is the body-cache size cap in bytes. 0 disables.
	// Default 2GB when populated from [cache] in config.toml.
	MaxSize int64
	// MaxAttachmentSize is the attachment-bytes-cache size cap in
	// bytes. 0 disables. Tracked separately from MaxSize.
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
	// modernc.org/sqlite uses one connection per pragma scope by
	// default. Cap to a small pool with the same DSN so writers
	// serialize on a single connection (matches the WAL "one writer,
	// many readers" pattern).
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

// DroppedEvents returns the count of CacheEvents the drainer has
// dropped because the events buffer was full. When the count
// advances, do a full cache re-read rather than incremental
// Event handling.
func (a *Account) DroppedEvents() uint64 { return a.droppedEvents.Load() }

// Close stops background goroutines and closes the database.
func (a *Account) Close() error {
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

// signalDrainer wakes the drainer if it's idle. Non-blocking: a
// pending wake-up coalesces with the new one.
func (a *Account) signalDrainer() {
	select {
	case a.drainSignal <- struct{}{}:
	default:
	}
}

// Slugify lowercases name and reduces non-[a-z0-9-] runs to a
// single dash. Leading/trailing dashes are stripped.
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
