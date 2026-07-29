package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/platform"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/uerr"
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

// connectorSignalingFirstChanges returns a backendConnector whose fake
// backend sends on called the first time its Mail Changes method
// runs, a real sign the sync worker started that a caller can wait on
// before stopping it, rather than racing a fresh goroutine's own
// scheduling against an immediate cancellation.
func connectorSignalingFirstChanges(called chan<- struct{}) backendConnector {
	return func(context.Context) (backend.Backend, string, error) {
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
// the next run's NeedsIntegrityCheck skips its check.
func TestRunStartsAndShutsDownCleanly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	if _, err := runToCompletion(t, dbPath, flags{}, noopConnector); err != nil {
		t.Fatalf("run: %v", err)
	}

	if _, err := os.Stat(dbPath + ".clean-shutdown"); err != nil {
		t.Errorf("clean-shutdown marker missing after a graceful run: %v", err)
	}
	if store.NeedsIntegrityCheck(dbPath, false) {
		t.Error("NeedsIntegrityCheck after a clean run = true, want false")
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
	if store.NeedsIntegrityCheck(dbPath, false) {
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
	var uerrErr uerr.Error
	if !errors.As(err, &uerrErr) {
		t.Fatalf("error is not a uerr.Error: %v", err)
	}
	if uerrErr.Class != uerr.ClassSchemaVersion {
		t.Errorf("Class = %v, want %v; a newer store must not be routed into recovery", uerrErr.Class, uerr.ClassSchemaVersion)
	}

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

// TestRunEnginesStopBeforeCleanShutdown proves run's shutdown path
// (ctx cancellation, standing in for SIGINT/SIGTERM) stops the sync
// worker and outbox dispatcher, closes the writer, and marks a clean
// shutdown, driven against a fake backend rather than a live network
// reach.
func TestRunEnginesStopBeforeCleanShutdown(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	called := make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	out := newReadySignal()
	done := make(chan error, 1)
	go func() { done <- run(ctx, dbPath, flags{}, out, io.Discard, connectorSignalingFirstChanges(called)) }()

	// The engines get a real chance to call Changes before shutdown
	// stops them: cancelling as soon as the ready line prints races
	// run's freshly spawned goroutines against their own first
	// scheduling, which the writer's bulk lane can lose against an
	// already-cancelled ctx (RunPush's immediate poll flush finds
	// nothing left to call).
	select {
	case <-called:
	case err := <-done:
		t.Fatalf("run exited before the sync worker called Changes, output %q: %v", out.String(), err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the sync worker to call Changes")
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}

	if _, err := os.Stat(dbPath + ".clean-shutdown"); err != nil {
		t.Errorf("clean-shutdown marker missing after the engines stopped: %v", err)
	}
}
