package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
)

// killHarnessRunEnv is the environment variable
// TestQA6KillHarnessSmoke sets to re-invoke its own test binary as the
// kill harness driver, the same self-exec trick
// cmd/poplar/main_test.go uses to run the real main as a subprocess.
const killHarnessRunEnv = "POPLAR_KILL_HARNESS_RUN"

// killHarnessDBPathEnv names the store file killHarnessRunEnv's
// subprocess drives.
const killHarnessDBPathEnv = "POPLAR_KILL_HARNESS_DB"

// Fixed ids the driver's seed step assigns explicitly, rather than
// trusting autoincrement: the parent process asserts against the
// killed subprocess's on-disk state with no way to ask it what ids it
// chose, so the seed order and these constants must agree by
// construction.
const (
	killHarnessAccountID    = 1
	killHarnessInboxID      = 1
	killHarnessArchiveID    = 2
	killHarnessProjectsID   = 3
	killHarnessMessageCount = 5

	killHarnessArchiveServerID = "mbx-archive"
	killHarnessRenamedProjects = "Projects Renamed"
	killHarnessStepDelay       = 4 * time.Millisecond
	killHarnessDispatchDelay   = 4 * time.Millisecond
)

// killHarnessMsgID returns the fixed internal id the driver's seed
// loop gives message index i (0-based).
func killHarnessMsgID(i int) int64 { return int64(i + 1) }

// killHarnessDriverState carries what killActionTable's entries need
// to run one action against the store a kill harness trial is
// building.
type killHarnessDriverState struct {
	w          *store.Writer
	d          *Dispatcher
	archiveID  int64
	projectsID int64
	now        time.Time
}

// killAction is one action kind the QA-6 kill harness can perform
// against a live store, table-driven so a later pass adds support by
// adding an entry here rather than editing the driver's control flow.
type killAction struct {
	name string
	run  func(ctx context.Context, h *killHarnessDriverState) error
}

// killActionTable is the fixed smoke-scope script: two single-message
// triage moves, one two-message bulk move, and one folder rename, the
// three action kinds whose subsystems exist by the end of pass 1.
// qa6ActionKinds names the full QA-6 taxonomy this table draws from.
var killActionTable = []killAction{
	{name: "triage", run: func(ctx context.Context, h *killHarnessDriverState) error {
		return h.moveAndDispatch(ctx, []int64{killHarnessMsgID(0)})
	}},
	{name: "triage", run: func(ctx context.Context, h *killHarnessDriverState) error {
		return h.moveAndDispatch(ctx, []int64{killHarnessMsgID(1)})
	}},
	{name: "bulk", run: func(ctx context.Context, h *killHarnessDriverState) error {
		return h.moveAndDispatch(ctx, []int64{killHarnessMsgID(2), killHarnessMsgID(3)})
	}},
	{name: "folder_rename", run: func(ctx context.Context, h *killHarnessDriverState) error {
		return h.renameAndDispatch(ctx, killHarnessRenamedProjects)
	}},
}

// moveAndDispatch enqueues msgIDs' move to h.archiveID alongside the
// local optimistic mutation ADR-0006 pairs with it, in one
// transaction, then drains the outbox against h.d.
func (h *killHarnessDriverState) moveAndDispatch(ctx context.Context, msgIDs []int64) error {
	err := h.w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		if _, err := EnqueueMoveMessagesChunkTx(tx, killHarnessAccountID, msgIDs, h.archiveID, 0, newUndoGroup(), 0, h.now, h.now); err != nil {
			return err
		}
		for _, id := range msgIDs {
			if err := store.SyncMessageMailboxes(tx, killHarnessAccountID, id, h.now.Unix(), false, []string{killHarnessArchiveServerID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return h.dispatchAndCheck(ctx)
}

// renameAndDispatch enqueues h.projectsID's rename to newName
// alongside its local optimistic mutation in one transaction, then
// drains the outbox against h.d.
func (h *killHarnessDriverState) renameAndDispatch(ctx context.Context, newName string) error {
	err := h.w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		if _, _, err := EnqueueRenameMailboxTx(tx, killHarnessAccountID, h.projectsID, newName, h.now); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE mailbox SET name = ? WHERE id = ?`, newName, h.projectsID)
		return err
	})
	if err != nil {
		return err
	}
	return h.dispatchAndCheck(ctx)
}

func (h *killHarnessDriverState) dispatchAndCheck(ctx context.Context) error {
	result, err := h.d.DispatchOnce(ctx, h.now)
	if err != nil {
		return err
	}
	if len(result.Failures) != 0 {
		return fmt.Errorf("dispatch reported %d unexpected failure(s): %+v", len(result.Failures), result.Failures)
	}
	return nil
}

// runKillHarnessDriver seeds one account, three mailboxes, and five
// messages, then runs killActionTable's script against a fake
// backend that always succeeds, closing the store cleanly at the end.
// TestQA6KillHarnessSmoke's parent process sends it SIGKILL at a
// pseudorandom point before it gets there.
func runKillHarnessDriver(dbPath string) {
	ctx := context.Background()
	now := time.Now()

	w, err := store.Open(dbPath, store.DefaultWriterConfig())
	if err != nil {
		killHarnessDriverFail("open store", err)
	}

	be := newFakeBackend()
	// Sleeping inside the fake's own backend calls widens the real
	// window DispatchOnce leaves a claimed row sitting in
	// 'dispatching' between its claim and finalize transactions, the
	// exact window a kill landing there must survive.
	be.MailSource.ApplyBatchFunc = func(context.Context, []backend.Mutation) (backend.BatchResult, error) {
		time.Sleep(killHarnessDispatchDelay)
		return backend.BatchResult{}, nil
	}
	be.MailSource.RenameMailboxFunc = func(context.Context, string, string) error {
		time.Sleep(killHarnessDispatchDelay)
		return nil
	}

	if err := driverSeedAccount(w); err != nil {
		killHarnessDriverFail("seed account", err)
	}
	if err := driverSeedMailbox(w, killHarnessInboxID, "Inbox", "mbx-inbox"); err != nil {
		killHarnessDriverFail("seed Inbox", err)
	}
	if err := driverSeedMailbox(w, killHarnessArchiveID, "Archive", killHarnessArchiveServerID); err != nil {
		killHarnessDriverFail("seed Archive", err)
	}
	if err := driverSeedMailbox(w, killHarnessProjectsID, "Projects", "mbx-projects"); err != nil {
		killHarnessDriverFail("seed Projects", err)
	}
	for i := range killHarnessMessageCount {
		id := killHarnessMsgID(i)
		if err := driverSeedMessage(w, id, killHarnessInboxID, fmt.Sprintf("msg-%d", id)); err != nil {
			killHarnessDriverFail(fmt.Sprintf("seed message %d", id), err)
		}
	}

	h := &killHarnessDriverState{
		w:          w,
		d:          NewDispatcher(killHarnessAccountID, be, w),
		archiveID:  killHarnessArchiveID,
		projectsID: killHarnessProjectsID,
		now:        now,
	}
	for _, a := range killActionTable {
		time.Sleep(killHarnessStepDelay)
		if err := a.run(ctx, h); err != nil {
			killHarnessDriverFail(a.name, err)
		}
	}

	if err := w.Close(); err != nil {
		killHarnessDriverFail("close writer", err)
	}
	if err := store.MarkCleanShutdown(dbPath); err != nil {
		killHarnessDriverFail("mark clean shutdown", err)
	}
	os.Exit(0)
}

// killHarnessDriverFail reports a genuine driver error, distinct from
// the SIGKILL the parent sends on purpose: the parent tells the two
// apart by exit code, so a real bug in the driver fails the test
// loudly instead of reading as just another kill point.
func killHarnessDriverFail(step string, err error) {
	fmt.Fprintf(os.Stderr, "kill harness driver: %s: %v\n", step, err)
	os.Exit(2)
}

func driverSeedAccount(w *store.Writer) error {
	return w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (?, ?, ?, ?)`,
			killHarnessAccountID, "acct", "jmap", "geoff@example.com")
		return err
	})
}

func driverSeedMailbox(w *store.Writer, id int64, name, serverID string) error {
	return w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO mailbox (id, account_id, name, server_id) VALUES (?, ?, ?, ?)`,
			id, killHarnessAccountID, name, serverID)
		return err
	})
}

func driverSeedMessage(w *store.Writer, id, mailboxID int64, serverID string) error {
	return w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		now := time.Now().Unix()
		if _, err := tx.Exec(`INSERT INTO message (id, account_id, server_id, received_at) VALUES (?, ?, ?, ?)`,
			id, killHarnessAccountID, serverID, now); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at) VALUES (?, ?, ?)`,
			id, mailboxID, now)
		return err
	})
}
