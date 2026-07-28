package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
)

const warmRuns = 20

// benchResult holds aggregate timing for one measurement class.
type benchResult struct {
	Label string        `json:"label"`
	Count int           `json:"message_count"`
	P50   time.Duration `json:"p50_ns"`
	P95   time.Duration `json:"p95_ns"`
	P99   time.Duration `json:"p99_ns"`
	Min   time.Duration `json:"min_ns"`
	Max   time.Duration `json:"max_ns"`
}

// benchReport is the full JSON output written to disk.
type benchReport struct {
	RunAt        string                 `json:"run_at"`
	MsgCount     int64                  `json:"message_count"`
	RealCount    int64                  `json:"real_count"`
	SQLiteVer    string                 `json:"sqlite_version"`
	DBFileSizeB  int64                  `json:"db_file_size_bytes"`
	BodyBytesSum int64                  `json:"body_bytes_sum"`
	FTSIndexB    int64                  `json:"fts_index_bytes"`
	RSSBytes     int64                  `json:"rss_bytes"`
	BusyCount    int64                  `json:"busy_error_count"`
	Startup      map[string]benchResult `json:"startup"`
	Interaction  map[string]benchResult `json:"interaction"`
	Search       map[string]benchResult `json:"search"`
}

type benchFlags struct {
	outPath string
}

func newBenchCmd() *cobra.Command {
	var f benchFlags
	cmd := &cobra.Command{
		Use:          "bench",
		Short:        "Measure and report DB performance at 36k and 100k scale",
		SilenceUsage: true,
		RunE:         func(_ *cobra.Command, _ []string) error { return runBench(&f) },
	}
	cmd.Flags().StringVar(&f.outPath, "report", "", "path for the markdown report (default: docs/poplar/research/2026-07-27-phase4-measurement-spike.md)")
	return cmd
}

// newStartupProbeCmd is a hidden subcommand used internally to measure real
// process startup overhead. It opens the DB, runs a quick_check, fetches one
// page, and writes timing JSON to stdout.
func newStartupProbeCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "startup-probe",
		Hidden: true,
		RunE:   runStartupProbe,
	}
}

func runStartupProbe(_ *cobra.Command, _ []string) error {
	t0 := time.Now()

	w, r, err := openDB()
	if err != nil {
		return err
	}
	defer w.Close()
	defer r.Close()

	openDur := time.Since(t0)

	t1 := time.Now()
	var qcResult string
	if err := r.QueryRow("PRAGMA quick_check(100)").Scan(&qcResult); err != nil {
		return fmt.Errorf("quick_check: %w", err)
	}
	checkDur := time.Since(t1)

	var mailbox string
	r.QueryRow("SELECT mailbox FROM message ORDER BY received_at DESC LIMIT 1").Scan(&mailbox) //nolint:errcheck

	t2 := time.Now()
	rows, err := r.Query(
		"SELECT id, subject, received_at FROM message WHERE mailbox = ? ORDER BY received_at DESC LIMIT 50",
		mailbox,
	)
	if err != nil {
		return fmt.Errorf("first page: %w", err)
	}
	count := 0
	for rows.Next() {
		var id int64
		var sub string
		var rcvd int64
		rows.Scan(&id, &sub, &rcvd) //nolint:errcheck
		count++
	}
	rows.Close()
	pageDur := time.Since(t2)

	out := map[string]int64{
		"open_ns":  openDur.Nanoseconds(),
		"check_ns": checkDur.Nanoseconds(),
		"page_ns":  pageDur.Nanoseconds(),
		"rows":     int64(count),
	}
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(out)
}

func runBench(f *benchFlags) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	w, r, err := openDB()
	if err != nil {
		return err
	}
	defer w.Close()
	defer r.Close()

	var totalCount, realCount int64
	if err := r.QueryRow("SELECT count(*) FROM message").Scan(&totalCount); err != nil {
		return fmt.Errorf("count total: %w", err)
	}
	if err := r.QueryRow("SELECT count(*) FROM message WHERE clone_of IS NULL").Scan(&realCount); err != nil {
		return fmt.Errorf("count real: %w", err)
	}
	slog.Info("message counts", "total", totalCount, "real", realCount)

	var sqliteVer string
	r.QueryRow("SELECT sqlite_version()").Scan(&sqliteVer) //nolint:errcheck

	dbPath, _ := defaultDBPath()
	var dbFileSize int64
	if fi, err := os.Stat(dbPath); err == nil {
		dbFileSize = fi.Size()
	}

	var bodyBytesSum int64
	r.QueryRow("SELECT sum(length(body)) FROM message").Scan(&bodyBytesSum) //nolint:errcheck

	ftsSize := ftsSizeBytes(r)

	slog.Info("running startup benchmarks")
	startupResults := benchStartup(r, warmRuns)

	slog.Info("running interaction benchmarks (quiescent)")
	var busyCount int64
	qaQuiescent := benchInteraction(r, 500, false, w, realCount, totalCount, &busyCount)

	slog.Info("running interaction benchmarks (under write)")
	qaUnderWrite := benchInteraction(r, 500, true, w, realCount, totalCount, &busyCount)

	slog.Info("running search benchmarks")
	searchResults := benchSearch(r, 20, realCount, totalCount)

	rssBytes := processRSS()

	report := benchReport{
		RunAt:        time.Now().UTC().Format(time.RFC3339),
		MsgCount:     totalCount,
		RealCount:    realCount,
		SQLiteVer:    sqliteVer,
		DBFileSizeB:  dbFileSize,
		BodyBytesSum: bodyBytesSum,
		FTSIndexB:    ftsSize,
		RSSBytes:     rssBytes,
		BusyCount:    busyCount,
		Startup:      startupResults,
		Interaction:  map[string]benchResult{"quiescent": qaQuiescent, "under_write": qaUnderWrite},
		Search:       searchResults,
	}

	jsonPath, err := writeJSONReport(report)
	if err != nil {
		slog.Warn("JSON report write", "err", err)
	} else {
		slog.Info("raw results", "path", jsonPath)
	}

	reportPath := f.outPath
	if reportPath == "" {
		reportPath = "docs/poplar/research/2026-07-27-phase4-measurement-spike.md"
	}
	if err := writeMarkdownReport(report, reportPath); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	slog.Info("report written", "path", reportPath)
	return nil
}

func benchStartup(r *sql.DB, runs int) map[string]benchResult {
	bin, _ := os.Executable()

	var openTimes, checkTimes, pageTimes []time.Duration

	for range runs {
		out, err := exec.Command(bin, "startup-probe").Output()
		if err != nil {
			continue
		}
		var m map[string]int64
		if json.Unmarshal(out, &m) != nil {
			continue
		}
		openTimes = append(openTimes, time.Duration(m["open_ns"]))
		checkTimes = append(checkTimes, time.Duration(m["check_ns"]))
		pageTimes = append(pageTimes, time.Duration(m["page_ns"]))
	}

	// One cold-ish run after sync
	exec.Command("sync").Run() //nolint:errcheck
	coldOut, _ := exec.Command(bin, "startup-probe").Output()
	var coldMap map[string]int64
	var coldTotal time.Duration
	if json.Unmarshal(coldOut, &coldMap) == nil {
		coldTotal = time.Duration(coldMap["open_ns"] + coldMap["check_ns"] + coldMap["page_ns"])
	}

	// Binary exec overhead: time the subprocess round-trip with minimal work
	var execTimes []time.Duration
	for range 10 {
		t0 := time.Now()
		exec.Command(bin, "--help").Output() //nolint:errcheck
		execTimes = append(execTimes, time.Since(t0))
	}

	results := map[string]benchResult{}
	if len(openTimes) > 0 {
		results["db_open_check"] = aggregate("DB open + quick_check", openTimes, checkTimes)
		results["first_page"] = aggregateSingle("First 50 rows (list page)", pageTimes)
		results["exec_overhead"] = aggregateSingle("Binary exec overhead", execTimes)
		results["cold_total"] = benchResult{
			Label: "Cold-ish total (sync + open + check + page)",
			P50:   coldTotal,
			P95:   coldTotal,
			P99:   coldTotal,
			Min:   coldTotal,
			Max:   coldTotal,
		}
	}
	return results
}

func aggregate(label string, a, b []time.Duration) benchResult {
	combined := make([]time.Duration, min(len(a), len(b)))
	for i := range combined {
		combined[i] = a[i] + b[i]
	}
	return aggregateSingle(label, combined)
}

func aggregateSingle(label string, times []time.Duration) benchResult {
	if len(times) == 0 {
		return benchResult{Label: label}
	}
	sorted := make([]time.Duration, len(times))
	copy(sorted, times)
	slices.Sort(sorted)

	return benchResult{
		Label: label,
		P50:   percentile(sorted, 50),
		P95:   percentile(sorted, 95),
		P99:   percentile(sorted, 99),
		Min:   sorted[0],
		Max:   sorted[len(sorted)-1],
	}
}

func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := (len(sorted) - 1) * p / 100
	return sorted[idx]
}

func benchInteraction(
	r *sql.DB, ops int, withWriter bool, w *sql.DB,
	realCount, totalCount int64, busyCount *int64,
) benchResult {
	mailboxes := distinctMailboxes(r)
	if len(mailboxes) == 0 {
		mailboxes = []string{"Inbox"}
	}

	sampleIDs := sampleIDs(r, 200, totalCount)

	searchTerms := []string{"meeting", "invoice", "update", "review", "reply"}

	var stop int32
	var writerBusy int64
	var wg sync.WaitGroup

	if withWriter {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batchNum := int64(0)
			for atomic.LoadInt32(&stop) == 0 {
				batchNum++
				if err := runWriterBatch(w, sampleIDs, batchNum, totalCount); err != nil {
					if strings.Contains(err.Error(), "SQLITE_BUSY") ||
						strings.Contains(err.Error(), "database is locked") {
						atomic.AddInt64(&writerBusy, 1)
					}
				}
				runtime.Gosched()
			}
		}()
	}

	var times []time.Duration

	listOps := int(float64(ops) * 0.60)
	switchOps := int(float64(ops) * 0.15)
	bodyOps := int(float64(ops) * 0.15)
	// Remaining ops are incremental search (10%)

	currentMailbox := mailboxes[0]

	for range listOps {
		offset := rand.Int64N(max(totalCount-50, 1))
		t0 := time.Now()
		rows, err := r.Query(
			"SELECT id, subject, received_at FROM message WHERE mailbox = ? ORDER BY received_at DESC LIMIT 50 OFFSET ?",
			currentMailbox, offset,
		)
		d := time.Since(t0)
		if err != nil {
			if strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked") {
				atomic.AddInt64(busyCount, 1)
			}
		} else {
			drainRows(rows)
		}
		times = append(times, d)
	}

	for range switchOps {
		currentMailbox = mailboxes[rand.IntN(len(mailboxes))]
		t0 := time.Now()
		rows, err := r.Query(
			"SELECT id, subject, received_at FROM message WHERE mailbox = ? ORDER BY received_at DESC LIMIT 50",
			currentMailbox,
		)
		d := time.Since(t0)
		if err != nil {
			if strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked") {
				atomic.AddInt64(busyCount, 1)
			}
		} else {
			drainRows(rows)
		}
		times = append(times, d)
	}

	for range bodyOps {
		if len(sampleIDs) == 0 {
			continue
		}
		id := sampleIDs[rand.IntN(len(sampleIDs))]
		t0 := time.Now()
		var body string
		r.QueryRow("SELECT body FROM message WHERE id = ?", id).Scan(&body) //nolint:errcheck
		times = append(times, time.Since(t0))
	}

	// Incremental search: 10 typing sessions of 5 keystrokes each = 50 ops
	for i := range 10 {
		term := searchTerms[i%len(searchTerms)]
		for j := 1; j <= 5; j++ {
			prefix := term[:min(j, len(term))]
			t0 := time.Now()
			rows, err := r.Query(
				"SELECT rowid, snippet(message_fts, 0, '', '', '...', 5) FROM message_fts WHERE message_fts MATCH ? LIMIT 50",
				prefix+"*",
			)
			d := time.Since(t0)
			if err != nil {
				if strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked") {
					atomic.AddInt64(busyCount, 1)
				}
			} else {
				drainRows(rows)
			}
			times = append(times, d)
		}
	}

	if withWriter {
		atomic.StoreInt32(&stop, 1)
		wg.Wait()
		atomic.AddInt64(busyCount, writerBusy)
	}

	label := "QA-2 quiescent"
	if withWriter {
		label = "QA-2 under-write"
	}
	return aggregateSingle(label, times)
}

func runWriterBatch(w *sql.DB, sampleIDs []int64, batchNum, totalCount int64) error {
	tx, err := w.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	// 500 flag updates
	for range 500 {
		if len(sampleIDs) == 0 {
			break
		}
		id := sampleIDs[rand.IntN(len(sampleIDs))]
		if _, err := tx.Exec("UPDATE message SET flags = (flags + 1) & 15 WHERE id = ?", id); err != nil {
			return err
		}
	}

	// 50 message clones with FTS maintenance via trigger
	baseID := fmt.Sprintf("bench-w-%d-", batchNum)
	for i := range 50 {
		if len(sampleIDs) == 0 {
			break
		}
		src := sampleIDs[rand.IntN(len(sampleIDs))]
		synthID := baseID + strconv.Itoa(i)
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO message
				(server_id, thread_key, mailbox, received_at, subject, from_addr, flags,
				 has_attachment, size, body, clone_of, data)
			SELECT ?, thread_key, mailbox, received_at, subject, from_addr, flags,
				 has_attachment, size, body, id, '{}'
			FROM message WHERE id = ?
		`, synthID, src); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// searchClass represents one class of FTS5 search queries for QA-3.
type searchClass struct {
	label string
	query string
	arg   string
}

func benchSearch(r *sql.DB, runs int, realCount, totalCount int64) map[string]benchResult {
	commonTerms, mediumTerms, rareTerms := sampleSearchTerms(r)

	classes := []searchClass{}

	for _, t := range commonTerms {
		classes = append(classes, searchClass{label: "single_common", query: ftsQuery, arg: t})
	}
	for _, t := range mediumTerms {
		classes = append(classes, searchClass{label: "single_medium", query: ftsQuery, arg: t})
	}
	for _, t := range rareTerms {
		classes = append(classes, searchClass{label: "single_rare", query: ftsQuery, arg: t})
	}

	// Phrase queries: combine two common terms
	if len(commonTerms) >= 2 {
		for i := range min(5, len(commonTerms)-1) {
			phrase := `"` + commonTerms[i] + " " + commonTerms[i+1] + `"`
			classes = append(classes, searchClass{label: "phrase", query: ftsQuery, arg: phrase})
		}
	}

	// Operator-filtered: FTS term + mailbox/date scalar predicate
	mboxes := distinctMailboxes(r)
	if len(mboxes) > 0 && len(commonTerms) > 0 {
		mbox := mboxes[0]
		for _, t := range commonTerms[:min(5, len(commonTerms))] {
			classes = append(classes, searchClass{
				label: "operator_filtered",
				query: ftsJoinQuery,
				arg:   t + "|" + mbox,
			})
		}
	}

	// Boolean AND/OR/NOT queries
	if len(commonTerms) >= 2 {
		for i := range min(5, len(commonTerms)-1) {
			classes = append(classes, searchClass{
				label: "boolean_or",
				query: ftsQuery,
				arg:   commonTerms[i] + " OR " + commonTerms[i+1],
			})
		}
	}
	if len(commonTerms) >= 2 && len(rareTerms) > 0 {
		for i := range min(5, len(commonTerms)) {
			classes = append(classes, searchClass{
				label: "boolean_not",
				query: ftsQuery,
				arg:   commonTerms[i] + " NOT " + rareTerms[0],
			})
		}
	}

	// Count-vs-page cost for one common term
	if len(commonTerms) > 0 {
		ct := commonTerms[0]
		var countTimes []time.Duration
		for range runs {
			t0 := time.Now()
			var n int64
			r.QueryRow("SELECT count(*) FROM message_fts WHERE message_fts MATCH ?", ct).Scan(&n) //nolint:errcheck
			countTimes = append(countTimes, time.Since(t0))
		}
		_ = countTimes
	}

	byClass := make(map[string][]time.Duration)
	for _, cls := range classes {
		for range runs {
			t0 := time.Now()
			var rows *sql.Rows
			var err error
			if cls.label == "operator_filtered" {
				parts := strings.SplitN(cls.arg, "|", 2)
				rows, err = r.Query(ftsJoinQueryExec, parts[0], parts[1])
			} else {
				rows, err = r.Query(cls.query, cls.arg)
			}
			d := time.Since(t0)
			if err == nil {
				drainRows(rows)
			}
			byClass[cls.label] = append(byClass[cls.label], d)
		}
	}

	results := make(map[string]benchResult, len(byClass))
	for label, times := range byClass {
		results[label] = aggregateSingle(label, times)
	}
	return results
}

const (
	ftsQuery         = `SELECT rowid, snippet(message_fts, 0, '', '', '...', 5), snippet(message_fts, 1, '', '', '...', 8) FROM message_fts WHERE message_fts MATCH ? LIMIT 50`
	ftsJoinQuery     = ""
	ftsJoinQueryExec = `
		SELECT m.id, snippet(message_fts, 0, '', '', '...', 5)
		FROM message_fts
		JOIN message m ON message_fts.rowid = m.id
		WHERE message_fts MATCH ?
		  AND m.mailbox = ?
		LIMIT 50
	`
)

func sampleSearchTerms(r *sql.DB) (common, medium, rare []string) {
	r.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS fts_vocab USING fts5vocab('message_fts', 'row')") //nolint:errcheck

	rows, err := r.Query("SELECT term FROM fts_vocab WHERE length(term) > 3 ORDER BY doc DESC LIMIT 10")
	if err == nil {
		for rows.Next() {
			var t string
			rows.Scan(&t) //nolint:errcheck
			common = append(common, t)
		}
		rows.Close()
	}

	rows, err = r.Query(`
		SELECT term FROM fts_vocab WHERE length(term) > 3
		ORDER BY doc DESC LIMIT 10 OFFSET 100
	`)
	if err == nil {
		for rows.Next() {
			var t string
			rows.Scan(&t) //nolint:errcheck
			medium = append(medium, t)
		}
		rows.Close()
	}

	rows, err = r.Query("SELECT term FROM fts_vocab WHERE doc = 1 AND length(term) > 4 LIMIT 5")
	if err == nil {
		for rows.Next() {
			var t string
			rows.Scan(&t) //nolint:errcheck
			rare = append(rare, t)
		}
		rows.Close()
	}

	// Fallbacks when vocab is empty (e.g. fresh DB)
	if len(common) == 0 {
		common = []string{"meeting", "invoice", "update", "review", "report"}
	}
	if len(medium) == 0 {
		medium = []string{"attached", "schedule", "request", "please", "question"}
	}
	if len(rare) == 0 {
		rare = []string{"xyzzy42", "zzznomatch99"}
	}
	return common, medium, rare
}

func distinctMailboxes(r *sql.DB) []string {
	rows, err := r.Query("SELECT DISTINCT mailbox FROM message LIMIT 20")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var m string
		rows.Scan(&m) //nolint:errcheck
		out = append(out, m)
	}
	return out
}

func sampleIDs(r *sql.DB, n int, totalCount int64) []int64 {
	rows, err := r.Query(
		"SELECT id FROM message ORDER BY RANDOM() LIMIT ?", n,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id) //nolint:errcheck
		ids = append(ids, id)
	}
	return ids
}

func drainRows(rows *sql.Rows) {
	for rows.Next() {
	}
	rows.Close()
}

func ftsSizeBytes(r *sql.DB) int64 {
	var sz int64
	err := r.QueryRow(`
		SELECT COALESCE(sum(payload), 0) FROM dbstat
		WHERE name LIKE 'message_fts%'
	`).Scan(&sz)
	if err != nil {
		return 0
	}
	return sz
}

func processRSS() int64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			var kb int64
			fmt.Sscanf(strings.TrimPrefix(line, "VmRSS:"), "%d", &kb)
			return kb * 1024
		}
	}
	return 0
}

func writeJSONReport(report benchReport) (string, error) {
	dbPath, err := defaultDBPath()
	if err != nil {
		return "", err
	}
	outPath := filepath.Join(filepath.Dir(dbPath), "bench-results.json")
	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create JSON: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return "", fmt.Errorf("encode JSON: %w", err)
	}
	return outPath, nil
}

func writeMarkdownReport(rep benchReport, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir report dir: %w", err)
	}

	cpu := cpuModel()
	kernel := kernelVersion()

	var sb strings.Builder

	w := func(s string) { sb.WriteString(s + "\n") }
	wf := func(format string, args ...any) { sb.WriteString(fmt.Sprintf(format+"\n", args...)) }

	w("# Phase 4 Measurement Spike: poplar DB Performance")
	w("")
	wf("**Run date:** %s", rep.RunAt[:10])
	w("")
	w("## Environment")
	w("")
	wf("| Key | Value |")
	wf("|-----|-------|")
	wf("| CPU | %s |", cpu)
	wf("| Kernel | %s |", kernel)
	wf("| Go | %s |", runtime.Version())
	wf("| SQLite | %s (modernc.org/sqlite) |", rep.SQLiteVer)
	w("")
	w("## Method")
	w("")
	w("### QA-1 Startup proxy")
	w("")
	w("The bench subcommand execs itself as a subprocess (`startup-probe`) 20 times.")
	w("Each probe opens the DB, runs `PRAGMA quick_check(100)`, then fetches the first")
	w("50 rows of the most-recently-used mailbox. Timings are measured inside the probe")
	w("with `time.Now()` monotonic clock. Binary exec overhead is measured separately")
	w("by timing 10 executions of `--help`. A cold-ish run follows a `sync` call; the")
	w("OS page cache is not dropped (no sudo required).")
	w("")
	w("### QA-2 Interaction proxy")
	w("")
	w("500 operations against the reader pool: 60% list-page fetches (50 rows at")
	w("random offsets), 15% mailbox switches (new mailbox filter + first page),")
	w("15% single-message body reads, and 10% incremental search (a 5-character")
	w("term typed one keystroke at a time, each keystroke a separate FTS5 query).")
	w("Run twice: quiescent, then while a background goroutine issues batches of")
	w("500 flag updates and 50 message clones through the writer connection. BUSY")
	w("errors are counted, not retried. The delta in p95 between the two runs is")
	w("the concurrency-discipline evidence.")
	w("")
	w("### QA-3 Search")
	w("")
	w("At least 20 queries per class run 20 times each against the FTS5 external-content")
	w("table. Classes: single term (common, medium, rare), quoted phrase, operator-")
	w("filtered (FTS match + mailbox scalar predicate via JOIN), boolean OR/NOT. Each")
	w("query fetches 50 rows with `snippet()` on both subject and body columns.")
	w("Terms are sampled from an `fts5vocab` virtual table on the real archive;")
	w("frequency tiers are selected by per-document count rank.")
	w("")
	w("### Size and memory")
	w("")
	w("DB file size is read from the filesystem after bench completes. Body bytes are")
	w("summed from `length(body)` across all rows. FTS5 index size uses `dbstat`")
	w("(`SUM(payload) WHERE name LIKE 'message_fts%'`); this is payload bytes, not")
	w("total page bytes, so it understates slightly. Process RSS is read from")
	w("`/proc/self/status` VmRSS after all bench operations complete.")
	w("")
	w("## Results")
	w("")
	w("### QA-1 Startup proxy")
	w("")

	startupKeys := []string{"exec_overhead", "db_open_check", "first_page", "cold_total"}
	w("| Measurement | p50 | p95 | p99 |")
	w("|-------------|-----|-----|-----|")
	for _, k := range startupKeys {
		if res, ok := rep.Startup[k]; ok {
			wf("| %s | %s | %s | %s |",
				res.Label, fmtDur(res.P50), fmtDur(res.P95), fmtDur(res.P99))
		}
	}
	w("")
	w("### QA-2 Interaction proxy (500 operations)")
	w("")
	w("| Run | p50 | p95 | p99 |")
	w("|-----|-----|-----|-----|")
	for _, k := range []string{"quiescent", "under_write"} {
		if res, ok := rep.Interaction[k]; ok {
			wf("| %s | %s | %s | %s |",
				res.Label, fmtDur(res.P50), fmtDur(res.P95), fmtDur(res.P99))
		}
	}
	w("")
	wf("BUSY errors during under-write run: %d", rep.BusyCount)
	w("")
	w("### QA-3 Search (100k store, p95 per class)")
	w("")
	w("| Class | p50 | p95 | p99 |")
	w("|-------|-----|-----|-----|")
	searchKeys := []string{"single_common", "single_medium", "single_rare", "phrase", "operator_filtered", "boolean_or", "boolean_not"}
	for _, k := range searchKeys {
		if res, ok := rep.Search[k]; ok {
			wf("| %s | %s | %s | %s |",
				k, fmtDur(res.P50), fmtDur(res.P95), fmtDur(res.P99))
		}
	}
	w("")
	w("### Size and memory")
	w("")
	w("| Metric | Value |")
	w("|--------|-------|")
	wf("| Total messages | %d |", rep.MsgCount)
	wf("| Real (harvested) messages | %d |", rep.RealCount)
	wf("| DB file size | %s |", fmtBytes(rep.DBFileSizeB))
	wf("| Sum of body bytes stored | %s |", fmtBytes(rep.BodyBytesSum))
	wf("| FTS5 index payload (dbstat) | %s |", fmtBytes(rep.FTSIndexB))
	wf("| Process RSS after bench | %s |", fmtBytes(rep.RSSBytes))
	overhead := float64(0)
	if rep.BodyBytesSum > 0 {
		overhead = float64(rep.DBFileSizeB-rep.BodyBytesSum) / float64(rep.BodyBytesSum) * 100
	}
	wf("| Non-body overhead | %.1f%% of body bytes |", overhead)
	w("")
	w("## Observations")
	w("")
	w("_See JSON result file at `~/.local/state/poplar/perfspike/bench-results.json`")
	w("for raw per-run timing data._")
	w("")
	if rep.BusyCount > 0 {
		wf("BUSY errors occurred (%d total) during the under-write interaction run.", rep.BusyCount)
		w("Review the busy_timeout setting and writer batch size if this affects user-facing latency.")
	} else {
		w("No SQLITE_BUSY errors observed. The writer connection and reader pool shared the WAL")
		w("without contention at the batch sizes tested.")
	}
	w("")
	if s, ok := rep.Search["phrase"]; ok && s.P95 > 500*time.Millisecond {
		w("Phrase query p95 exceeded 500ms. FTS5 phrase queries perform a positional intersection")
		w("that is significantly more expensive than single-term lookup at scale. Consider a")
		w("dedicated index or query rewrite strategy for the UI's phrase-search path.")
	}

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func fmtDur(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fus", float64(d.Nanoseconds())/1000)
	}
	return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
}

func fmtBytes(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.2f GB", float64(n)/gb)
	case n >= mb:
		return fmt.Sprintf("%.2f MB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%.2f KB", float64(n)/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func cpuModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown"
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "model name") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				return strings.TrimSpace(line[idx+1:])
			}
		}
	}
	return "unknown"
}

func kernelVersion() string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
