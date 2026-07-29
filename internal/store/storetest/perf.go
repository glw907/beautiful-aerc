package storetest

import (
	"database/sql"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/store"
)

// CommonWords, MediumWords, and RareWords are the small committed
// vocabulary SeedPerfEnvelope amplifies into a QA-5-scale message
// set: common words give every search class a realistic hit rate
// against most of the index, medium words a moderate hit rate, and
// rare words a small, known match count for the low-selectivity
// classes, without needing the private mail corpus the Phase 4 spike
// measured against.
var (
	CommonWords = []string{
		"meeting", "invoice", "update", "review", "report", "schedule",
		"budget", "project", "release", "attached", "proposal", "summary",
		"deadline", "status", "feedback", "roadmap", "ticket", "customer",
	}
	MediumWords = []string{"onboarding", "vendor", "compliance", "migration"}
	RareWords   = []string{"xylograph", "umbraculum", "zenithal", "quixotry"}
)

// PerfEnvelope is what SeedPerfEnvelope built: the ids a perf harness
// scripts its reads and searches against, since a fresh store's
// autoincrement ids are otherwise only knowable by re-querying what
// was just written.
type PerfEnvelope struct {
	MailboxIDs []int64
	MessageIDs []int64
}

// SeedPerfEnvelope migrates a fresh store at path and fills it with
// messageCount messages spread across mailboxCount mailboxes, the
// QA-5 scale envelope (100k messages) the QA-1/2/3 harness runs its
// scripted reads and searches against. It writes directly through a
// raw connection rather than the writer's intent path: this is bulk
// fixture generation, not a production write, the same shape
// storetest.OpenWriter's callers already use at small scale.
func SeedPerfEnvelope(t *testing.T, path string, messageCount, mailboxCount int) PerfEnvelope {
	t.Helper()

	db, err := store.OpenWriteConn(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'perf', 'jmap', 'perf@example.com')`); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	env := PerfEnvelope{MessageIDs: make([]int64, 0, messageCount)}
	for i := range mailboxCount {
		mailboxID := int64(i + 1)
		role := ""
		if i == 0 {
			role = "inbox"
		}
		if _, err := db.Exec(`INSERT INTO mailbox (id, account_id, role, name) VALUES (?, 1, ?, ?)`,
			mailboxID, role, fmt.Sprintf("Mailbox %d", i)); err != nil {
			t.Fatalf("seed mailbox %d: %v", mailboxID, err)
		}
		env.MailboxIDs = append(env.MailboxIDs, mailboxID)
	}

	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: a fixed seed makes this fixture's shape reproducible across runs, not a security-sensitive use
	receivedBase := time.Now().Add(-2 * 365 * 24 * time.Hour).Unix()

	const batchSize = 2000
	for batchStart := 0; batchStart < messageCount; batchStart += batchSize {
		batchEnd := min(batchStart+batchSize, messageCount)
		if err := perfSeedBatch(db, env.MailboxIDs, rng, receivedBase, batchStart, batchEnd); err != nil {
			t.Fatalf("seed messages %d..%d: %v", batchStart, batchEnd, err)
		}
		for i := batchStart; i < batchEnd; i++ {
			env.MessageIDs = append(env.MessageIDs, int64(i+1))
		}
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close seed connection: %v", err)
	}
	// MarkCleanShutdown requires every connection over path closed
	// first, so the caller's next store.Open skips the integrity check
	// QA-1 measures around.
	if err := store.MarkCleanShutdown(path); err != nil {
		t.Fatalf("mark clean shutdown: %v", err)
	}
	return env
}

// perfSeedBatch inserts one transaction's worth of messages
// [batchStart, batchEnd), each into exactly one of mailboxIDs weighted
// so the first (the seeded inbox) holds most of the traffic, the same
// skew a real account's Inbox vs Archive split has.
func perfSeedBatch(db *sql.DB, mailboxIDs []int64, rng *rand.Rand, receivedBase int64, batchStart, batchEnd int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	messageStmt, err := tx.Prepare(`
		INSERT INTO message (id, account_id, thread_key, received_at, subject, from_addr, flags, search_text)
		VALUES (?, 1, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare message insert: %w", err)
	}
	defer func() { _ = messageStmt.Close() }()

	mailboxStmt, err := tx.Prepare(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at, unread) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare message_mailbox insert: %w", err)
	}
	defer func() { _ = mailboxStmt.Close() }()

	bodyStmt, err := tx.Prepare(`INSERT INTO body (message_id, content, fetched_at) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare body insert: %w", err)
	}
	defer func() { _ = bodyStmt.Close() }()

	for i := batchStart; i < batchEnd; i++ {
		id := int64(i + 1)
		receivedAt := receivedBase + int64(i)*97
		subject := perfRandomText(rng, 4)
		searchText := perfRandomText(rng, 20)
		flags, unread := 0, 0
		if rng.IntN(5) != 0 {
			flags, unread = 1, 0 // FlagSeen
		} else {
			unread = 1
		}
		mailboxID := perfPickMailbox(rng, mailboxIDs)

		if _, err := messageStmt.Exec(id, fmt.Sprintf("thread-%d", i/6), receivedAt, subject, "sender@example.com", flags, searchText); err != nil {
			return fmt.Errorf("insert message %d: %w", id, err)
		}
		if _, err := mailboxStmt.Exec(id, mailboxID, receivedAt, unread); err != nil {
			return fmt.Errorf("insert message_mailbox %d: %w", id, err)
		}
		if _, err := bodyStmt.Exec(id, searchText, receivedAt); err != nil {
			return fmt.Errorf("insert body %d: %w", id, err)
		}
	}

	return tx.Commit()
}

// perfPickMailbox weights mailboxIDs[0] (the seeded inbox) at roughly
// 80% of traffic, splitting the rest evenly across the remaining
// mailboxes, so a folder-switch operation lands on mailboxes that
// actually hold mail.
func perfPickMailbox(rng *rand.Rand, mailboxIDs []int64) int64 {
	if len(mailboxIDs) == 1 || rng.IntN(5) != 0 {
		return mailboxIDs[0]
	}
	return mailboxIDs[1+rng.IntN(len(mailboxIDs)-1)]
}

// perfRandomText draws words words from CommonWords, salting in a
// medium word roughly one time in fifty and a rare word roughly one
// time in five hundred, so the search harness's single-term class has
// a genuine common/medium/rare spread of hit rates to measure against
// rather than a vocabulary where every term matches nearly every
// document.
func perfRandomText(rng *rand.Rand, words int) string {
	picked := make([]string, words)
	for i := range picked {
		switch {
		case rng.IntN(500) == 0:
			picked[i] = RareWords[rng.IntN(len(RareWords))]
		case rng.IntN(50) == 0:
			picked[i] = MediumWords[rng.IntN(len(MediumWords))]
		default:
			picked[i] = CommonWords[rng.IntN(len(CommonWords))]
		}
	}
	return strings.Join(picked, " ")
}

// Measure runs op exactly count times through testing.B.Loop, forcing
// that exact iteration count with -benchtime's Nx form rather than the
// default calibrated duration: a QA harness names a fixed script or
// query set, not however many iterations fit in a second. It captures
// op's own reported duration per call rather than B's mean, since a
// p95/p99 budget needs the distribution a mean hides.
func Measure(t *testing.T, count int, op func() time.Duration) (samples []time.Duration, benchLine string) {
	t.Helper()

	if err := flag.Set("test.benchtime", fmt.Sprintf("%dx", count)); err != nil {
		t.Fatalf("set benchtime: %v", err)
	}
	result := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			samples = append(samples, op())
		}
	})
	return samples, result.String()
}

// Percentile returns the p-th percentile (0-100) of samples,
// nearest-rank over the sorted set.
func Percentile(samples []time.Duration, p float64) time.Duration {
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	idx := int(p / 100 * float64(len(sorted)-1))
	return sorted[idx]
}

// WriteBaseline records name's baseline summary under dir, a path
// relative to the calling package's own directory: the go-test-style
// benchmark line plus this run's percentiles. Unlike t.ArtifactDir(),
// which Go deletes after the test unless -artifacts is passed, dir is
// a committed testdata directory, so the file survives past the test
// that wrote it for a later pass to benchstat a fresh run against.
// WriteBaseline never overwrites an existing file: the baseline is a
// fixed reference point, not a number that moves every time someone
// runs go test. Deleting the file is how a pass deliberately rebases.
func WriteBaseline(t *testing.T, dir, name, benchLine string, samples []time.Duration) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}

	path := filepath.Join(dir, name+".txt")
	if _, err := os.Stat(path); err == nil {
		return
	}

	summary := fmt.Sprintf("Benchmark%s %s\nn=%d p50=%s p95=%s p99=%s max=%s\n",
		name, benchLine, len(samples),
		Percentile(samples, 50), Percentile(samples, 95), Percentile(samples, 99),
		Percentile(samples, 100))
	if err := os.WriteFile(path, []byte(summary), 0o644); err != nil { //nolint:gosec // G306: a perf baseline is diagnostic output alongside the test binary, not sensitive data
		t.Fatalf("write baseline %s: %v", path, err)
	}
}
