package outbox

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/store"
)

// killHarnessSeed, killHarnessTrials, and killHarnessMaxSleep are
// QA-6's smoke scope at pass 1: one seed, driving the fixed
// killActionTable script through enough independent kill trials to
// spread across its whole runtime. Full scope (30 actions, 200 kill
// points, three seeds) is a pass-6 CI job, not this test.
const (
	killHarnessSeed     = 1
	killHarnessTrials   = 20
	killHarnessMaxSleep = 120 * time.Millisecond
)

// qa6ActionKinds is QA-6's full action taxonomy (requirements
// QA-6: triage, bulk, compose, send, RSVP, event edit, folder
// rename). implemented flips to true in the same change that gives an
// action a durable subsystem to drive and adds its killActionTable
// entry: compose and send in pass 4, RSVP and event edit in pass 5.
var qa6ActionKinds = []struct {
	name        string
	implemented bool
}{
	{"triage", true},
	{"bulk", true},
	{"compose", false},
	{"send", false},
	{"rsvp", false},
	{"event_edit", false},
	{"folder_rename", true},
}

// qa6KindActions names, for every intent kind this package declares,
// the killActionTable actions that drive it. A kind maps to nothing
// when QA-6's taxonomy has no category for it: creating and deleting a
// mailbox are neither triage, bulk, nor a folder rename.
var qa6KindActions = map[Kind][]string{
	KindCreateMailbox: nil,
	KindDeleteMailbox: nil,
	KindRenameMailbox: {"folder_rename"},
	KindMoveMessages:  {"triage", "bulk"},
}

// TestActionTableComplete asserts killActionTable names exactly the
// QA-6 action kinds this build's engines support: a later pass that
// flips an entry in qa6ActionKinds to implemented without adding its
// killActionTable entry, or vice versa, fails here instead of
// shipping a kill harness that quietly skipped the new action. It
// reads the Kind constants back out of the package's own source, so a
// pass that adds an intent kind fails here until it says which action
// drives the new kind.
func TestActionTableComplete(t *testing.T) {
	var want []string
	for _, k := range qa6ActionKinds {
		if k.implemented {
			want = append(want, k.name)
		}
	}

	seen := map[string]bool{}
	for _, a := range killActionTable {
		seen[a.name] = true
	}

	for _, name := range want {
		if !seen[name] {
			t.Errorf("killActionTable has no entry for %q, a QA-6 action kind this build supports", name)
		}
	}
	for name := range seen {
		if !slices.Contains(want, name) {
			t.Errorf("killActionTable has an entry for %q, which qa6ActionKinds does not mark implemented", name)
		}
	}

	for _, kind := range declaredKinds(t) {
		actions, classified := qa6KindActions[kind]
		if !classified {
			t.Errorf("intent kind %q is missing from qa6KindActions: name the killActionTable actions that drive it, or none if QA-6 has no category for it", kind)
			continue
		}
		for _, name := range actions {
			if !seen[name] {
				t.Errorf("qa6KindActions drives intent kind %q with action %q, which killActionTable has no entry for", kind, name)
			}
		}
	}
}

// declaredKinds returns every Kind constant the package's non-test
// source declares.
func declaredKinds(t *testing.T) []Kind {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	fset := token.NewFileSet()
	var kinds []Kind
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				if value, ok := kindConstValue(spec); ok {
					kinds = append(kinds, value)
				}
			}
		}
	}
	if len(kinds) == 0 {
		t.Fatal("found no Kind constants in the package source, so this test proves nothing")
	}
	return kinds
}

// kindConstValue returns the value spec's Kind literal and reports
// whether it declares one.
func kindConstValue(spec ast.Spec) (Kind, bool) {
	value, ok := spec.(*ast.ValueSpec)
	if !ok || len(value.Values) != 1 {
		return "", false
	}
	if name, ok := value.Type.(*ast.Ident); !ok || name.Name != "Kind" {
		return "", false
	}
	lit, ok := value.Values[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	unquoted, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return Kind(unquoted), true
}

// TestMain lets this package's test binary double as the kill
// harness's driver subprocess: re-invoked with killHarnessRunEnv set,
// it runs the driver instead of the test suite, the same self-exec
// trick cmd/poplar/main_test.go uses to run the real main.
func TestMain(m *testing.M) {
	if os.Getenv(killHarnessRunEnv) != "" {
		runKillHarnessDriver(os.Getenv(killHarnessDBPathEnv))
		return
	}
	os.Exit(m.Run())
}

// TestQA6KillHarnessSmoke is QA-6's kill harness at pass 1 scope: each
// trial runs killActionTable's fixed script in a subprocess against a
// fresh store, SIGKILLs it at a pseudorandom point drawn from
// killHarnessSeed, and asserts the surviving store's invariants.
func TestQA6KillHarnessSmoke(t *testing.T) {
	rng := rand.New(rand.NewPCG(killHarnessSeed, killHarnessSeed)) //nolint:gosec // G404: a fixed seed makes this smoke run's kill points reproducible across CI runs, not a security-sensitive use

	for trial := range killHarnessTrials {
		t.Run(fmt.Sprintf("trial-%d", trial), func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "store.db")
			sleep := time.Duration(rng.Int64N(int64(killHarnessMaxSleep)))

			runAndKill(t, dbPath, sleep)
			assertKillHarnessInvariants(t, dbPath)
		})
	}
}

// runAndKill starts the kill harness driver against dbPath and sends
// it SIGKILL after sleep, failing the test only when the driver's own
// exit does not fit one of the two legitimate shapes: it finished
// before the kill landed, or the kill itself ended it.
func runAndKill(t *testing.T, dbPath string, sleep time.Duration) {
	t.Helper()

	cmd := exec.Command(os.Args[0]) //nolint:gosec // G204: os.Args[0] is this same test binary, re-invoked as its own subprocess
	cmd.Env = append(os.Environ(), killHarnessRunEnv+"=1", killHarnessDBPathEnv+"="+dbPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start kill harness driver: %v", err)
	}

	time.Sleep(sleep)
	_ = cmd.Process.Kill()

	waitErr := cmd.Wait()
	if !driverExitedCleanly(waitErr) {
		t.Fatalf("driver exited unexpectedly: %v\nstderr:\n%s", waitErr, stderr.String())
	}
}

// driverExitedCleanly reports whether waitErr matches one of the two
// legitimate outcomes runAndKill's kill can produce: nil, the driver
// finished before the kill was delivered, or a SIGKILL termination,
// the kill landing as intended. Anything else, an exit code the
// driver's own failure path chose, is a genuine bug to fail loudly on.
func driverExitedCleanly(waitErr error) bool {
	if waitErr == nil {
		return true
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGKILL
}

// restartStore takes dbPath through the sequence cmd/poplar's run
// performs on every startup: migrate, the integrity check a missing
// clean-shutdown marker leaves owed, then the sweep that requeues
// intents a mid-dispatch death stranded. internal/outbox cannot import
// the main package, so the harness reproduces the order rather than
// calling run.
func restartStore(t *testing.T, dbPath string) {
	t.Helper()

	db, err := store.OpenWriteConn(dbPath)
	if err != nil {
		t.Fatalf("reopen store after kill: %v", err)
	}

	before, err := store.CurrentSchemaVersion(db)
	if err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("re-migrate after kill: %v", err)
	}
	after, err := store.CurrentSchemaVersion(db)
	if err != nil {
		t.Fatalf("read schema version after migrate: %v", err)
	}
	if store.NeedsIntegrityCheck(dbPath, after != before) {
		if err := store.CheckIntegrity(context.Background(), db, nil); err != nil {
			t.Fatalf("CheckIntegrity after a kill (QA-6 corruption): %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close migration connection: %v", err)
	}

	w, err := store.Open(dbPath, store.DefaultWriterConfig())
	if err != nil {
		t.Fatalf("open writer after kill: %v", err)
	}
	if err := ReclaimOrphaned(context.Background(), w); err != nil {
		t.Fatalf("reclaim intents stranded by the kill: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer after reclaim: %v", err)
	}
}

// assertKillHarnessInvariants restarts the store at dbPath as a real
// poplar startup would and asserts QA-6's restart invariants against
// whatever the killed driver left behind.
func assertKillHarnessInvariants(t *testing.T, dbPath string) {
	t.Helper()

	restartStore(t, dbPath)

	db, err := store.OpenWriteConn(dbPath)
	if err != nil {
		t.Fatalf("reopen store after restart: %v", err)
	}

	assertNoStrandedDispatching(t, db)
	assertControlMessageUntouched(t, db)
	assertMoveIntentsMatchLocalState(t, db)
	assertRenameIntentsMatchLocalState(t, db)
	assertProjectsMailboxNameValid(t, db)

	if err := db.Close(); err != nil {
		t.Fatalf("close post-kill connection: %v", err)
	}

	assertRevisionMonotonicAfterRestart(t, dbPath)
}

// assertNoStrandedDispatching pins the invariant this pass found the
// hard way: a row a kill catches between DispatchOnce's claim and
// finalize transactions must not survive a restart in 'dispatching',
// since selectEligible never selects it again.
func assertNoStrandedDispatching(t *testing.T, db *sql.DB) {
	t.Helper()

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE state = 'dispatching'`).Scan(&n); err != nil {
		t.Fatalf("count dispatching rows: %v", err)
	}
	if n != 0 {
		t.Errorf("outbox rows stuck in dispatching after a restart = %d, want 0", n)
	}
}

// assertControlMessageUntouched proves the one seeded message no
// killActionTable entry ever targets never moved: a committed
// mutation against it would mean one existed with no outbox row ever
// behind it, QA-6's "none the reverse" half.
func assertControlMessageUntouched(t *testing.T, db *sql.DB) {
	t.Helper()

	control := killHarnessMsgID(killHarnessMessageCount - 1)
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM message_mailbox WHERE message_id = ? AND mailbox_id = ?`,
		control, killHarnessArchiveID).Scan(&n)
	if err != nil {
		t.Fatalf("check control message: %v", err)
	}
	if n != 0 {
		t.Errorf("control message %d moved to Archive though no action ever targeted it", control)
	}
}

// assertMoveIntentsMatchLocalState proves every surviving
// move_messages outbox row's messages already carry the local
// mutation the row's own enqueue transaction paired it with: no
// outbox row exists without its committed mutation.
func assertMoveIntentsMatchLocalState(t *testing.T, db *sql.DB) {
	t.Helper()

	rows, err := db.Query(`SELECT id, payload FROM outbox WHERE kind = ?`, string(KindMoveMessages))
	if err != nil {
		t.Fatalf("query move intents: %v", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id int64
		var payload []byte
		if err := rows.Scan(&id, &payload); err != nil {
			t.Fatalf("scan move intent: %v", err)
		}
		var p MoveMessagesPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			t.Fatalf("decode move intent %d: %v", id, err)
		}
		for _, msgID := range p.MessageIDs {
			var n int
			if err := db.QueryRow(`SELECT COUNT(*) FROM message_mailbox WHERE message_id = ? AND mailbox_id = ?`,
				msgID, p.DestMailboxID).Scan(&n); err != nil {
				t.Fatalf("check message %d membership: %v", msgID, err)
			}
			if n != 1 {
				t.Errorf("outbox move intent %d names message %d for mailbox %d with no local membership row", id, msgID, p.DestMailboxID)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate move intents: %v", err)
	}
}

// assertRenameIntentsMatchLocalState proves every surviving
// rename_mailbox outbox row's mailbox already carries the name the
// row's own enqueue transaction paired it with.
func assertRenameIntentsMatchLocalState(t *testing.T, db *sql.DB) {
	t.Helper()

	rows, err := db.Query(`SELECT id, payload FROM outbox WHERE kind = ?`, string(KindRenameMailbox))
	if err != nil {
		t.Fatalf("query rename intents: %v", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id int64
		var payload []byte
		if err := rows.Scan(&id, &payload); err != nil {
			t.Fatalf("scan rename intent: %v", err)
		}
		var p RenameMailboxPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			t.Fatalf("decode rename intent %d: %v", id, err)
		}
		var name string
		if err := db.QueryRow(`SELECT name FROM mailbox WHERE id = ?`, p.MailboxID).Scan(&name); err != nil {
			t.Fatalf("read mailbox %d name: %v", p.MailboxID, err)
		}
		if name != p.Name {
			t.Errorf("outbox rename intent %d wants mailbox %d named %q, local name is %q", id, p.MailboxID, p.Name, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rename intents: %v", err)
	}
}

// assertProjectsMailboxNameValid proves the Projects mailbox's name
// never drifts to anything but its seeded name or the one rename this
// script ever requests: a third value would be a committed mutation
// no action asked for.
func assertProjectsMailboxNameValid(t *testing.T, db *sql.DB) {
	t.Helper()

	var name string
	err := db.QueryRow(`SELECT name FROM mailbox WHERE id = ?`, killHarnessProjectsID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		t.Fatalf("read Projects mailbox name: %v", err)
	}
	if name != "Projects" && name != killHarnessRenamedProjects {
		t.Errorf("Projects mailbox name = %q, want %q or %q", name, "Projects", killHarnessRenamedProjects)
	}
}

// assertRevisionMonotonicAfterRestart proves a fresh Writer opened
// over the killed store still advances its revision counter strictly
// with each commit, the property SY-1's read path depends on holding
// in the session that follows a restart, not only the one before it.
func assertRevisionMonotonicAfterRestart(t *testing.T, dbPath string) {
	t.Helper()

	w, err := store.Open(dbPath, store.DefaultWriterConfig())
	if err != nil {
		t.Fatalf("reopen store's writer after kill: %v", err)
	}
	defer func() { _ = w.Close() }()

	touch := func() store.Revision {
		if err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.Exec(`UPDATE account SET address = address`)
			return err
		}); err != nil {
			t.Fatalf("write after restart: %v", err)
		}
		return w.Revision().Current()
	}

	first := w.Revision().Current()
	second := touch()
	if second <= first {
		t.Errorf("revision after a post-restart commit = %d, want > %d", second, first)
	}
	third := touch()
	if third <= second {
		t.Errorf("revision after a second post-restart commit = %d, want > %d", third, second)
	}
}
