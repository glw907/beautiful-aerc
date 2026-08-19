package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/uerr/uerrtest"
)

// TestCheckpointLifecycle proves the writer's own checkpoint
// schedule (ADR-0003 revision 2): PASSIVE after each bulk chunk
// keeps the WAL synced but does not shrink its file while a reader
// holds an old snapshot, and TRUNCATE at idle shrinks it back down
// once that snapshot is released.
func TestCheckpointLifecycle(t *testing.T) {
	cfg := DefaultWriterConfig()
	cfg.CheckpointIdle = 30 * time.Millisecond
	w, path := newTestWriter(t, cfg)

	reader := openTestReader(t, path)

	rtx, err := reader.Begin()
	if err != nil {
		t.Fatalf("begin reader tx: %v", err)
	}
	var accounts int
	if err := rtx.QueryRow(`SELECT COUNT(*) FROM account`).Scan(&accounts); err != nil {
		t.Fatalf("hold reader snapshot: %v", err)
	}

	fillWithFatRows(t, w, 200, 4096)

	walPath := path + "-wal"
	grown := walSize(t, walPath)
	if grown < 500_000 {
		t.Fatalf("wal size = %d bytes, want it grown past PASSIVE's reach while the reader held a snapshot", grown)
	}

	if err := rtx.Rollback(); err != nil {
		t.Fatalf("release reader snapshot: %v", err)
	}

	// The writer's idle timer fires the TRUNCATE checkpoint now that
	// nothing holds an old snapshot open, but when it lands is
	// scheduling and IO rather than a fixed multiple of CheckpointIdle.
	// The fixed sleep this replaces went red on a loaded machine while
	// the subject behaved correctly. The deadline is generous because
	// it only bounds the failing case.
	shrunk := walSize(t, walPath)
	for deadline := time.Now().Add(30 * time.Second); shrunk > 4096 && time.Now().Before(deadline); {
		time.Sleep(cfg.CheckpointIdle)
		shrunk = walSize(t, walPath)
	}
	if shrunk > 4096 {
		t.Errorf("wal size after idle = %d bytes, want it back near its bound", shrunk)
	}
}

// TestCheckpointPassiveReclaimsWithoutAReader covers the case
// TestCheckpointLifecycle's blocked reader does not: with nothing
// holding an old snapshot, PASSIVE alone keeps the WAL from growing
// across many bulk chunks. CheckpointIdle is set far longer than the
// test can run, so a TRUNCATE firing mid-test cannot be why the WAL
// stayed small; deleting the PASSIVE call from runBulk fails this
// assertion the same way it would fail TestCheckpointLifecycle's.
func TestCheckpointPassiveReclaimsWithoutAReader(t *testing.T) {
	cfg := DefaultWriterConfig()
	cfg.CheckpointIdle = time.Hour
	w, path := newTestWriter(t, cfg)

	fillWithFatRows(t, w, 200, 4096)

	// runBulk's PASSIVE checkpoint runs after the job's done channel
	// already fired, so give the last chunk's checkpoint room to
	// finish before measuring.
	time.Sleep(50 * time.Millisecond)

	// 450_000 bytes: this workload's flat, PASSIVE-kept WAL measures
	// ~410KB at the schema's page_size=8192 (WAL frames are
	// page_size-sized, so it roughly doubled from the ~200KB territory
	// a 4096-byte page_size left it at). The ceiling stays comfortably
	// under TestCheckpointLifecycle's 500_000-byte floor for the
	// blocked-reader case, so the two assertions keep proving distinct
	// things rather than converging on the same number.
	flat := walSize(t, path+"-wal")
	if flat > 450_000 {
		t.Errorf("wal size with no reader holding a snapshot = %d bytes, want PASSIVE to keep it well under the 500_000 bytes TestCheckpointLifecycle's blocked reader grows past", flat)
	}
}

// TestIncrementalVacuumReclaimsFreelist proves the idle path's bounded
// incremental_vacuum step actually shrinks the freelist a large delete
// leaves behind, rather than only running the pragma without effect:
// with auto_vacuum=INCREMENTAL, a deleted row's pages sit on the
// freelist until reclaimed. CheckpointIdle is set past the test's
// lifetime and the one idle window runs by hand, because an idle
// window short enough to fire on its own also fires before the reader
// gets its first look: under -race this fixture's delete alone takes
// ~400ms, and the freelist then reads as 0 pages, already reclaimed.
// The writer's own timer keeps its coverage in
// TestCheckpointLifecycle, whose TRUNCATE assertion polls until that
// timer fires.
func TestIncrementalVacuumReclaimsFreelist(t *testing.T) {
	cfg := DefaultWriterConfig()
	cfg.CheckpointIdle = time.Hour
	w, path := newTestWriter(t, cfg)

	reader := openTestReader(t, path)

	fillWithFatRows(t, w, 300, 4096)
	if err := w.submit(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM account`)
		return err
	}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	before := freelistCount(t, reader)
	if before == 0 {
		t.Fatal("freelist_count = 0 right after a large delete, want pages awaiting reclaim")
	}
	if before > incrementalVacuumPages {
		t.Fatalf("freelist_count = %d, want it within the %d pages one idle window reclaims, so a single window can clear it", before, incrementalVacuumPages)
	}

	w.runIdleCheckpoint()

	if after := freelistCount(t, reader); after != 0 {
		t.Errorf("freelist_count after the idle window = %d, want 0: incremental_vacuum should reclaim it", after)
	}
}

// TestIncrementalVacuumIsBoundedPerIdleWindow proves the idle path's
// incremental_vacuum step reclaims only incrementalVacuumPages per
// idle window rather than clearing an oversized freelist in one shot:
// with a freelist well past that bound, a regression that dropped the
// pragma's argument (making the call an unbounded incremental_vacuum)
// would clear the whole freelist here and fail this test, the same
// bug TestIncrementalVacuumReclaimsFreelist's smaller freelist cannot
// catch. CheckpointIdle is set past the test's lifetime so the
// writer's own timer never fires; the test drives exactly one idle
// window itself by calling runIdleCheckpoint directly.
func TestIncrementalVacuumIsBoundedPerIdleWindow(t *testing.T) {
	cfg := DefaultWriterConfig()
	cfg.CheckpointIdle = time.Hour
	w, path := newTestWriter(t, cfg)

	reader := openTestReader(t, path)

	fillWithFatRows(t, w, 1000, 8192)
	if err := w.submit(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM account`)
		return err
	}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	before := freelistCount(t, reader)
	if before <= incrementalVacuumPages {
		t.Fatalf("freelist_count = %d, want it to exceed incrementalVacuumPages (%d) so the bound actually binds", before, incrementalVacuumPages)
	}

	w.runIdleCheckpoint()

	after := freelistCount(t, reader)
	if want := before - incrementalVacuumPages; after != want {
		t.Errorf("freelist_count after one idle window = %d, want exactly %d (%d minus the %d-page bound)", after, want, before, incrementalVacuumPages)
	}
}

// TestCheckpointTruncateSurfacesTimeoutRestoreFailure proves a failed
// busy_timeout restore reaches the caller. The writer's pool holds
// exactly one physical connection, so a restore that fails unnoticed
// leaves every later transaction in the process giving up after
// checkpointBusyTimeoutMS instead of busyTimeoutMS, turning ordinary
// lock waits into a rash of store failures with no visible cause.
func TestCheckpointTruncateSurfacesTimeoutRestoreFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	probe, err := OpenWriteConn(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := Migrate(probe); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	base := probe.Driver()
	if err := probe.Close(); err != nil {
		t.Fatalf("close probe connection: %v", err)
	}

	db := sql.OpenDB(rejectingConnector{
		base:   base,
		dsn:    dsn(path, connReadWrite),
		reject: fmt.Sprintf("busy_timeout = %d", busyTimeoutMS),
	})
	db.SetMaxOpenConns(1)
	defer func() { _ = db.Close() }()

	err = checkpointTruncate(context.Background(), db)
	if err == nil {
		t.Fatal("checkpointTruncate whose busy_timeout restore failed = nil, want the failure surfaced")
	}
	if !strings.Contains(err.Error(), "busy_timeout") {
		t.Errorf("err = %v, want it naming the busy_timeout restore", err)
	}
}

// TestCheckpointTruncateWarnsWhenTheWALWillNotDrain proves a TRUNCATE
// checkpoint that SQLite refused to complete leaves a log line. SQLite
// reports that refusal in the pragma's own result row rather than as
// an error, so a checkpoint call that reads only the error learns
// nothing: the WAL keeps growing past its bound and the writer reports
// a clean run. The driver swap is what made this reachable to test at
// all, since both drivers block the full busy_timeout and return
// busy=1 here (ADR-0001 revision 3's corrected condition 4).
func TestCheckpointTruncateWarnsWhenTheWALWillNotDrain(t *testing.T) {
	log := uerrtest.CaptureDefault(t)

	cfg := DefaultWriterConfig()
	cfg.CheckpointIdle = time.Hour
	w, path := newTestWriter(t, cfg)

	reader := openTestReader(t, path)
	rtx, err := reader.Begin()
	if err != nil {
		t.Fatalf("begin reader tx: %v", err)
	}
	defer func() { _ = rtx.Rollback() }()
	var accounts int
	if err := rtx.QueryRow(`SELECT COUNT(*) FROM account`).Scan(&accounts); err != nil {
		t.Fatalf("hold reader snapshot: %v", err)
	}

	fillWithFatRows(t, w, 50, 4096)

	if err := checkpointTruncate(context.Background(), w.db); err != nil {
		t.Fatalf("checkpointTruncate against a reader-pinned WAL = %v, want nil: a refused checkpoint is not a failed one", err)
	}

	line := lastLogLine(t, log.String(), "checkpoint")
	if !strings.Contains(line, "mode=TRUNCATE") {
		t.Errorf("checkpoint warning = %q, want it naming the TRUNCATE mode", line)
	}
	// An empty WAL warns about nothing a reader did, which is how a
	// vacuous version of this test would pass with the snapshot never
	// pinning anything. The frame counts are what rule that out: the
	// WAL held frames, and the checkpoint stopped short of them at the
	// reader's mark.
	logFrames := logAttrInt(t, line, "log_frames")
	checkpointed := logAttrInt(t, line, "checkpointed_frames")
	if logFrames == 0 {
		t.Errorf("checkpoint warning = %q, want a non-zero log_frames: an empty WAL proves nothing about a blocked checkpoint", line)
	}
	if checkpointed >= logFrames {
		t.Errorf("checkpointed_frames = %d, log_frames = %d, want the checkpoint stopped short while the reader held its snapshot", checkpointed, logFrames)
	}
}

// lastLogLine returns the final captured log line containing want,
// failing t when nothing captured does.
func lastLogLine(t *testing.T, captured, want string) string {
	t.Helper()

	var found string
	for line := range strings.SplitSeq(strings.TrimSpace(captured), "\n") {
		if strings.Contains(line, want) {
			found = line
		}
	}
	if found == "" {
		t.Fatalf("no log line mentions %q; captured:\n%s", want, captured)
	}
	return found
}

// logAttrInt returns the integer value slog's text handler wrote for
// key in line.
func logAttrInt(t *testing.T, line, key string) int {
	t.Helper()

	_, rest, ok := strings.Cut(line, key+"=")
	if !ok {
		t.Fatalf("log line %q carries no %s attribute", line, key)
	}
	value, _, _ := strings.Cut(rest, " ")
	n, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("parse %s from %q: %v", key, line, err)
	}
	return n
}

// rejectingConnector opens the real sqlite driver and refuses the one
// statement whose text contains reject, so a test can fail a single
// step of a multi-statement sequence while every other step runs for
// real. checkpointTruncate's restore step is only reachable this way:
// it runs after a checkpoint the same connection has to have
// executed successfully.
type rejectingConnector struct {
	base        driver.Driver
	dsn, reject string
}

func (c rejectingConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.base.Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return rejectingConn{Conn: conn, reject: c.reject}, nil
}

func (c rejectingConnector) Driver() driver.Driver { return c.base }

// rejectingConn embeds driver.Conn as an interface on purpose: the
// wrapper's method set is then Prepare, Begin and Close alone, so
// database/sql routes every statement through Prepare rather than an
// optional Execer the underlying connection implements.
type rejectingConn struct {
	driver.Conn
	reject string
}

func (c rejectingConn) Prepare(query string) (driver.Stmt, error) {
	if strings.Contains(query, c.reject) {
		return nil, errors.New("statement rejected by the test connector")
	}
	return c.Conn.Prepare(query)
}

// fillWithFatRows writes rows bulk chunks, each one oversized account
// row padded to pad bytes: the WAL-growing workload every checkpoint
// test starts from.
func fillWithFatRows(t *testing.T, w *Writer, rows, pad int) {
	t.Helper()

	big := strings.Repeat("x", pad)
	for i := range rows {
		err := w.submitBulk(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.Exec(
				`INSERT INTO account (slug, backend_kind, address, data) VALUES (?, ?, ?, ?)`,
				fmt.Sprintf("acct-%d", i), "jmap", "user@example.com", big)
			return err
		})
		if err != nil {
			t.Fatalf("submitBulk(%d): %v", i, err)
		}
	}
}

// openTestReader opens a read-only connection onto the store at path,
// separate from the writer's own: a query through the writer would run
// on the interactive lane and reset the idle timer these tests wait on.
func openTestReader(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", dsn(path, connReadOnly))
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func freelistCount(t *testing.T, db *sql.DB) int {
	t.Helper()

	var n int
	if err := db.QueryRow(`PRAGMA freelist_count`).Scan(&n); err != nil {
		t.Fatalf("read freelist_count: %v", err)
	}
	return n
}

func walSize(t *testing.T, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}
