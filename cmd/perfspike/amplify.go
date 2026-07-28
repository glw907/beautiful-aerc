package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	targetMessages = 100_000
	targetEvents   = 5_000
)

var (
	eventTitles = []string{
		"Team standup", "Budget review", "Project sync", "Quarterly planning",
		"1:1 with manager", "Design review", "Release planning", "Sprint retrospective",
		"Customer call", "Board update", "Roadmap review", "All hands",
	}

	eventLocations = []string{
		"Conference Room A", "Conference Room B", "Main Office",
		"Remote / Video call", "Building 2 Floor 3", "Cafeteria",
		"Executive Suite", "Training Room",
	}

	descWords = []string{
		"agenda", "review", "action items", "follow-up", "slides",
		"preparation", "notes", "decisions", "participants", "recording",
	}
)

type amplifyFlags struct{}

func newAmplifyCmd() *cobra.Command {
	var f amplifyFlags
	cmd := &cobra.Command{
		Use:          "amplify",
		Short:        "Grow the store to the 100k-message, 5k-event envelope",
		SilenceUsage: true,
		RunE:         func(_ *cobra.Command, _ []string) error { return runAmplify(&f) },
	}
	return cmd
}

func runAmplify(_ *amplifyFlags) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	w, r, err := openDB()
	if err != nil {
		return err
	}
	defer w.Close()
	defer r.Close()

	var currentCount int64
	if err := r.QueryRow("SELECT count(*) FROM message").Scan(&currentCount); err != nil {
		return fmt.Errorf("count messages: %w", err)
	}
	slog.Info("current store", "messages", currentCount)

	if currentCount == 0 {
		return fmt.Errorf("no real messages found; run harvest first")
	}

	if currentCount >= targetMessages {
		slog.Info("messages already at target", "count", currentCount)
	} else {
		if err := cloneToTarget(w, r, currentCount); err != nil {
			return fmt.Errorf("clone messages: %w", err)
		}
	}

	var evtCount int64
	if err := r.QueryRow("SELECT count(*) FROM event").Scan(&evtCount); err != nil {
		return fmt.Errorf("count events: %w", err)
	}
	if evtCount >= targetEvents {
		slog.Info("events already at target", "count", evtCount)
	} else {
		if err := insertSyntheticEvents(w, int(targetEvents-evtCount)); err != nil {
			return fmt.Errorf("insert events: %w", err)
		}
	}

	slog.Info("amplify complete")
	return nil
}

type srcRow struct {
	id        int64
	threadKey string
	mailbox   string
	subject   string
	fromAddr  string
	flags     int
	hasAtt    int
	size      int64
	body      string
}

func cloneToTarget(w, r *sql.DB, currentCount int64) error {
	var minAt, maxAt int64
	if err := r.QueryRow(
		"SELECT min(received_at), max(received_at) FROM message WHERE clone_of IS NULL",
	).Scan(&minAt, &maxAt); err != nil {
		return fmt.Errorf("date range: %w", err)
	}
	if maxAt <= minAt {
		maxAt = minAt + 86400
	}
	dateRange := maxAt - minAt

	rows, err := r.Query(`
		SELECT id, thread_key, mailbox, subject, from_addr, flags, has_attachment, size, body
		FROM message WHERE clone_of IS NULL LIMIT 50000
	`)
	if err != nil {
		return fmt.Errorf("read source rows: %w", err)
	}
	var sources []srcRow
	for rows.Next() {
		var s srcRow
		if err := rows.Scan(&s.id, &s.threadKey, &s.mailbox, &s.subject,
			&s.fromAddr, &s.flags, &s.hasAtt, &s.size, &s.body); err != nil {
			rows.Close()
			return fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("source rows: %w", err)
	}
	if len(sources) == 0 {
		return fmt.Errorf("no source messages for cloning")
	}

	need := targetMessages - int(currentCount)
	slog.Info("cloning", "need", need, "source_pool", len(sources))

	const batchSize = 1000
	cloned := 0
	counter := time.Now().UnixNano()

	for cloned < need {
		tx, err := w.Begin()
		if err != nil {
			return fmt.Errorf("begin clone tx: %w", err)
		}

		stmt, err := tx.Prepare(`
			INSERT OR IGNORE INTO message
				(server_id, thread_key, mailbox, received_at, subject, from_addr,
				 flags, has_attachment, size, body, clone_of, data)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '{}')
		`)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("prepare clone stmt: %w", err)
		}

		limit := min(batchSize, need-cloned)
		for range limit {
			src := sources[rand.IntN(len(sources))]
			jitter := rand.Int64N(dateRange + 1)
			rcvd := minAt + jitter
			counter++
			synthID := fmt.Sprintf("synth-%d-%d", counter, src.id)
			if _, err := stmt.Exec(
				synthID, src.threadKey, src.mailbox, rcvd,
				src.subject, src.fromAddr, src.flags, src.hasAtt,
				src.size, src.body, src.id,
			); err != nil {
				stmt.Close()
				tx.Rollback() //nolint:errcheck
				return fmt.Errorf("exec clone: %w", err)
			}
		}
		stmt.Close()

		if err := tx.Commit(); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("commit clone batch: %w", err)
		}
		cloned += limit
		fmt.Fprintf(os.Stderr, "\rcloned %d / %d", cloned, need)
	}
	fmt.Fprintln(os.Stderr)
	return nil
}

func insertSyntheticEvents(w *sql.DB, n int) error {
	now := time.Now()
	span := int64(18 * 30 * 24 * 3600)

	tx, err := w.Begin()
	if err != nil {
		return fmt.Errorf("begin event tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	evtStmt, err := tx.Prepare(`
		INSERT INTO event (title, location, description, start_at, end_at, is_recurring, raw_ics)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare event insert: %w", err)
	}
	defer evtStmt.Close()

	occStmt, err := tx.Prepare(`
		INSERT INTO event_occurrence (event_id, start_at) VALUES (?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare occurrence insert: %w", err)
	}
	defer occStmt.Close()

	for i := range n {
		title := eventTitles[rand.IntN(len(eventTitles))]
		loc := eventLocations[rand.IntN(len(eventLocations))]
		desc := descWords[rand.IntN(len(descWords))] + " for " + strings.ToLower(title)

		offset := rand.Int64N(span*2+1) - span
		startAt := now.Unix() + offset
		endAt := startAt + 3600

		recurring := 0
		if rand.Float64() < 0.15 {
			recurring = 1
		}

		uid := fmt.Sprintf("synth-ev-%d@perfspike", i)
		startDT := time.Unix(startAt, 0).UTC().Format("20060102T150405Z")
		endDT := time.Unix(endAt, 0).UTC().Format("20060102T150405Z")
		ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//perfspike//EN\r\n" +
			"BEGIN:VEVENT\r\n" +
			"UID:" + uid + "\r\n" +
			"DTSTART:" + startDT + "\r\n" +
			"DTEND:" + endDT + "\r\n" +
			"SUMMARY:" + title + "\r\n" +
			"LOCATION:" + loc + "\r\n" +
			"DESCRIPTION:" + desc + "\r\n" +
			"END:VEVENT\r\nEND:VCALENDAR\r\n"

		res, err := evtStmt.Exec(title, loc, desc, startAt, endAt, recurring, ics)
		if err != nil {
			return fmt.Errorf("insert event %d: %w", i, err)
		}
		evtID, _ := res.LastInsertId()

		if _, err := occStmt.Exec(evtID, startAt); err != nil {
			return fmt.Errorf("insert primary occurrence: %w", err)
		}

		if recurring == 1 {
			for k := 1; k <= 3; k++ {
				occ := startAt + int64(k)*7*24*3600
				if _, err := occStmt.Exec(evtID, occ); err != nil {
					return fmt.Errorf("insert recurring occurrence k=%d: %w", k, err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit events: %w", err)
	}
	slog.Info("inserted events", "count", n)
	return nil
}
