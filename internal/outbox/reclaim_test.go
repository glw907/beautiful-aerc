package outbox

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
)

// reclaimKillDBEnv names the store TestReclaimKillChild drives when
// TestReclaimOrphanedAfterKill re-execs this test binary as its own
// subprocess. The child selects itself by name through -test.run, so
// this package needs no TestMain of its own.
const reclaimKillDBEnv = "POPLAR_RECLAIM_KILL_DB"

// TestReclaimOrphanedAfterKill proves an intent survives a process
// death between DispatchOnce's two commits. The child claims one move
// intent, commits the claim, and SIGKILLs itself inside the backend
// call before the finalize transaction begins, leaving the row in
// dispatching, where selectEligible never looks again. The restart
// must reclaim it and dispatch it on the next pass.
func TestReclaimOrphanedAfterKill(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	cmd := exec.Command(os.Args[0], "-test.run=^TestReclaimKillChild$") //nolint:gosec // G204: os.Args[0] is this same test binary, re-invoked as its own subprocess
	cmd.Env = append(os.Environ(), reclaimKillDBEnv+"="+dbPath)
	out, err := cmd.CombinedOutput()
	if !killedBySIGKILL(err) {
		t.Fatalf("child exited %v, want death by SIGKILL inside the backend call; output:\n%s", err, out)
	}

	intentID, state := readOnlyOutboxRow(t, dbPath)
	if state != "dispatching" {
		t.Fatalf("outbox row %d after the kill = %q, want %q; the child died outside the window under test", intentID, state, "dispatching")
	}

	w, err := store.Open(dbPath, store.DefaultWriterConfig())
	if err != nil {
		t.Fatalf("reopen store after the kill: %v", err)
	}
	defer func() { _ = w.Close() }()
	reads := openReads(t, dbPath, w)

	if err := ReclaimOrphaned(context.Background(), w); err != nil {
		t.Fatalf("ReclaimOrphaned: %v", err)
	}
	if state, _ := outboxState(t, w, intentID); state != "queued" {
		t.Fatalf("outbox row %d after the sweep = %q, want %q", intentID, state, "queued")
	}

	be := newFakeBackend()
	be.MailSource.ApplyBatchFunc = func(context.Context, []backend.Mutation) (backend.BatchResult, error) {
		return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
	}
	result, err := NewDispatcher(reclaimKillAccountID, be, w, reads).DispatchOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("dispatch after the sweep: %v", err)
	}
	if len(result.Delivered) != 1 || result.Delivered[0].IntentID != intentID {
		t.Errorf("Delivered = %+v, want the reclaimed intent %d", result.Delivered, intentID)
	}
	if n := outboxCount(t, w, intentID); n != 0 {
		t.Errorf("outbox rows for intent %d after a successful redispatch = %d, want 0", intentID, n)
	}
}

// TestReclaimOrphanedLogs proves the sweep reports itself as the
// informational event it is: one info line carrying the count when a
// restart finds orphans, and silence on the ordinary restart that
// finds none. A run recovering from a kill has failed at nothing, so
// this line does not travel the uerr seam, which speaks in
// user-facing failure sentences.
func TestReclaimOrphanedLogs(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	log := uerrtest.CaptureDefault(t)

	if err := ReclaimOrphaned(context.Background(), w); err != nil {
		t.Fatalf("ReclaimOrphaned over an empty outbox: %v", err)
	}
	if got := log.String(); strings.Contains(got, "outbox:") {
		t.Errorf("a sweep with nothing to reclaim logged %q, want silence", got)
	}

	id, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, time.Now())
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	strandDispatching(t, w, id)

	if err := ReclaimOrphaned(context.Background(), w); err != nil {
		t.Fatalf("ReclaimOrphaned: %v", err)
	}
	line := log.String()
	if !strings.Contains(line, "level=INFO") || !strings.Contains(line, "count=1") {
		t.Errorf("the sweep logged %q, want an info line carrying count=1", line)
	}
	if strings.Contains(line, "level=ERROR") {
		t.Errorf("the sweep logged %q at error level, want info: nothing failed", line)
	}
}

// reclaimKillAccountID is the account id TestReclaimKillChild seeds
// first into a fresh store, which the parent process names again
// after the child is gone.
const reclaimKillAccountID = 1

// TestReclaimKillChild is TestReclaimOrphanedAfterKill's subprocess:
// it enqueues one move intent and dispatches it against a backend
// that kills the process mid-call. It runs only under
// reclaimKillDBEnv, which the parent sets.
func TestReclaimKillChild(t *testing.T) {
	dbPath := os.Getenv(reclaimKillDBEnv)
	if dbPath == "" {
		t.Skip("subprocess half of TestReclaimOrphanedAfterKill; the parent names its store through " + reclaimKillDBEnv)
	}

	w, err := store.Open(dbPath, store.DefaultWriterConfig())
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	reads := openReads(t, dbPath, w)

	accountID := seedAccount(t, w)
	if accountID != reclaimKillAccountID {
		t.Fatalf("seeded account id = %d, want %d; the parent names it after this process is gone", accountID, reclaimKillAccountID)
	}
	src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
	dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")
	messageID := seedMessage(t, w, accountID, src, "msg-1")

	be := newFakeBackend()
	be.MailSource.ApplyBatchFunc = func(context.Context, []backend.Mutation) (backend.BatchResult, error) {
		// The claim transaction has committed and the finalize
		// transaction has not begun: the one window a death strands a
		// row in dispatching.
		if err := syscall.Kill(os.Getpid(), syscall.SIGKILL); err != nil {
			t.Errorf("kill self mid-dispatch: %v", err)
		}
		time.Sleep(30 * time.Second)
		return backend.BatchResult{}, nil
	}

	if _, _, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{messageID}, dest, 0, be, false, time.Now()); err != nil {
		t.Fatalf("enqueue move: %v", err)
	}
	if _, err := NewDispatcher(accountID, be, w, reads).DispatchOnce(context.Background(), time.Now()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	t.Fatal("dispatch returned; the backend call was supposed to end this process")
}

// killedBySIGKILL reports whether err is the exit a self-inflicted
// SIGKILL produces, distinct from an exit code the child's own
// failure path chose.
func killedBySIGKILL(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGKILL
}

// readOnlyOutboxRow returns the id and state of the single outbox row
// the killed child left in the store at dbPath, over a connection of
// its own since no writer is open yet.
func readOnlyOutboxRow(t *testing.T, dbPath string) (id int64, state string) {
	t.Helper()

	db, err := store.OpenWriteConn(dbPath)
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	defer func() { _ = db.Close() }()

	if err := db.QueryRow(`SELECT id, state FROM outbox`).Scan(&id, &state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			t.Fatal("the killed child left no outbox row; it died before enqueueing one")
		}
		t.Fatalf("read the outbox row: %v", err)
	}
	return id, state
}
