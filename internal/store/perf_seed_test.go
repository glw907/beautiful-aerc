//go:build !race

// The QA-1/2/3 perf harness is excluded from the race build: race
// instrumentation costs 2-20x time and 5-10x memory, so a p95 gate
// asserted under it would measure the detector instead of the store
// (build machine section 2). CI's `go test -race ./...` job never
// links this file.
package store

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

// perfCommonWords and perfRareWords are the small committed vocabulary
// perfSeedEnvelope amplifies into a QA-5-scale message set: common
// words give every search class a realistic hit rate, and rare words
// give the negation and low-frequency single-term classes a small,
// known match count, without needing the private mail corpus the
// Phase 4 spike measured against.
var (
	perfCommonWords = []string{
		"meeting", "invoice", "update", "review", "report", "schedule",
		"budget", "project", "release", "attached", "proposal", "summary",
		"deadline", "status", "feedback", "roadmap", "ticket", "customer",
	}
	perfRareWords = []string{"xylograph", "umbraculum", "zenithal", "quixotry"}
)

// perfEnvelope is what perfSeedEnvelope built: the ids a QA-2/3
// harness scripts its reads and searches against, since a fresh
// store's autoincrement ids are otherwise only knowable by re-querying
// what was just written.
type perfEnvelope struct {
	mailboxIDs []int64
	messageIDs []int64
}

// perfSeedEnvelope migrates a fresh store at path and fills it with
// messageCount messages spread across mailboxCount mailboxes, the
// QA-5 scale envelope (100k messages) a QA-2/3 test runs its scripted
// reads and searches against. It writes directly through db rather
// than the writer's intent path: this is bulk fixture generation, the
// same shape seedStoreNeedingRecovery (cmd/poplar) already uses at
// small scale, not a production write.
func perfSeedEnvelope(t *testing.T, path string, messageCount, mailboxCount int) perfEnvelope {
	t.Helper()

	db, err := sql.Open("sqlite", dsn(path, connReadWrite))
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'perf', 'jmap', 'perf@example.com')`); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	env := perfEnvelope{messageIDs: make([]int64, 0, messageCount)}
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
		env.mailboxIDs = append(env.mailboxIDs, mailboxID)
	}

	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: a fixed seed makes this fixture's shape reproducible across runs, not a security-sensitive use
	receivedBase := time.Now().Add(-2 * 365 * 24 * time.Hour).Unix()

	const batchSize = 2000
	for batchStart := 0; batchStart < messageCount; batchStart += batchSize {
		batchEnd := min(batchStart+batchSize, messageCount)
		if err := perfSeedBatch(db, env.mailboxIDs, rng, receivedBase, batchStart, batchEnd); err != nil {
			t.Fatalf("seed messages %d..%d: %v", batchStart, batchEnd, err)
		}
		for i := batchStart; i < batchEnd; i++ {
			env.messageIDs = append(env.messageIDs, int64(i+1))
		}
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close seed connection: %v", err)
	}
	// MarkCleanShutdown requires every connection over path closed
	// first, so the caller's next store.Open skips the integrity check
	// QA-1 measures around.
	if err := MarkCleanShutdown(path); err != nil {
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
// mailboxes, so QA-2's folder-switch operations land on mailboxes that
// actually hold mail.
func perfPickMailbox(rng *rand.Rand, mailboxIDs []int64) int64 {
	if len(mailboxIDs) == 1 || rng.IntN(5) != 0 {
		return mailboxIDs[0]
	}
	return mailboxIDs[1+rng.IntN(len(mailboxIDs)-1)]
}

// perfRandomText draws words words from the common vocabulary, salting
// in one rare word roughly one time in five hundred so the rare-term
// search class has a small, known population to match against.
func perfRandomText(rng *rand.Rand, words int) string {
	picked := make([]string, words)
	for i := range picked {
		if rng.IntN(500) == 0 {
			picked[i] = perfRareWords[rng.IntN(len(perfRareWords))]
			continue
		}
		picked[i] = perfCommonWords[rng.IntN(len(perfCommonWords))]
	}
	return strings.Join(picked, " ")
}
