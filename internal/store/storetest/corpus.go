package storetest

import (
	"bytes"
	"database/sql"
	"embed"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/store"
)

// CommonWords, MediumWords, and RareWords are the small committed
// vocabulary the perf corpus measures its search classes against, so
// no measurement needs the private mail corpus. Common words carry the
// prose itself and match nearly every message; a medium word is salted
// into roughly one message in ten and a rare word into one in two
// hundred, which is what gives the search classes a genuine spread of
// hit rates. The rates are per message, not per word: a message
// carries hundreds of words, so a per-word rate high enough to be
// visible would put every term in every document.
//
// These are the measured surface of a much larger generated
// vocabulary; perfSurfaceVocabulary explains the rest of it.
var (
	CommonWords = []string{
		"meeting", "invoice", "update", "review", "report", "schedule",
		"budget", "project", "release", "attached", "proposal", "summary",
		"deadline", "status", "feedback", "roadmap", "ticket", "customer",
	}
	MediumWords = []string{"onboarding", "vendor", "compliance", "migration"}
	RareWords   = []string{"xylograph", "umbraculum", "zenithal", "quixotry"}
)

// perfFillerWords is the connective tissue between the measured
// vocabulary above: prose built from CommonWords alone tokenizes
// nothing like mail, since every term then lands in every document.
var perfFillerWords = []string{
	"the", "and", "for", "with", "that", "this", "from", "into", "about",
	"we", "our", "your", "team", "week", "please", "before", "after",
	"since", "which", "would", "should", "here", "there", "when", "while",
	"send", "read", "check", "note", "plan", "call", "reply", "thanks",
}

// perfNames are the correspondents a generated message greets, signs
// off as, and quotes, so a body reads as mail rather than as a wall of
// prose.
var perfNames = []string{
	"Dana", "Milo", "Priya", "Sven", "Rosa", "Tomas", "Ada", "Kofi",
	"Ines", "Rafa", "Nadia", "Oskar",
}

// The perf corpus is the SQLite driver audit's section 4.1
// specification: one account, four mailboxes at an 80/20 inbox skew,
// messages spread over two years, lognormal bodies of mail-shaped
// text, search_text derived from the body, threads in small groups,
// and a calendar alongside. Building it is expensive, so the corpus is
// generated once into the user cache directory and copied per run,
// the model the audit itself names.
//
// The full 100,000-message envelope is what QA-2, QA-3, and QA-5 are
// certified against, and it costs minutes to build and around two
// gigabytes on disk. The perf step runs on every commit, so the
// default corpus is a tenth of that envelope at the same
// distributions: a regression gate, not the certification. Set
// POPLAR_PERF_FULL to seed the full envelope.
const (
	perfFullMessages        = 100_000
	perfGateMessages        = 10_000
	perfMailboxes           = 4
	perfMessagesPerEvent    = 20 // the audit's 5,000 events at the full envelope
	perfOccurrencesPerEvent = 10 // and its ~50,000 occurrences
	perfEventSeconds        = 3600
	perfBatchSize           = 1_000
	perfSpread              = 2 * 365 * 24 * time.Hour

	// perfCorpusVersion invalidates cached master corpora: a change to
	// anything the generator below writes, or to the migrated schema a
	// master is seeded against, gets a bump, or machines that already
	// hold a master keep measuring the old corpus. v3: message_fts's
	// prefix widened to '2 3 4' in 0001_initial.sql; the generator
	// itself is unchanged.
	perfCorpusVersion = 3

	perfFullEnvVar = "POPLAR_PERF_FULL"
)

// perfEpoch anchors the corpus's arrival clock. A wall-clock base
// would put a different received_at in every seeded file, and the
// committed fingerprints could then never be more than approximate.
var perfEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// Body sizes are a two-component draw, which is what the audit's own
// table describes: 99 messages in 100 are ordinary mail, lognormal
// with a 4 KB median, and the hundredth is a thread digest or an
// inlined attachment, lognormal around 350 KB. The parameters are
// solved for the table's quantiles rather than chosen: sigma 1.277
// puts the bulk's p90.9 at 22 KB, and the heavy component's median and
// sigma place the mixture's p99 at 180 KB while leaving enough spread
// that a handful of bodies per 100,000 actually reach the 2 MB cap.
//
// The mean this produces is ~13.5 KB, not the table's ~8 KB, and the
// two cannot be reconciled: a p99 of 180 KB puts 1% of the mass above
// 180 KB, which alone costs about 4.5 KB of mean, and a p90 of 22 KB
// with a 4 KB median forces a bulk mean of 9.3 KB. The quantiles and
// the cap are what the read path and QA-5 actually feel, so they win
// and the total is reported rather than trimmed.
var (
	perfBodyLogMedian  = math.Log(4096)
	perfBodySigma      = 1.277
	perfHeavyLogMedian = math.Log(350 << 10)
	perfHeavySigma     = 0.666
)

const (
	perfHeavyOdds = 100 // one body in this many is drawn from the heavy component

	// perfBodyFloor is about the shortest mail that still carries a
	// greeting and a signature, and perfBodyCap is the audit's own
	// ceiling on a single stored body. The prose window is drawn
	// perfBodyEnvelope under the cap so that the greeting, signature,
	// quote markers and HTML tags a body wraps its prose in still leave
	// the stored content inside it.
	perfBodyFloor    = 256
	perfBodyCap      = 2 << 20
	perfBodyEnvelope = 64 << 10

	// perfSlabBytes is four times the body cap, so every window fits
	// whatever the distribution draws, and small enough to build in
	// well under a second.
	perfSlabBytes = 8 << 20

	// perfSearchTextFloor and perfSearchTextCeiling are the audit's
	// extraction budget for search_text. A body shorter than the floor
	// contributes all of it, which is what a short mail indexes: at
	// this body distribution just under a quarter of messages land
	// below 200 words (23% measured at the full envelope).
	perfSearchTextFloor   = 200
	perfSearchTextCeiling = 800

	// perfSurfaceWords is how large the generated surface vocabulary
	// is. FTS5 prefix expansion is the shape most likely to blow p99
	// (audit R7), and it only means anything when a short prefix
	// expands over many terms; a corpus written from the measured
	// vocabulary alone gives every prefix exactly one term, which
	// would let R7 clear the gate without ever expanding anything.
	perfSurfaceWords = 16_000
)

// perfConsonants, perfVowels, and perfSuffixes are what a generated
// surface word is built from.
const (
	perfConsonants = "bcdfghklmnprstvwz"
	perfVowels     = "aeiou"
)

var perfSuffixes = []string{"s", "ed", "ing", "er", "ers", "ion", "ions", "al", "ly", "ment"}

//go:embed testdata/perf-corpus-fingerprint-*.txt
var perfFingerprints embed.FS

// PerfEnvelope is what SeedPerfEnvelope placed: the ids a perf harness
// scripts its reads and searches against, since a seeded store's
// primary keys are otherwise only knowable by re-querying what the
// corpus generator wrote.
type PerfEnvelope struct {
	MailboxIDs []int64
	MessageIDs []int64
}

// SeedPerfEnvelope places the perf corpus at path, copying the cached
// master this machine holds and seeding that master first if it has
// none. The copy is a full independent store: a harness that writes to
// it (QA-2's concurrent backfill) leaves the master untouched, and
// every run measures byte-identical starting state.
//
// The master's fingerprint is checked against the committed one for
// its scale on every call, so a stale cache or a generator change
// without a perfCorpusVersion bump fails the measurement instead of
// quietly moving what it measured.
func SeedPerfEnvelope(t *testing.T, path string) PerfEnvelope {
	t.Helper()

	messages := perfGateMessages
	if os.Getenv(perfFullEnvVar) != "" {
		messages = perfFullMessages
	}

	master := perfCorpusMaster(t, messages)
	perfVerifyFingerprint(t, master, messages)
	perfCopyFile(t, master, path)
	// The master was closed cleanly, but the marker is a sidecar file
	// rather than a page inside the database, so the copy needs its own.
	if err := store.MarkCleanShutdown(path); err != nil {
		t.Fatalf("mark clean shutdown: %v", err)
	}

	env := PerfEnvelope{
		MailboxIDs: make([]int64, perfMailboxes),
		MessageIDs: make([]int64, messages),
	}
	for i := range env.MailboxIDs {
		env.MailboxIDs[i] = int64(i + 1)
	}
	for i := range env.MessageIDs {
		env.MessageIDs[i] = int64(i + 1)
	}
	return env
}

// perfCorpusMaster returns the cached master corpus for messages,
// seeding it if this machine has not built it yet. Seeding runs into a
// temp file alongside the master and renames it into place, so a
// second test binary racing the same corpus either finds a finished
// file or builds its own and wins or loses the rename. Neither ever
// opens a half-seeded database, at the cost of one duplicated seed on
// a cold cache.
func perfCorpusMaster(t *testing.T, messages int) string {
	t.Helper()

	dir := perfCorpusDir(t)
	master := filepath.Join(dir, fmt.Sprintf("corpus-v%d-%d.db", perfCorpusVersion, messages))
	if _, err := os.Stat(master); err == nil {
		perfAssertColdWAL(t, master)
		return master
	}

	partial, err := os.CreateTemp(dir, "corpus-*.partial")
	if err != nil {
		t.Fatalf("create partial corpus in %s: %v", dir, err)
	}
	partialPath := partial.Name()
	if err := partial.Close(); err != nil {
		t.Fatalf("close partial corpus %s: %v", partialPath, err)
	}
	defer func() { _ = os.Remove(partialPath) }()

	start := time.Now()
	corpus := seedPerfCorpus(t, partialPath, messages)
	if err := os.Rename(partialPath, master); err != nil {
		t.Fatalf("install corpus %s: %v", master, err)
	}
	perfAssertColdWAL(t, master)
	perfWriteRecords(t, master, corpus)
	t.Logf("perf corpus: seeded %d messages into %s in %s\n%s",
		messages, master, time.Since(start).Round(time.Second), corpus.fingerprint())
	return master
}

// perfAssertColdWAL fails when the master carries a live write-ahead
// log. A copy taken while a WAL holds committed pages is an older
// database than the one whose fingerprint was recorded, and it would
// be silently older.
func perfAssertColdWAL(t *testing.T, master string) {
	t.Helper()

	info, err := os.Stat(master + "-wal")
	if err != nil || info.Size() == 0 {
		return
	}
	t.Fatalf("%s-wal holds %d bytes; the master was left open by another process and a copy of it would be stale",
		master, info.Size())
}

// perfVerifyFingerprint compares the master's own fingerprint against
// the one committed for its scale.
func perfVerifyFingerprint(t *testing.T, master string, messages int) {
	t.Helper()

	name := fmt.Sprintf("testdata/perf-corpus-fingerprint-%d.txt", messages)
	want, err := perfFingerprints.ReadFile(name)
	if err != nil {
		t.Fatalf("read committed fingerprint: %v", err)
	}
	got, err := os.ReadFile(master + ".fingerprint.txt")
	if err != nil {
		t.Fatalf("read corpus fingerprint: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("the corpus at %s does not match %s.\ngot:\n%s\nwant:\n%s\nDelete the master to reseed it, and bump perfCorpusVersion if the generator changed.",
			master, name, got, want)
	}
}

// perfCorpusDir is where master corpora live between runs.
func perfCorpusDir(t *testing.T) string {
	t.Helper()

	base, err := os.UserCacheDir()
	if err != nil {
		// The corpus then lands somewhere a reboot may clear, which
		// costs a re-seed per run rather than failing the measurement.
		base = os.TempDir()
		t.Logf("perf corpus: no user cache directory (%v); caching under %s instead", err, base)
	}
	dir := filepath.Join(base, "poplar", "perf-corpus")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}
	return dir
}

func perfCopyFile(t *testing.T, src, dst string) {
	t.Helper()

	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		t.Fatalf("copy %s to %s: %v", src, dst, err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close %s: %v", dst, err)
	}
}

// perfCorpus is what one seeding run produced: the fingerprint the
// audit asks to be committed alongside the harness, and the
// sqlite_stat1 rows ANALYZE derived from the corpus, which
// internal/store's EXPLAIN goldens are asserted against.
type perfCorpus struct {
	messages      int
	events        int
	occurrences   int
	bodyBytes     int64
	fileBytes     int64
	pageSize      int64
	pageCount     int64
	freelistCount int64
	ftsIndexBytes int64
	stat1         string
}

func (c perfCorpus) fingerprint() string {
	var b strings.Builder
	b.WriteString("# poplar perf corpus fingerprint (SQLite driver audit section 4.1).\n")
	b.WriteString("# Regenerate by seeding the corpus with POPLAR_PERF_FULL=1 and copying\n")
	b.WriteString("# the .fingerprint.txt file the seeder writes beside the cached master.\n")
	for _, field := range []struct {
		name  string
		value int64
	}{
		{"messages", int64(c.messages)},
		{"mailboxes", perfMailboxes},
		{"events", int64(c.events)},
		{"occurrences", int64(c.occurrences)},
		{"body_bytes", c.bodyBytes},
		{"file_bytes", c.fileBytes},
		{"page_size", c.pageSize},
		{"page_count", c.pageCount},
		{"freelist_count", c.freelistCount},
		{"fts_index_bytes", c.ftsIndexBytes},
	} {
		fmt.Fprintf(&b, "%s=%d\n", field.name, field.value)
	}
	// QA-5's store-size criterion is a ratio against retained body
	// bytes, so the fingerprint carries the ratio rather than leaving
	// every reader to divide.
	fmt.Fprintf(&b, "file_over_body=%.2f\n", float64(c.fileBytes)/float64(c.bodyBytes))
	return b.String()
}

// perfWriteRecords writes the fingerprint and the ANALYZE statistics
// beside the master corpus. The fingerprint is what perfVerifyFingerprint
// checks on every later run, and internal/store's EXPLAIN goldens are
// asserted against a copy of the statistics file committed under
// testdata, which is how a plan is pinned at the QA envelope without
// every run rebuilding two gigabytes of mail.
func perfWriteRecords(t *testing.T, master string, corpus perfCorpus) {
	t.Helper()

	for _, record := range []struct{ path, content string }{
		{master + ".fingerprint.txt", corpus.fingerprint()},
		{master + ".stat1.sql", corpus.stat1},
	} {
		if err := os.WriteFile(record.path, []byte(record.content), 0o600); err != nil {
			t.Fatalf("write %s: %v", record.path, err)
		}
	}
}

// seedPerfCorpus builds the corpus at path from nothing and returns
// its fingerprint. It writes through a raw connection rather than the
// writer's intent path: this is bulk fixture generation, not a
// production write.
func seedPerfCorpus(t *testing.T, path string, messages int) perfCorpus {
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
	for i := range perfMailboxes {
		role := ""
		if i == 0 {
			role = "inbox"
		}
		if _, err := db.Exec(`INSERT INTO mailbox (id, account_id, role, name) VALUES (?, 1, ?, ?)`,
			i+1, role, fmt.Sprintf("Mailbox %d", i)); err != nil {
			t.Fatalf("seed mailbox %d: %v", i+1, err)
		}
	}

	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: a fixed seed makes this corpus reproducible across runs and machines, not a security-sensitive use
	s := &perfSeeder{
		rng:          rng,
		slab:         perfProseSlab(rng),
		receivedBase: perfEpoch.Add(-perfSpread).Unix(),
		receivedStep: max(int64(perfSpread/time.Second)/int64(messages), 1),
	}
	for batchStart := 0; batchStart < messages; batchStart += perfBatchSize {
		batchEnd := min(batchStart+perfBatchSize, messages)
		if err := s.seedMessages(db, batchStart, batchEnd); err != nil {
			t.Fatalf("seed messages %d..%d: %v", batchStart, batchEnd, err)
		}
	}

	events := messages / perfMessagesPerEvent
	if err := s.seedCalendar(db, events); err != nil {
		t.Fatalf("seed calendar: %v", err)
	}

	// ANALYZE is part of the corpus, not a measurement step: every
	// query the harness runs, and every plan the goldens pin, is meant
	// to be the plan SQLite chooses knowing what the tables hold.
	if _, err := db.Exec(`ANALYZE`); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	corpus := perfCorpus{
		messages:    messages,
		events:      events,
		occurrences: events * perfOccurrencesPerEvent,
	}
	if err := perfReadStats(db, &corpus); err != nil {
		t.Fatalf("read corpus statistics: %v", err)
	}
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed connection: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	corpus.fileBytes = info.Size()
	return corpus
}

// perfSeeder carries the state one corpus generation threads through
// its batches: the shared prose, the arrival clock, and the thread
// group being filled.
type perfSeeder struct {
	rng          *rand.Rand
	slab         []byte
	receivedBase int64
	receivedStep int64
	threadIndex  int
	threadLeft   int
}

// seedMessages inserts one transaction's worth of messages
// [batchStart, batchEnd), each into exactly one mailbox weighted so
// the seeded inbox holds most of the traffic, the skew a real
// account's Inbox and Archive split has.
func (s *perfSeeder) seedMessages(db *sql.DB, batchStart, batchEnd int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	messageStmt, err := tx.Prepare(`
		INSERT INTO message (id, account_id, thread_key, received_at, subject, from_addr, flags, size, search_text)
		VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?)`)
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
		receivedAt := s.receivedBase + int64(i)*s.receivedStep
		content, searchText := s.body(id)
		flags, unread := 1, 0 // FlagSeen
		if s.rng.IntN(5) == 0 {
			flags, unread = 0, 1
		}

		if _, err := messageStmt.Exec(id, s.threadKey(), receivedAt, perfRandomText(s.rng, 4+s.rng.IntN(9)),
			perfSender(s.rng), flags, len(content), searchText); err != nil {
			return fmt.Errorf("insert message %d: %w", id, err)
		}
		if _, err := mailboxStmt.Exec(id, s.mailbox(), receivedAt, unread); err != nil {
			return fmt.Errorf("insert message_mailbox %d: %w", id, err)
		}
		if _, err := bodyStmt.Exec(id, content, receivedAt); err != nil {
			return fmt.Errorf("insert body %d: %w", id, err)
		}
	}

	return tx.Commit()
}

// seedCalendar inserts the audit's calendar alongside the mail: one
// calendar holding events, each recurring over perfOccurrencesPerEvent
// weekly occurrences, spread across the same two years.
func (s *perfSeeder) seedCalendar(db *sql.DB, events int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`INSERT INTO calendar (id, account_id, name, is_default) VALUES (1, 1, 'Perf', 1)`); err != nil {
		return fmt.Errorf("insert calendar: %w", err)
	}

	eventStmt, err := tx.Prepare(`
		INSERT INTO event (id, account_id, calendar_id, uid, summary, start_local, tzid, duration_secs, is_recurring)
		VALUES (?, 1, 1, ?, ?, ?, 'America/Anchorage', ?, 1)`)
	if err != nil {
		return fmt.Errorf("prepare event insert: %w", err)
	}
	defer func() { _ = eventStmt.Close() }()

	occurrenceStmt, err := tx.Prepare(`
		INSERT INTO occurrence (event_id, recurrence_id, start_utc, end_utc, start_local, local_date)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare occurrence insert: %w", err)
	}
	defer func() { _ = occurrenceStmt.Close() }()

	step := max(int64(perfSpread/time.Second)/int64(max(events, 1)), 1)
	for i := range events {
		id := int64(i + 1)
		start := s.receivedBase + int64(i)*step
		if _, err := eventStmt.Exec(id, fmt.Sprintf("perf-%d@example.com", id),
			perfRandomText(s.rng, 3+s.rng.IntN(4)), start, perfEventSeconds); err != nil {
			return fmt.Errorf("insert event %d: %w", id, err)
		}
		for n := range perfOccurrencesPerEvent {
			at := start + int64(n)*int64(7*24*time.Hour/time.Second)
			when := time.Unix(at, 0).UTC()
			if _, err := occurrenceStmt.Exec(id, when.Format("20060102T150405Z"), at, at+perfEventSeconds, at, when.Format(time.DateOnly)); err != nil {
				return fmt.Errorf("insert occurrence %d/%d: %w", id, n, err)
			}
		}
	}

	return tx.Commit()
}

// mailbox weights mailbox 1 (the seeded inbox) at roughly 80% of
// traffic, splitting the rest evenly across the others, so a
// folder-switch operation lands on mailboxes that actually hold mail.
func (s *perfSeeder) mailbox() int64 {
	if s.rng.IntN(5) != 0 {
		return 1
	}
	return int64(2 + s.rng.IntN(perfMailboxes-1))
}

// threadKey hands out thread keys in runs, so consecutive arrivals
// form a conversation. Run lengths are exponential and truncated at
// the audit's 30: mostly singletons and pairs, with a tail of real
// threads for queryThreadAcrossFolders to fan out over.
func (s *perfSeeder) threadKey() string {
	if s.threadLeft == 0 {
		s.threadIndex++
		s.threadLeft = min(1+int(s.rng.ExpFloat64()*2.5), 30)
	}
	s.threadLeft--
	return "thread-" + strconv.Itoa(s.threadIndex)
}

// body returns one message's stored content and the search_text
// derived from it. The body is mail-shaped: a greeting, a reference
// line carrying the identifiers that give the index a realistic term
// dictionary, prose, a signature, and for a third of messages a quoted
// reply. A quarter of messages are stored as HTML, and search_text is
// taken from the plain text either way, the same text an HTML mail's
// extraction step would index.
func (s *perfSeeder) body(id int64) (content []byte, searchText string) {
	prose := perfSlabWindow(s.rng, s.slab, perfBodySize(s.rng))

	var b strings.Builder
	b.Grow(len(prose) + 512)
	fmt.Fprintf(&b, "Hi %s,\n\n%s\n\n", perfNames[s.rng.IntN(len(perfNames))], s.reference(id))

	quoted := len(prose)
	if s.rng.IntN(3) == 0 {
		quoted = perfWordBoundary(prose, 2*len(prose)/3)
	}
	b.Write(prose[:quoted])
	fmt.Fprintf(&b, "\n\n-- \n%s\n", perfNames[s.rng.IntN(len(perfNames))])
	if quoted < len(prose) {
		fmt.Fprintf(&b, "\nOn Tuesday, %s wrote:\n", perfNames[s.rng.IntN(len(perfNames))])
		perfWriteQuoted(&b, prose[quoted:])
	}

	plain := b.String()
	content = []byte(plain)
	if s.rng.IntN(4) == 0 {
		content = perfHTML(plain)
	}
	words := perfSearchTextFloor + s.rng.IntN(perfSearchTextCeiling-perfSearchTextFloor+1)
	return content, perfFirstWords(plain, words)
}

// reference builds the line that carries a message's own identifiers,
// and salts in the medium and rare vocabulary at the per-message rates
// the search classes are calibrated to. It sits near the top of the
// body so those terms always land inside the derived search_text.
func (s *perfSeeder) reference(id int64) string {
	var b strings.Builder
	// Knuth's multiplicative hash constant, only to scatter the build
	// token across the term dictionary rather than leaving it a run of
	// consecutive numbers.
	fmt.Fprintf(&b, "Ref: PLR-%d build-%08x case-%d", id, id*2654435761&0xffffffff, id/7)
	if s.rng.IntN(10) == 0 {
		b.WriteByte(' ')
		b.WriteString(MediumWords[s.rng.IntN(len(MediumWords))])
	}
	if s.rng.IntN(200) == 0 {
		b.WriteByte(' ')
		b.WriteString(RareWords[s.rng.IntN(len(RareWords))])
	}
	return b.String()
}

func perfBodySize(rng *rand.Rand) int {
	logMedian, sigma := perfBodyLogMedian, perfBodySigma
	if rng.IntN(perfHeavyOdds) == 0 {
		logMedian, sigma = perfHeavyLogMedian, perfHeavySigma
	}
	return min(max(int(math.Exp(logMedian+sigma*rng.NormFloat64())), perfBodyFloor), perfBodyCap-perfBodyEnvelope)
}

// perfVocabulary is the word supply the prose slab draws from: the
// generated surface terms under a Zipf frequency, plus the measured
// and filler vocabularies at fixed shares.
type perfVocabulary struct {
	surface []string
	zipf    *rand.Zipf
}

// word draws one token. A sixth of the prose is the measured common
// vocabulary, which keeps every common word in nearly every document
// and so keeps QA-3's hit rates where they were; a third is filler,
// and the rest is surface vocabulary, whose Zipf frequency is what
// gives the term dictionary the long tail real mail has.
func (v perfVocabulary) word(rng *rand.Rand) string {
	switch rng.IntN(6) {
	case 0:
		return CommonWords[rng.IntN(len(CommonWords))]
	case 1, 2:
		return perfFillerWords[rng.IntN(len(perfFillerWords))]
	default:
		return v.surface[v.zipf.Uint64()]
	}
}

// perfSurfaceVocabulary coins the corpus's word-like terms. Growth
// starts from the measured vocabulary and mostly extends a prefix of a
// word already coined, the way a real lexicon shares stems: that is
// what makes "meet*" and "meeti*" expand over dozens of terms instead
// of one, which is the whole point of measuring prefix expansion.
func perfSurfaceVocabulary(rng *rand.Rand) []string {
	coined := make([]string, 0, perfSurfaceWords+len(CommonWords)+len(MediumWords))
	coined = append(coined, CommonWords...)
	coined = append(coined, MediumWords...)
	seen := make(map[string]bool, len(coined))
	for _, word := range coined {
		seen[word] = true
	}

	surface := make([]string, 0, perfSurfaceWords)
	for len(surface) < perfSurfaceWords {
		word := perfSyllable(rng) + perfSyllable(rng)
		if rng.IntN(5) != 0 {
			base := coined[rng.IntN(len(coined))]
			word = base[:max(len(base)-1-rng.IntN(3), 2)] + perfSyllable(rng)
		}
		if rng.IntN(3) == 0 {
			word += perfSuffixes[rng.IntN(len(perfSuffixes))]
		}
		if len(word) < 3 || seen[word] {
			continue
		}
		seen[word] = true
		coined = append(coined, word)
		surface = append(surface, word)
	}
	return surface
}

func perfSyllable(rng *rand.Rand) string {
	syllable := string(perfConsonants[rng.IntN(len(perfConsonants))]) + string(perfVowels[rng.IntN(len(perfVowels))])
	if rng.IntN(3) == 0 {
		syllable += string(perfConsonants[rng.IntN(len(perfConsonants))])
	}
	return syllable
}

// perfProseSlab builds the block of mail-shaped prose every body
// slices its paragraphs out of. Bodies are windows into one slab
// rather than freshly composed text: the full envelope holds well over
// a gigabyte of body, and composing that word by word costs more than
// the SQLite writes it feeds.
func perfProseSlab(rng *rand.Rand) []byte {
	vocab := perfVocabulary{
		surface: perfSurfaceVocabulary(rng),
		zipf:    rand.NewZipf(rng, 1.1, 1, perfSurfaceWords-1),
	}

	var b bytes.Buffer
	b.Grow(perfSlabBytes + 4096)
	for b.Len() < perfSlabBytes {
		perfWriteSentence(&b, rng, vocab)
		if rng.IntN(7) == 0 {
			b.WriteString("\n\n")
		} else {
			b.WriteByte(' ')
		}
	}
	return b.Bytes()
}

func perfWriteSentence(b *bytes.Buffer, rng *rand.Rand, vocab perfVocabulary) {
	words := 6 + rng.IntN(13)
	for i := range words {
		word := vocab.word(rng)
		if i == 0 {
			b.WriteString(strings.ToUpper(word[:1]))
			b.WriteString(word[1:])
			continue
		}
		b.WriteByte(' ')
		b.WriteString(word)
		if i == words/2 && rng.IntN(4) == 0 {
			b.WriteByte(',')
		}
	}
	b.WriteByte('.')
}

// perfSlabWindow returns roughly size bytes of slab from a random
// offset, both ends moved forward to a word boundary so the window
// reads as prose. The body cap is a quarter of the slab, so a window
// always fits.
func perfSlabWindow(rng *rand.Rand, slab []byte, size int) []byte {
	start := perfWordBoundary(slab, rng.IntN(len(slab)-size))
	return slab[start:perfWordBoundary(slab, min(start+size, len(slab)))]
}

// perfWordBoundary returns the offset at or after at where prose
// breaks between words.
func perfWordBoundary(prose []byte, at int) int {
	for at < len(prose) && !perfIsSpace(prose[at]) {
		at++
	}
	return at
}

func perfIsSpace(c byte) bool { return c == ' ' || c == '\n' }

// perfWriteQuoted writes prose as a quoted reply, marking whole
// paragraphs rather than rewrapping every line: the marker is what
// makes the text quote-shaped, and rewrapping a gigabyte of body would
// cost more than it buys.
func perfWriteQuoted(b *strings.Builder, prose []byte) {
	for para := range bytes.SplitSeq(prose, []byte("\n\n")) {
		b.WriteString("> ")
		b.Write(para)
		b.WriteString("\n>\n")
	}
}

// perfHTML renders a plain-text body as the quoted-reply HTML a real
// mail store holds for a good fraction of its messages.
func perfHTML(plain string) []byte {
	var b bytes.Buffer
	b.Grow(len(plain) + len(plain)/8 + 64)
	b.WriteString("<html><body>\n")
	for block := range strings.SplitSeq(plain, "\n\n") {
		if strings.HasPrefix(block, "> ") {
			b.WriteString("<blockquote>")
			b.WriteString(strings.ReplaceAll(block, "> ", ""))
			b.WriteString("</blockquote>\n")
			continue
		}
		b.WriteString("<p>")
		b.WriteString(block)
		b.WriteString("</p>\n")
	}
	b.WriteString("</body></html>\n")
	return b.Bytes()
}

// perfFirstWords returns the first n words of s.
func perfFirstWords(s string, n int) string {
	seen := 0
	for i := range len(s) {
		if !perfIsSpace(s[i]) {
			continue
		}
		seen++
		if seen == n {
			return s[:i]
		}
	}
	return s
}

func perfSender(rng *rand.Rand) string {
	return strings.ToLower(perfNames[rng.IntN(len(perfNames))]) + "@example.com"
}

// perfRandomText draws words words from CommonWords, salting in a
// medium word roughly one time in fifty and a rare word roughly one
// time in five hundred. It writes the short fields, subjects and event
// summaries, where a per-word rate still leaves most of the field
// common (a body's rates are per message; see the vocabulary above).
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

// perfReadStats fills in the parts of the fingerprint that only the
// seeded database knows, and dumps the ANALYZE statistics as the SQL
// that reproduces them on an empty schema.
func perfReadStats(db *sql.DB, corpus *perfCorpus) error {
	for _, stat := range []struct {
		query string
		dest  *int64
	}{
		{`PRAGMA page_size`, &corpus.pageSize},
		{`PRAGMA page_count`, &corpus.pageCount},
		{`PRAGMA freelist_count`, &corpus.freelistCount},
		{`SELECT COALESCE(SUM(LENGTH(content)), 0) FROM body`, &corpus.bodyBytes},
		// dbstat is not compiled into the store's driver, so the index
		// size comes from summing the compressed b-tree segment blobs
		// FTS5 itself stores in message_fts_data (the shadow table
		// every write and merge lands in) rather than from dbstat's
		// page-aligned accounting. It runs smaller than a page-based
		// sum would, since a block's stored length is its compressed
		// size rather than a whole page, but it is the same query
		// every run, which is what the fingerprint needs.
		{`SELECT COALESCE(SUM(LENGTH(block)), 0) FROM message_fts_data`, &corpus.ftsIndexBytes},
	} {
		if err := db.QueryRow(stat.query).Scan(stat.dest); err != nil {
			return fmt.Errorf("%s: %w", stat.query, err)
		}
	}

	rows, err := db.Query(`SELECT tbl, idx, stat FROM sqlite_stat1 ORDER BY tbl, COALESCE(idx, '')`)
	if err != nil {
		return fmt.Errorf("read sqlite_stat1: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var b strings.Builder
	fmt.Fprintf(&b, "-- ANALYZE statistics from poplar's %d-message perf corpus.\n", corpus.messages)
	b.WriteString("-- Regenerate by seeding the corpus with POPLAR_PERF_FULL=1 and copying\n")
	b.WriteString("-- the .stat1.sql file the seeder writes beside the cached master.\n")
	b.WriteString("ANALYZE;\nDELETE FROM sqlite_stat1;\n")
	for rows.Next() {
		var tbl, stat string
		var idx sql.NullString
		if err := rows.Scan(&tbl, &idx, &stat); err != nil {
			return fmt.Errorf("scan sqlite_stat1 row: %w", err)
		}
		index := "NULL"
		if idx.Valid {
			index = "'" + idx.String + "'"
		}
		fmt.Fprintf(&b, "INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('%s', %s, '%s');\n", tbl, index, stat)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite_stat1: %w", err)
	}
	// Statistics loaded by hand are not consulted until something makes
	// SQLite reload them; analyzing the schema table is that no-op.
	b.WriteString("ANALYZE sqlite_schema;\n")
	corpus.stat1 = b.String()
	return nil
}
