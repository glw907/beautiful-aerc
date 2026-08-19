package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/platform"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
)

// runMainEnvVar names the environment variable
// TestMainReportsStorePathFailureThroughUerr sets to re-invoke its own
// test binary as a subprocess running the real main, rather than the
// test suite.
const runMainEnvVar = "POPLAR_RUN_MAIN"

// TestMain lets the test binary double as a subprocess running the
// real main: re-invoked with runMainEnvVar set, it calls main()
// directly instead of the test suite.
func TestMain(m *testing.M) {
	if os.Getenv(runMainEnvVar) != "" {
		main()
		return
	}
	os.Exit(m.Run())
}

// readySignal is an io.Writer that closes ready the first time it
// sees run's "poplar is running" line, so a test can cancel run's
// context only once startup has actually finished, rather than racing
// it with a context cancelled before run begins.
type readySignal struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	once  sync.Once
	ready chan struct{}
}

func newReadySignal() *readySignal {
	return &readySignal{ready: make(chan struct{})}
}

func (r *readySignal) Write(p []byte) (int, error) {
	r.mu.Lock()
	n, err := r.buf.Write(p)
	r.mu.Unlock()
	if strings.Contains(string(p), "poplar is running") {
		r.once.Do(func() { close(r.ready) })
	}
	return n, err
}

func (r *readySignal) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

// runToCompletion starts run in the background against connect, waits
// for its "poplar is running" line, cancels its context, and returns
// run's outcome along with everything it printed.
func runToCompletion(t *testing.T, dbPath string, f flags, connect backendConnector) (*readySignal, error) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := newReadySignal()
	done := make(chan error, 1)
	go func() { done <- run(ctx, dbPath, f, out, io.Discard, connect) }()

	select {
	case <-out.ready:
	case err := <-done:
		t.Fatalf("run exited before reaching startup, output %q: %v", out.String(), err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for run to start")
	}

	cancel()
	return out, <-done
}

// noopConnector returns a backendConnector wired to an unscripted
// backendtest.Fake, for a startup test that is not exercising the
// sync worker or outbox dispatcher: FakeSource's Changes and
// ApplyBatch already default to a zero-value success, so run reaches
// "poplar is running" and shuts down cleanly with no token and no
// network reach.
func noopConnector(context.Context) (backend.Backend, string, error) {
	return &backendtest.Fake{}, "test-account", nil
}

// connectorBlockingOnFirstChanges returns a backendConnector whose
// fake backend sends on entered the first time its Mail Changes
// method runs, then blocks until release is closed: a real,
// observable sign that an engine is still live inside a Changes call,
// for a test proving run waits for it rather than merely hoping it
// finished in time.
func connectorBlockingOnFirstChanges(entered chan<- struct{}, release <-chan struct{}) backendConnector {
	return func(context.Context) (backend.Backend, string, error) {
		be := &backendtest.Fake{}
		be.MailSource.ChangesFunc = func(_ context.Context, _ backend.ObjectKind, token string, _ int) (backend.ChangeSet, error) {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
			return backend.ChangeSet{NewToken: token}, nil
		}
		return be, "test-account", nil
	}
}

// TestMainReportsStorePathFailureThroughUerr proves an xdg.DataFile
// failure reaches the operator through uerr, not a bare fmt.Errorf
// that never logs (ER-1, ADR-0013): it re-execs this test binary with
// XDG_DATA_HOME and XDG_DATA_DIRS both pointed at a file, so every
// candidate poplar tries for its data directory fails the same way a
// read-only or missing home would.
func TestMainReportsStorePathFailureThroughUerr(t *testing.T) {
	dataHome := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(dataHome, nil, 0o600); err != nil {
		t.Fatalf("seed a file where XDG_DATA_HOME should be a directory: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0]) //nolint:gosec // G204: os.Args[0] is this same test binary, re-invoked as its own subprocess
	cmd.Env = append(os.Environ(), runMainEnvVar+"=1", "XDG_DATA_HOME="+dataHome, "XDG_DATA_DIRS="+dataHome)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("subprocess did not exit within the timeout, want a resolve-store-path failure; output: %s", out)
	}
	if err == nil {
		t.Fatalf("poplar started despite an unusable XDG_DATA_HOME, output: %s", out)
	}

	if !strings.Contains(string(out), "Poplar could not open its store") {
		t.Errorf("output = %q, want the ClassStoreLocal sentence", out)
	}
	if !strings.Contains(string(out), "resolve store path") {
		t.Errorf("output = %q, want the wrapped cause naming what failed", out)
	}
}

// TestRunReportsAnUnwritableLog proves a poplar whose log cannot be
// written says so on a channel that does not depend on the log. slog
// discards a handler's write error, so on a full disk or a read-only
// state directory every error the run reports afterward is dropped
// with no trace anywhere, which is the one failure the log itself
// cannot report (SY-8, ADR-0013). The subprocess runs the real main
// with the log path occupied by a directory: the state directory
// resolves happily and no write can ever open it.
func TestRunReportsAnUnwritableLog(t *testing.T) {
	home := t.TempDir()
	stateHome := filepath.Join(home, "state")
	if err := os.MkdirAll(filepath.Join(stateHome, "poplar", "poplar.log"), 0o700); err != nil {
		t.Fatalf("occupy the log path with a directory: %v", err)
	}

	dataHome := filepath.Join(home, "data")
	if err := os.MkdirAll(filepath.Join(dataHome, "poplar"), 0o750); err != nil {
		t.Fatalf("create the data directory: %v", err)
	}
	seedStore(t, filepath.Join(dataHome, "poplar", "store.db"),
		seedAccountSQL,
		`INSERT INTO mailbox (id, account_id, name) VALUES (1, 1, 'Inbox')`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// --startup-trace runs the whole startup path and exits on its own,
	// so the subprocess needs no signal to finish.
	cmd := exec.CommandContext(ctx, os.Args[0], "--startup-trace") //nolint:gosec // G204: os.Args[0] is this same test binary, re-invoked as its own subprocess
	cmd.Env = append(os.Environ(), runMainEnvVar+"=1", "XDG_DATA_HOME="+dataHome, "XDG_STATE_HOME="+stateHome)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("startup-trace run: %v (stderr: %s)", err, stderr.String())
	}

	if !strings.Contains(stderr.String(), "cannot write its log") {
		t.Errorf("stderr = %q, want poplar reporting the log it cannot write", stderr.String())
	}
	// The report keeps off stdout, which the QA-1 harness parses.
	if strings.Contains(stdout.String(), "cannot write its log") {
		t.Errorf("stdout = %q, want the log report on stderr alone", stdout.String())
	}
}

// TestRunStartsAndShutsDownCleanly proves a normal run opens and
// migrates the store, then marks a clean shutdown on the way out, so
// the next run's ShouldRunIntegrityCheck skips its check.
func TestRunStartsAndShutsDownCleanly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	if _, err := runToCompletion(t, dbPath, flags{}, noopConnector); err != nil {
		t.Fatalf("run: %v", err)
	}

	if _, err := os.Stat(dbPath + ".clean-shutdown"); err != nil {
		t.Errorf("clean-shutdown marker missing after a graceful run: %v", err)
	}
	if store.ShouldRunIntegrityCheck(dbPath, false) {
		t.Error("ShouldRunIntegrityCheck after a clean run = true, want false")
	}
}

// TestRunReclaimsOrphanedIntents proves every startup sweeps outbox
// rows a previous run left in dispatching back to queued, where the
// dispatcher can see them again. The seeded store is marked cleanly
// shut down, so this run owes no integrity check and runs no
// recovery: a stranded row causes neither, which is why the sweep is
// unconditional rather than a step inside one of them.
func TestRunReclaimsOrphanedIntents(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	seedStrandedDispatchingRow(t, dbPath)
	if err := store.MarkCleanShutdown(dbPath); err != nil {
		t.Fatalf("mark clean shutdown: %v", err)
	}
	if store.ShouldRunIntegrityCheck(dbPath, false) {
		t.Fatal("the seeded store owes an integrity check, want a startup that runs neither a check nor a recovery")
	}

	if _, err := runToCompletion(t, dbPath, flags{}, noopConnector); err != nil {
		t.Fatalf("run: %v", err)
	}

	db, err := store.OpenWriteConn(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() { _ = db.Close() }()

	var state string
	if err := db.QueryRow(`SELECT state FROM outbox WHERE id = 1`).Scan(&state); err != nil {
		t.Fatalf("read the stranded outbox row: %v", err)
	}
	if state != "queued" {
		t.Errorf("outbox row state after a startup = %q, want %q", state, "queued")
	}
}

// seedStrandedDispatchingRow migrates a fresh store at dbPath holding
// one outbox row in the state a process killed between DispatchOnce's
// claim and finalize transactions leaves behind.
func seedStrandedDispatchingRow(t *testing.T, dbPath string) {
	t.Helper()

	seedStore(t, dbPath,
		seedAccountSQL,
		`INSERT INTO outbox (id, account_id, kind, payload, state, created_at) VALUES (1, 1, 'move-messages', '{}', 'dispatching', 0)`)
}

// seedAccountSQL is the one account row every startup fixture hangs
// its own rows off, at id 1 so a fixture can name account_id 1
// literally.
const seedAccountSQL = `INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'a', 'jmap', 'a@example.com')`

// seedStore migrates a fresh store at dbPath, runs stmts against it in
// order, and closes the connection, leaving dbPath ready for a run or
// prepareStore call to open on its own.
func seedStore(t *testing.T, dbPath string, stmts ...string) {
	t.Helper()

	seed, err := store.OpenWriteConn(dbPath)
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	if err := store.Migrate(seed); err != nil {
		t.Fatalf("seed Migrate: %v", err)
	}
	for _, stmt := range stmts {
		if _, err := seed.Exec(stmt); err != nil {
			t.Fatalf("seed (%s): %v", stmt, err)
		}
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed connection: %v", err)
	}
}

// TestRunRebuildsIndexOnFlag proves the --rebuild-index flag reaches
// store.RebuildIndex through the writer.
func TestRunRebuildsIndexOnFlag(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	out, err := runToCompletion(t, dbPath, flags{rebuildIndex: true}, noopConnector)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), "rebuilding full-text index") {
		t.Errorf("output = %q, want a rebuild-index status line", out.String())
	}
}

// brokenWriter fails every Write, standing in for a stdout a
// --startup-trace exec cannot reach (a closed pipe, most commonly).
type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

// TestRunStartupTraceEncodeFailureReachesUerr proves a
// --startup-trace run that cannot write its JSON result classifies
// the failure through uerr under ClassLocalIO, a local I/O failure
// distinct from a store failure, so it reaches the log the same way
// every other startup failure does, rather than surfacing only as a
// bare error string on stderr.
func TestRunStartupTraceEncodeFailureReachesUerr(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	seedStore(t, dbPath, seedAccountSQL,
		`INSERT INTO mailbox (id, account_id, server_id, role, name, sort_order, unread_count, total_count) VALUES (1, 1, 'm1', '', 'Inbox', 0, 0, 0)`)

	logged := uerrtest.Capture(t)

	err := run(context.Background(), dbPath, flags{startupTrace: true}, brokenWriter{}, io.Discard, noopConnector)
	if err == nil {
		t.Fatal("run --startup-trace with an unwritable out succeeded, want a failure")
	}
	uerrErr := uerrtest.AssertClass(t, err, uerr.ClassLocalIO)
	if uerrErr.Op != "main.startup-trace" {
		t.Errorf("Op = %q, want main.startup-trace", uerrErr.Op)
	}

	lines := uerrtest.Lines(t, logged)
	if len(lines) == 0 {
		t.Fatal("the encode failure logged nothing through uerr")
	}
	last := lines[len(lines)-1]
	if last["msg"] != "main.startup-trace" {
		t.Errorf("logged msg = %v, want main.startup-trace", last["msg"])
	}
	if cause, _ := last["cause"].(string); !strings.Contains(cause, "write trace result") {
		t.Errorf("logged cause = %v, want it naming the trace write", last["cause"])
	}
}

// TestRunRefusesSecondInstance proves run's own startup path surfaces
// AcquireInstanceLock's refusal instead of swallowing it (SY-7).
func TestRunRefusesSecondInstance(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	lock, err := platform.AcquireInstanceLock(dbPath)
	if err != nil {
		t.Fatalf("AcquireInstanceLock: %v", err)
	}
	defer func() { _ = lock.Release() }()

	var out bytes.Buffer
	if err := run(context.Background(), dbPath, flags{}, &out, io.Discard, noopConnector); err == nil {
		t.Fatal("run succeeded against a locked store, want refusal")
	}
}

// seedStoreNeedingRecovery migrates a fresh store at dbPath, seeds one
// account and one undispatched outbox row, then forces the next
// Migrate call against it to fail, the same schema_version collision
// TestRecoverAfterFailedMigration uses against a store now holding
// real preserved data (SY-8).
func seedStoreNeedingRecovery(t *testing.T, dbPath string) {
	t.Helper()

	seedStore(t, dbPath,
		seedAccountSQL,
		`INSERT INTO outbox (id, account_id, kind, payload, created_at) VALUES (1, 1, 'move-messages', '{}', 0)`,
		`UPDATE schema_version SET version = 0`)
}

// TestPrepareStoreRecoversFromFailedMigration proves prepareStore
// rebuilds the store when Migrate fails and --recover was given,
// rather than propagating the failure and leaving startup dead in the
// water (SY-8).
func TestPrepareStoreRecoversFromFailedMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	seedStoreNeedingRecovery(t, dbPath)

	var out bytes.Buffer
	if err := prepareStore(context.Background(), dbPath, flags{recover: true}, &out); err != nil {
		t.Fatalf("prepareStore: %v", err)
	}

	if !strings.Contains(out.String(), "rebuilding from local data") {
		t.Errorf("output = %q, want a rebuild status line", out.String())
	}

	db, err := store.OpenWriteConn(dbPath)
	if err != nil {
		t.Fatalf("reopen rebuilt store: %v", err)
	}
	defer func() { _ = db.Close() }()

	var outboxCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id = 1`).Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	if outboxCount != 1 {
		t.Errorf("outbox rows after recovery = %d, want 1 (the preserved row must survive the rebuild)", outboxCount)
	}
}

// TestPrepareStoreOffersRecoveryWithoutRebuilding proves prepareStore
// refuses to rebuild on its own: without --recover, a failed
// migration is reported with instructions and returned as an error,
// and the store is left exactly as found, still recoverable on
// request. Recovery is an offer, not an automatic response (SY-8).
func TestPrepareStoreOffersRecoveryWithoutRebuilding(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	seedStoreNeedingRecovery(t, dbPath)

	var out bytes.Buffer
	err := prepareStore(context.Background(), dbPath, flags{}, &out)
	if err == nil {
		t.Fatal("prepareStore without --recover succeeded despite a failed migration, want a refusal")
	}
	if !strings.Contains(out.String(), "--recover") {
		t.Errorf("output = %q, want instructions naming --recover", out.String())
	}

	matches, globErr := filepath.Glob(dbPath + ".corrupt-*")
	if globErr != nil {
		t.Fatalf("glob for a quarantine file: %v", globErr)
	}
	if len(matches) != 0 {
		t.Errorf("refusal quarantined the store anyway: %v", matches)
	}

	counts, err := store.Recover(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Recover after a refused offer: %v", err)
	}
	if counts.Outbox != 1 {
		t.Errorf("RecoveredCounts = %+v, want the refused store still recoverable", counts)
	}
}

// TestPrepareStorePropagatesSchemaVersion proves prepareStore does not
// route a ClassSchemaVersion failure (a store a newer poplar migrated
// forward) into recovery: that store is not corrupt, and rebuilding it
// at this build's older schema would destroy the newer data instead of
// telling the operator to upgrade.
func TestPrepareStorePropagatesSchemaVersion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	seed, err := store.OpenWriteConn(dbPath)
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	if err := store.Migrate(seed); err != nil {
		t.Fatalf("seed Migrate: %v", err)
	}
	if _, err := seed.Exec(`UPDATE schema_version SET version = version + 1`); err != nil {
		t.Fatalf("advance schema_version past this build's known maximum: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed connection: %v", err)
	}

	var out bytes.Buffer
	err = prepareStore(context.Background(), dbPath, flags{recover: true}, &out)
	if err == nil {
		t.Fatal("prepareStore over a newer schema version succeeded, want a refusal")
	}
	// A newer store must not be routed into recovery.
	_ = uerrtest.AssertClass(t, err, uerr.ClassSchemaVersion)

	matches, globErr := filepath.Glob(dbPath + ".corrupt-*")
	if globErr != nil {
		t.Fatalf("glob for a quarantine file: %v", globErr)
	}
	if len(matches) != 0 {
		t.Errorf("a newer store was quarantined and rebuilt anyway: %v", matches)
	}
}

// TestPrepareStorePropagatesCancellation proves a context already
// cancelled when CheckIntegrity runs surfaces as a plain cancellation,
// not a misclassified store failure that triggers a rebuild offer: a
// Ctrl-C during the multi-second quick_check window (QA-1) must not
// read as corruption.
func TestPrepareStorePropagatesCancellation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	err := prepareStore(ctx, dbPath, flags{}, &out)
	if err == nil {
		t.Fatal("prepareStore with an already-cancelled context succeeded, want the cancellation to surface")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled in its chain", err)
	}
	if uerrErr, ok := errors.AsType[uerr.Error](err); ok {
		t.Errorf("cancellation was classified as a uerr.Error (%+v), want a plain cancellation", uerrErr)
	}
	if strings.Contains(out.String(), "rebuilding") || strings.Contains(out.String(), "needs recovery") {
		t.Errorf("output = %q, a cancelled integrity check must not offer or run a rebuild", out.String())
	}
}

// TestReportStartupFailurePrintsCause proves reportStartupFailure
// prints a uerr.Error's cause alongside its fixed sentence: the
// sentence is the same string for every error of its class, so the
// pid a locked instance names, or a failed migration's instructions,
// live only in the cause (SY-7, ADR-0015).
func TestReportStartupFailurePrintsCause(t *testing.T) {
	err := uerr.New("test.op", nil, uerr.ClassInstanceLocked, errors.New("poplar is already running (pid 4242); stop it before starting another instance"))

	var out bytes.Buffer
	reportStartupFailure(&out, err)

	if !strings.Contains(out.String(), "Poplar is already running") {
		t.Errorf("output = %q, want the fixed sentence", out.String())
	}
	if !strings.Contains(out.String(), "pid 4242") {
		t.Errorf("output = %q, want the cause naming the pid", out.String())
	}
}

// TestRunWaitsForEnginesBeforeCleanShutdown proves run's shutdown
// path (ctx cancellation, standing in for SIGINT/SIGTERM) actually
// waits for the sync worker and outbox dispatcher to return before
// closing the writer and marking a clean shutdown, driven against a
// fake backend rather than a live network reach. The fake's
// ChangesFunc blocks on release until the test lets it go, so
// cancelling ctx races the shutdown path against an engine
// demonstrably still live, not one that merely might not have
// finished yet: run must not return while release is held closed.
func TestRunWaitsForEnginesBeforeCleanShutdown(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	connect := connectorBlockingOnFirstChanges(entered, release)

	ctx, cancel := context.WithCancel(context.Background())
	out := newReadySignal()
	done := make(chan error, 1)
	go func() { done <- run(ctx, dbPath, flags{}, out, io.Discard, connect) }()

	select {
	case <-out.ready:
	case err := <-done:
		t.Fatalf("run exited before reaching startup, output %q: %v", out.String(), err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for run to start")
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the sync worker to enter Changes")
	}

	cancel()

	select {
	case err := <-done:
		t.Fatalf("run returned while an engine was still blocked in Changes, want it to wait; err = %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if _, err := os.Stat(dbPath + ".clean-shutdown"); err == nil {
		t.Fatal("clean-shutdown marker written while an engine was still blocked in Changes")
	}

	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for run to return after releasing the blocked engine")
	}
	if _, err := os.Stat(dbPath + ".clean-shutdown"); err != nil {
		t.Errorf("clean-shutdown marker missing after the engines actually stopped: %v", err)
	}
}

// TestRunEnsureAccountReusesExistingRowOnSecondStart proves a second
// start against the same store finds ensureAccount's existing account
// row rather than trying to insert another one under the same slug: a
// find-that-fails-open-into-insert helper whose find branch is never
// exercised is a UNIQUE constraint violation waiting for anyone who
// restarts poplar against a store it has already run against once.
func TestRunEnsureAccountReusesExistingRowOnSecondStart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	if _, err := runToCompletion(t, dbPath, flags{}, noopConnector); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := runToCompletion(t, dbPath, flags{}, noopConnector); err != nil {
		t.Fatalf("second run against the same store: %v", err)
	}
}

// TestRunRetriesANonFatalConnectFailureRatherThanExiting proves SY-3's
// no-network resilience at startup: a connect failure that is not a
// rejected or missing credential (isFatalConnect's only fatal case)
// does not abort run. run keeps reporting itself running and starts
// the engines once a later attempt succeeds, rather than the process
// making a reachable server a precondition for existing at all.
func TestRunRetriesANonFatalConnectFailureRatherThanExiting(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	var attempts atomic.Int64
	called := make(chan struct{}, 1)
	connect := func(context.Context) (backend.Backend, string, error) {
		if attempts.Add(1) <= 2 {
			return nil, "", errors.New("jmapsource: dial: dial tcp: connection refused")
		}
		be := &backendtest.Fake{}
		be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
			select {
			case called <- struct{}{}:
			default:
			}
			return backend.ChangeSet{}, nil
		}
		return be, "test-account", nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	out := newReadySignal()
	done := make(chan error, 1)
	go func() { done <- run(ctx, dbPath, flags{}, out, io.Discard, connect) }()

	select {
	case <-out.ready:
	case err := <-done:
		t.Fatalf("run exited on a non-fatal connect failure instead of retrying, output %q: %v", out.String(), err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for run to report itself running")
	}

	select {
	case <-called:
	case err := <-done:
		t.Fatalf("run exited before the retried connect ever succeeded: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the retried connect to start the engines")
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}
}

// TestRunFailsFastOnAFatalConnectError proves the rejected-credential
// case stays fatal and stays visible: run returns immediately without
// retrying, the operator reads the fixed ClassAuth sentence rather
// than the transport's raw status text, and the failure reaches the
// log as exactly one main.connect line (ER-1's correlation between
// what the user sees and what the log records, under ADR-0013
// revision 2's one-line-per-outcome rule).
//
// Both shapes a real connector produces run through it, because run's
// fatal branch has to treat them differently to log once.
// jmapsource.Dial's rejected session comes back as a wrapped
// backend.Failure, classified but deliberately unlogged so a
// retry loop can dedup it, and run's exit is the only place its
// uerr.Error can come from. keyring.Token's
// missing-token failure has already been through uerr.New inside
// connectLiveJMAP, before any network reach, so constructing a second
// one for it would log the same outcome twice.
func TestRunFailsFastOnAFatalConnectError(t *testing.T) {
	rejectedSession := fmt.Errorf("jmapsource: dial: %w", backend.Failure{
		Class: uerr.ClassAuth,
		Cause: errors.New("session https://api.fastmail.com/jmap/session: unexpected status 401"),
	})

	tests := []struct {
		name       string
		connectErr func() error
		wantCause  string
	}{
		{
			name:       "a session the server rejected",
			connectErr: func() error { return rejectedSession },
			wantCause:  "unexpected status 401",
		},
		{
			// uerr.New logs on construction, so this shape must be
			// built inside the test case to land in that case's own
			// captured buffer.
			name: "a token poplar never found",
			connectErr: func() error {
				return uerr.New("main.connect", nil, uerr.ClassAuth,
					errors.New("keyring: no fastmail token: set account config or FASTMAIL_API_TOKEN"))
			},
			wantCause: "set account config or FASTMAIL_API_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "store.db")
			logged := uerrtest.Capture(t)

			var attempts atomic.Int64
			connect := func(context.Context) (backend.Backend, string, error) {
				attempts.Add(1)
				return nil, "", tt.connectErr()
			}

			var out bytes.Buffer
			err := run(t.Context(), dbPath, flags{}, &out, io.Discard, connect)
			if err == nil {
				t.Fatal("run succeeded despite a fatal connect failure, want it returned immediately")
			}
			if n := attempts.Load(); n != 1 {
				t.Errorf("connect called %d time(s), want exactly 1 (a fatal failure must not retry)", n)
			}

			var stderr bytes.Buffer
			reportStartupFailure(&stderr, err)
			if !strings.Contains(stderr.String(), "Sign-in was rejected") {
				t.Errorf("stderr = %q, want the fixed ClassAuth sentence", stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantCause) {
				t.Errorf("stderr = %q, want the cause naming %q", stderr.String(), tt.wantCause)
			}

			lines := uerrtest.Lines(t, logged)
			if len(lines) != 1 {
				t.Fatalf("got %d log line(s), want exactly 1 main.connect: %v", len(lines), lines)
			}
			if lines[0]["msg"] != "main.connect" {
				t.Errorf("msg = %v, want %q", lines[0]["msg"], "main.connect")
			}
			if lines[0]["class"] != "auth" {
				t.Errorf("class = %v, want %q", lines[0]["class"], "auth")
			}
		})
	}
}

// TestApplyLogLevelOpensTheDebugLines covers the one thing standing
// between an operator and every slog.Debug call in the tree. uerr's
// handler is built at Info, and this is its only lever in pass 1b, so
// without the wiring the debug lines the engines write are filtered
// before they reach the file, and a comment claiming one of them is
// the operator's only record of some event is simply false.
func TestApplyLogLevelOpensTheDebugLines(t *testing.T) {
	tests := []struct {
		name      string
		env       string
		wantDebug bool
	}{
		{"unset", "", false},
		{"debug", "debug", true},
		{"debug in capitals", "DEBUG", true},
		{"some other level", "info", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture restores uerr's level as well as its destination,
			// so a case that raised the level leaves none behind.
			logged := uerrtest.Capture(t)
			previous := slog.Default()
			t.Cleanup(func() { slog.SetDefault(previous) })

			t.Setenv(debugLogEnv, tt.env)
			applyLogLevel(os.Getenv(debugLogEnv))
			uerr.SetDefault()

			// The line internal/backend/jmapsource writes when a push
			// connection drops, verbatim.
			slog.Debug("jmapsource: push connection lost, the transport is reconnecting")
			slog.Info("poplar: store ready")

			lines := uerrtest.Lines(t, logged)
			var levels []any
			for _, line := range lines {
				levels = append(levels, line["level"])
			}
			want := []any{"INFO"}
			if tt.wantDebug {
				want = []any{"DEBUG", "INFO"}
			}
			if !slices.Equal(levels, want) {
				t.Errorf("%s=%q logged levels %v, want %v", debugLogEnv, tt.env, levels, want)
			}
		})
	}
}
