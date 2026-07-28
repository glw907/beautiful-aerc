package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/uerr"
)

// newTestWriter opens a fresh migrated store file and returns a
// Writer over it, along with the file's path for tests that inspect
// the WAL file directly.
func newTestWriter(t *testing.T, cfg WriterConfig) (*Writer, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", dsn(path, connReadWrite))
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	w, err := NewWriter(db, cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w, path
}

// TestWriterSerializes proves that many concurrent Submit callers
// each get their job run in the order their send reached the
// interactive lane, and that no two jobs ever run at once.
func TestWriterSerializes(t *testing.T) {
	w, _ := newTestWriter(t, DefaultWriterConfig())

	// A first job blocks the writer so every submitter below is
	// guaranteed to be queued, in launch order, before any of them
	// runs.
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = w.Submit(context.Background(), func(*sql.Tx) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started

	const n = 20
	var active atomic.Bool
	var mu sync.Mutex
	var executed []int

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := w.Submit(context.Background(), func(*sql.Tx) error {
				if !active.CompareAndSwap(false, true) {
					t.Error("two jobs ran concurrently")
				}
				mu.Lock()
				executed = append(executed, i)
				mu.Unlock()
				time.Sleep(time.Millisecond)
				active.Store(false)
				return nil
			})
			if err != nil {
				t.Errorf("Submit(%d): %v", i, err)
			}
		}(i)
		// Give goroutine i time to block on the channel send before
		// launching i+1, so the queue order below is the launch order.
		time.Sleep(5 * time.Millisecond)
	}

	close(release)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	want := make([]int, n)
	for i := range want {
		want[i] = i
	}
	if len(executed) != n {
		t.Fatalf("executed %d jobs, want %d", len(executed), n)
	}
	for i := range executed {
		if executed[i] != want[i] {
			t.Fatalf("execution order = %v, want arrival order %v", executed, want)
		}
	}
}

// TestInteractivePreemption shows an interactive job admitted while
// a chunked bulk job is in flight, with the wait bounded by the
// current chunk rather than the whole queued bulk backlog.
func TestInteractivePreemption(t *testing.T) {
	w, _ := newTestWriter(t, DefaultWriterConfig())

	const chunkWork = 20 * time.Millisecond
	const chunks = 10
	const admissionCeiling = 15 * time.Millisecond // under one chunk's remaining time

	bulkDone := make(chan error, 1)
	go func() {
		for range chunks {
			err := w.SubmitBulk(context.Background(), func(*sql.Tx) error {
				time.Sleep(chunkWork)
				return nil
			})
			if err != nil {
				bulkDone <- err
				return
			}
		}
		bulkDone <- nil
	}()

	time.Sleep(chunkWork / 2) // let the bulk job start its first chunk

	start := time.Now()
	if err := w.Submit(context.Background(), func(*sql.Tx) error { return nil }); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waited := time.Since(start)

	if waited > admissionCeiling {
		t.Errorf("interactive admission took %v, want under %v; %d queued bulk chunks would be %v",
			waited, admissionCeiling, chunks, chunks*chunkWork)
	}

	if err := <-bulkDone; err != nil {
		t.Fatalf("bulk chunk failed: %v", err)
	}
}

// TestBackfillSubordination drives the chunk-decision loop a bulk
// worker (the future backfill) is expected to run: check
// RecentInteractiveActivity before each chunk and skip it if
// interactive use is recent. ADR-0003 revision 2 requires this
// signal specifically because the lane's queue depth is empty
// exactly when the writer is busy, so a depth-based check would pass
// while the worker never actually yielded.
func TestBackfillSubordination(t *testing.T) {
	cfg := DefaultWriterConfig()
	cfg.InteractiveQuiet = 30 * time.Millisecond
	w, _ := newTestWriter(t, cfg)

	chunk := func() bool {
		if w.RecentInteractiveActivity(cfg.InteractiveQuiet) {
			return false
		}
		return w.SubmitBulk(context.Background(), func(*sql.Tx) error { return nil }) == nil
	}

	if !chunk() {
		t.Fatal("chunk skipped with no interactive activity yet")
	}

	if err := w.Submit(context.Background(), func(*sql.Tx) error { return nil }); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if chunk() {
		t.Fatal("chunk ran immediately after interactive activity, want it to yield")
	}

	time.Sleep(cfg.InteractiveQuiet + 20*time.Millisecond)

	if !chunk() {
		t.Fatal("chunk still yielding after the quiet window elapsed")
	}
}

// TestDiskFullInjection simulates SQLITE_FULL with max_page_count,
// the standard portable stand-in for an actual full disk, and proves
// the failing transaction commits nothing and surfaces as a
// uerr.Error under ClassStoreLocal, the class uerr documents for a
// full disk.
func TestDiskFullInjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", dsn(path, connReadWrite))
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(`PRAGMA max_page_count = 40`); err != nil {
		t.Fatalf("set max_page_count: %v", err)
	}

	w, err := NewWriter(db, DefaultWriterConfig())
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	big := strings.Repeat("x", 4096)
	writeErr := w.Submit(context.Background(), func(tx *sql.Tx) error {
		for i := range 5000 {
			if _, err := tx.Exec(
				`INSERT INTO account (slug, backend_kind, address, data) VALUES (?, ?, ?, ?)`,
				fmt.Sprintf("acct-%d", i), "jmap", "user@example.com", big); err != nil {
				return err
			}
		}
		return nil
	})
	if writeErr == nil {
		t.Fatal("Submit succeeded past max_page_count, want a disk-full failure")
	}

	var uerrErr uerr.Error
	if !errors.As(writeErr, &uerrErr) {
		t.Fatalf("error is not a uerr.Error: %v", writeErr)
	}
	if uerrErr.Class != uerr.ClassStoreLocal {
		t.Errorf("Class = %v, want %v", uerrErr.Class, uerr.ClassStoreLocal)
	}

	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM account`).Scan(&rows); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if rows != 0 {
		t.Errorf("accounts = %d, want 0 (the failed transaction must not have committed partially)", rows)
	}
}
