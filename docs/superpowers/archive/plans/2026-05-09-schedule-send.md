# Pass 10b — Schedule send + outbox sidebar implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire schedule-send end to end: cache primitives, compose
`Ctrl+L` picker with text-input custom row, sidebar Outbox synthetic
entry, and an Outbox view supporting cancel / reschedule / edit-as-draft.

**Architecture:** Additive on the schema-v10 foundation from Pass 10a.
Two new cache methods (`RescheduleOp`, `OutboxScheduled`), one new
compose subwidget (schedule picker + parser), one new sidebar seam
(`SetOutboxCount`), one new UI subpackage (`internal/ui/outbox`), and
App-side routing glue that maps Outbox-folder selection to the new
view instead of `messagelist.Model`.

**Tech Stack:** Go 1.26, modernc.org/sqlite, bubbletea, bubbles,
lipgloss. No new third-party dependencies.

**Spec:** `docs/superpowers/specs/2026-05-09-schedule-send-design.md`.

---

## File map

```
internal/cache/
  reschedule.go            (new)         RescheduleOp
  reschedule_test.go       (new)
  outbox_reads.go          (modify)      add OutboxScheduled + OutboxRow
  outbox_reads_test.go     (modify)      add scheduled-rows tests

internal/compose/
  scheduleparse.go         (new)         ParseSchedule
  scheduleparse_test.go    (new)

internal/ui/compose/
  schedulepicker.go        (new)         SchedulePicker sub-model
  schedulepicker_test.go   (new)
  msgs.go                  (modify)      ScheduleAcceptedMsg, ScheduleCancelledMsg, OpenScheduleMsg
  model.go                 (modify)      Ctrl+L route, threading time into QueueOutbound

internal/ui/sidebar/
  model.go                 (modify)      SetOutboxCount + synthetic entry render

internal/ui/outbox/
  model.go                 (new)         Model + Update + View
  msgs.go                  (new)         RefreshMsg, OpenScheduleMsg, OpenComposeAsDraftMsg
  styles.go                (new)
  model_test.go            (new)

internal/ui/
  app.go                   (modify)      route Outbox selection, OutboxCount refresh, c/s/e dispatch

docs/poplar/
  keybindings.md           (modify)      Ctrl+L row + Outbox section
```

---

## Task 1: Cache — `RescheduleOp`

**Files:**
- Create: `internal/cache/reschedule.go`
- Create: `internal/cache/reschedule_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/cache/reschedule_test.go
package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRescheduleOp_UpdatesPendingRow(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	original := time.Now().Add(1 * time.Hour).UnixNano()
	id, err := a.QueueSend(ctx, "Sent", testEnvelope(), []byte("MIME"), original, "")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	want := time.Now().Add(2 * time.Hour).UnixNano()
	if err := a.RescheduleOp(ctx, id, want); err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	got := readScheduledFor(t, a, id)
	if got != want {
		t.Errorf("scheduled_for: got %d, want %d", got, want)
	}
}

func TestRescheduleOp_RejectsRowAboutToDispatch(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	past := time.Now().Add(-1 * time.Second).UnixNano()
	id, err := a.QueueSend(ctx, "Sent", testEnvelope(), []byte("MIME"), past, "")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	err = a.RescheduleOp(ctx, id, time.Now().Add(1*time.Hour).UnixNano())
	if !errors.Is(err, ErrNotPending) {
		t.Errorf("got %v, want ErrNotPending", err)
	}
}

func TestRescheduleOp_RejectsAdvancedRow(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	id, err := a.QueueSend(ctx, "Sent", testEnvelope(), []byte("MIME"),
		time.Now().Add(1*time.Hour).UnixNano(), "")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	markStatus(t, a, id, OpDone)
	err = a.RescheduleOp(ctx, id, time.Now().Add(2*time.Hour).UnixNano())
	if !errors.Is(err, ErrNotPending) {
		t.Errorf("got %v, want ErrNotPending", err)
	}
}
```

If `openTestAccount`, `testEnvelope`, `readScheduledFor`, or `markStatus` are not present, write minimal versions in this test file modeled on existing patterns in `cancel_test.go` / `send_test.go`.

- [ ] **Step 2: Run test, confirm it fails**

```
go test ./internal/cache -run TestRescheduleOp -v
```
Expected: undefined `Account.RescheduleOp`.

- [ ] **Step 3: Implement `RescheduleOp`**

```go
// internal/cache/reschedule.go
package cache

import (
	"context"
	"time"
)

// RescheduleOp updates the scheduled_for of an outbox row that is
// still pending and not yet eligible for pickup. Returns ErrNotPending
// when the row has advanced or already passed its scheduled time.
func (a *Account) RescheduleOp(ctx context.Context, opID int64, newScheduledFor int64) error {
	res, err := a.db.ExecContext(ctx, `
        UPDATE outbox
           SET scheduled_for = ?
         WHERE id = ?
           AND status = ?
           AND scheduled_for IS NOT NULL
           AND scheduled_for > ?`,
		newScheduledFor, opID, OpPending, time.Now().UnixNano())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotPending
	}
	return nil
}
```

- [ ] **Step 4: Run test, confirm green**

```
go test ./internal/cache -run TestRescheduleOp -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/cache/reschedule.go internal/cache/reschedule_test.go
git commit -m "Pass 10b: cache.RescheduleOp"
```

---

## Task 2: Cache — `OutboxScheduled` read

**Files:**
- Modify: `internal/cache/outbox_reads.go` (append `OutboxRow` + method)
- Modify: `internal/cache/outbox_reads_test.go` (append tests)

- [ ] **Step 1: Write failing tests**

```go
// internal/cache/outbox_reads_test.go (additions)
func TestOutboxScheduled_OrdersByScheduledForAsc(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	later := time.Now().Add(2 * time.Hour).UnixNano()
	earlier := time.Now().Add(1 * time.Hour).UnixNano()
	if _, err := a.QueueSend(ctx, "Sent", testEnvelope(), buildMIME("later"), later, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.QueueSend(ctx, "Sent", testEnvelope(), buildMIME("earlier"), earlier, ""); err != nil {
		t.Fatal(err)
	}
	rows, err := a.OutboxScheduled(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].Subject != "earlier" || rows[1].Subject != "later" {
		t.Errorf("order: got %q,%q want earlier,later", rows[0].Subject, rows[1].Subject)
	}
}

func TestOutboxScheduled_HydratesDraftViaJoin(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	draftID := putTestDraft(t, a, "draft-1")
	when := time.Now().Add(1 * time.Hour).UnixNano()
	if _, err := a.QueueSend(ctx, "Sent", testEnvelope(), buildMIME("subj"), when, draftID); err != nil {
		t.Fatal(err)
	}
	rows, err := a.OutboxScheduled(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(rows) != 1 || rows[0].Draft == nil {
		t.Fatalf("draft not hydrated: %+v", rows)
	}
	if rows[0].Draft.DraftID != draftID {
		t.Errorf("draft id: got %q, want %q", rows[0].Draft.DraftID, draftID)
	}
}

func TestOutboxScheduled_DecodesSubjectFromMIME(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	mime := []byte("From: a@x\r\nTo: b@x\r\nSubject: hello there\r\n\r\nbody")
	if _, err := a.QueueSend(ctx, "Sent", testEnvelope(), mime,
		time.Now().Add(1*time.Hour).UnixNano(), ""); err != nil {
		t.Fatal(err)
	}
	rows, err := a.OutboxScheduled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Subject != "hello there" {
		t.Errorf("subject: got %q, want %q", rows[0].Subject, "hello there")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```
go test ./internal/cache -run TestOutboxScheduled -v
```

- [ ] **Step 3: Implement**

Append to `internal/cache/outbox_reads.go`:

```go
// OutboxRow is one entry in the user-facing scheduled-outbox view.
type OutboxRow struct {
	ID           int64
	Kind         OpKind
	Folder       string
	To           []string
	Subject      string
	ScheduledFor time.Time
	Status       OpStatus
	Attempts     int
	LastError    string
	Draft        *DraftRow
}

// OutboxScheduled returns pending and failed outbox rows ordered by
// scheduled_for ascending. Rows with NULL scheduled_for sort last.
// Linked draft rows are joined via outbox.draft_id.
func (a *Account) OutboxScheduled(ctx context.Context) ([]OutboxRow, error) {
	const q = `
        SELECT o.id, o.kind, COALESCE(f.name, ''),
               o.args, o.payload,
               COALESCE(o.scheduled_for, 0),
               o.status, o.attempts, COALESCE(o.error, ''),
               d.draft_id, COALESCE(d.server_uid, ''), COALESCE(d.server_folder, ''),
               d.payload, d.dirty, d.created_at, d.updated_at,
               COALESCE(d.last_pushed_at, 0)
          FROM outbox o
          LEFT JOIN folders f ON f.id = o.folder
          LEFT JOIN drafts d  ON d.draft_id = o.draft_id
         WHERE o.status IN (?, ?)
         ORDER BY CASE WHEN o.scheduled_for IS NULL THEN 1 ELSE 0 END,
                  o.scheduled_for ASC,
                  o.id ASC`
	rows, err := a.db.QueryContext(ctx, q, OpPending, OpFailed)
	if err != nil {
		return nil, fmt.Errorf("outbox scheduled: %w", err)
	}
	defer rows.Close()

	var out []OutboxRow
	for rows.Next() {
		var r OutboxRow
		var kind, status, errPayload string
		var argsJSON, payload []byte
		var scheduledNS int64
		var draftID, serverUID, serverFolder sql.NullString
		var draftPayload []byte
		var dirty sql.NullInt64
		var created, updated, pushed sql.NullInt64
		if err := rows.Scan(&r.ID, &kind, &r.Folder, &argsJSON, &payload,
			&scheduledNS, &status, &r.Attempts, &errPayload,
			&draftID, &serverUID, &serverFolder, &draftPayload,
			&dirty, &created, &updated, &pushed); err != nil {
			return nil, err
		}
		r.Kind = OpKind(kind)
		r.Status = OpStatus(status)
		_, r.LastError = decodeErrorPayload(errPayload)
		if scheduledNS != 0 {
			r.ScheduledFor = time.Unix(0, scheduledNS)
		}
		r.To, r.Subject = decodeRowMeta(r.Kind, argsJSON, payload)
		if draftID.Valid {
			d := &DraftRow{
				DraftID:      draftID.String,
				ServerUID:    mail.UID(serverUID.String),
				ServerFolder: serverFolder.String,
				Payload:      draftPayload,
				Dirty:        dirty.Int64 != 0,
			}
			if created.Valid {
				d.CreatedAt = time.Unix(0, created.Int64)
			}
			if updated.Valid {
				d.UpdatedAt = time.Unix(0, updated.Int64)
			}
			if pushed.Valid && pushed.Int64 != 0 {
				d.LastPushedAt = time.Unix(0, pushed.Int64)
			}
			r.Draft = d
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// decodeRowMeta extracts To addresses from args JSON (Send only) and
// the Subject header from the first 4 KB of payload. A missing or
// malformed value returns "".
func decodeRowMeta(kind OpKind, argsJSON, payload []byte) (to []string, subject string) {
	if kind == KindSend {
		var sa SendArgs
		if err := json.Unmarshal(argsJSON, &sa); err == nil {
			to = sa.Envelope.Rcpts
		}
	}
	subject = extractSubject(payload)
	return
}

// extractSubject reads the Subject header from the first 4 KB of a
// MIME payload. Header parsing stops at the blank-line CRLF CRLF
// terminator. Folded continuation lines are joined with a single
// space.
func extractSubject(payload []byte) string {
	const cap = 4096
	if len(payload) > cap {
		payload = payload[:cap]
	}
	br := bufio.NewReader(bytes.NewReader(payload))
	tp := textproto.NewReader(br)
	hdr, err := tp.ReadMIMEHeader()
	if err != nil && err != io.EOF {
		return ""
	}
	return hdr.Get("Subject")
}
```

Add the imports: `"bufio"`, `"bytes"`, `"database/sql"`, `"encoding/json"`, `"io"`, `"net/textproto"`, `"github.com/glw907/poplar/internal/mail"`.

- [ ] **Step 4: Run, confirm green**

```
go test ./internal/cache -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/cache/outbox_reads.go internal/cache/outbox_reads_test.go
git commit -m "Pass 10b: cache.OutboxScheduled with draft join"
```

---

## Task 3: Schedule parser — `compose.ParseSchedule`

**Files:**
- Create: `internal/compose/scheduleparse.go`
- Create: `internal/compose/scheduleparse_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/compose/scheduleparse_test.go
package compose

import (
	"testing"
	"time"
)

func TestParseSchedule(t *testing.T) {
	now := time.Date(2026, 5, 9, 14, 30, 0, 0, time.Local) // Sat
	cases := []struct {
		in   string
		want time.Time
	}{
		{"2026-05-15 09:00", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"2026-05-15 9 AM", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"2026-05-15", time.Date(2026, 5, 15, 0, 0, 0, 0, time.Local)},
		{"05/15 09:00", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"5/15 9am", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"05/15/2026 09:00", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"May 15 9am", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"15 May 9am", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"09:00", time.Date(2026, 5, 10, 9, 0, 0, 0, time.Local)}, // past today → tomorrow
		{"15:00", time.Date(2026, 5, 9, 15, 0, 0, 0, time.Local)},
		{"3pm", time.Date(2026, 5, 9, 15, 0, 0, 0, time.Local)},
		{"tomorrow", time.Date(2026, 5, 10, 9, 0, 0, 0, time.Local)},
		{"tomorrow 3pm", time.Date(2026, 5, 10, 15, 0, 0, 0, time.Local)},
		{"tonight", time.Date(2026, 5, 9, 21, 0, 0, 0, time.Local)},
		{"next monday", time.Date(2026, 5, 18, 9, 0, 0, 0, time.Local)}, // next-week monday
		{"monday", time.Date(2026, 5, 11, 9, 0, 0, 0, time.Local)},      // first upcoming mon
		{"monday 8am", time.Date(2026, 5, 11, 8, 0, 0, 0, time.Local)},
		{"+30m", now.Add(30 * time.Minute)},
		{"+2h", now.Add(2 * time.Hour)},
		{"+3d", now.Add(72 * time.Hour)},
		{"03/05", time.Date(2027, 3, 5, 0, 0, 0, 0, time.Local)}, // past in current year → next year
	}
	for _, c := range cases {
		got, err := ParseSchedule(c.in, now)
		if err != nil {
			t.Errorf("%q: err %v", c.in, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("%q: got %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseSchedule_Errors(t *testing.T) {
	now := time.Date(2026, 5, 9, 14, 30, 0, 0, time.Local)
	for _, in := range []string{"", "garbage", "32:00", "13/40"} {
		if _, err := ParseSchedule(in, now); err == nil {
			t.Errorf("%q: want error, got nil", in)
		}
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```
go test ./internal/compose -run TestParseSchedule -v
```

- [ ] **Step 3: Implement parser**

```go
// internal/compose/scheduleparse.go
package compose

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrUnrecognized reports input that didn't match any accepted shape.
var ErrUnrecognized = errors.New("compose: not a recognized date — try \"tomorrow 3pm\" or \"2026-05-15 09:00\"")

// ParseSchedule parses a user-typed schedule string against now.
// Accepts ISO/US/English dates, time-only strings (today or rolled
// to tomorrow), keyword shortcuts (tomorrow, tonight, next <day>,
// <day>), and offsets (+Nm/+Nh/+Nd). Year defaults to now.Year() and
// past results roll forward by one unit.
func ParseSchedule(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, ErrUnrecognized
	}
	lower := strings.ToLower(s)

	if t, ok := parseRelative(lower, now); ok {
		return t, nil
	}
	if t, ok := parseKeyword(lower, now); ok {
		return t, nil
	}
	return parseLayouts(s, now)
}

var relRe = regexp.MustCompile(`^\+(\d+)([mhd])$`)

func parseRelative(s string, now time.Time) (time.Time, bool) {
	m := relRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	n, _ := strconv.Atoi(m[1])
	switch m[2] {
	case "m":
		return now.Add(time.Duration(n) * time.Minute), true
	case "h":
		return now.Add(time.Duration(n) * time.Hour), true
	case "d":
		return now.Add(time.Duration(n) * 24 * time.Hour), true
	}
	return time.Time{}, false
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

func parseKeyword(s string, now time.Time) (time.Time, bool) {
	// Split off optional trailing time clause.
	day, timePart := splitTimeTail(s)

	hh, mm, hasTime := parseTimeOnly(timePart)
	if timePart != "" && !hasTime {
		return time.Time{}, false
	}

	defH, defM := 9, 0

	switch {
	case day == "tomorrow":
		t := now.AddDate(0, 0, 1)
		return atHM(t, pick(hh, defH, hasTime), pick(mm, defM, hasTime)), true
	case day == "tonight":
		t := atHM(now, 21, 0)
		if t.Before(now) {
			t = t.AddDate(0, 0, 1)
		}
		return t, true
	}

	// "next <weekday>" → following occurrence (always +1 week from
	// the next upcoming).
	if rest, ok := strings.CutPrefix(day, "next "); ok {
		if wd, ok := weekdays[rest]; ok {
			t := nextWeekday(now, wd, true)
			return atHM(t, pick(hh, defH, hasTime), pick(mm, defM, hasTime)), true
		}
	}
	if wd, ok := weekdays[day]; ok {
		t := nextWeekday(now, wd, false)
		return atHM(t, pick(hh, defH, hasTime), pick(mm, defM, hasTime)), true
	}

	return time.Time{}, false
}

// splitTimeTail returns (head, tail) split on the first space whose
// suffix parses as a time-only string. Falls back to (s, "").
func splitTimeTail(s string) (head, tail string) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != ' ' {
			continue
		}
		if _, _, ok := parseTimeOnly(s[i+1:]); ok {
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
		}
	}
	return s, ""
}

var timeOnlyLayouts = []string{"15:04", "3:04 PM", "3:04PM", "3 PM", "3PM", "3pm", "3am"}

func parseTimeOnly(s string) (hh, mm int, ok bool) {
	if s == "" {
		return 0, 0, false
	}
	upper := strings.ToUpper(s)
	for _, layout := range timeOnlyLayouts {
		if t, err := time.Parse(layout, upper); err == nil {
			return t.Hour(), t.Minute(), true
		}
	}
	return 0, 0, false
}

func atHM(t time.Time, h, m int) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), h, m, 0, 0, t.Location())
}

func pick(v, def int, has bool) int {
	if has {
		return v
	}
	return def
}

// nextWeekday returns the next date at midnight whose weekday matches wd.
// If skipThisWeek, advances by another 7 days.
func nextWeekday(now time.Time, wd time.Weekday, skipThisWeek bool) time.Time {
	delta := (int(wd) - int(now.Weekday()) + 7) % 7
	if delta == 0 {
		delta = 7
	}
	t := now.AddDate(0, 0, delta)
	if skipThisWeek {
		t = t.AddDate(0, 0, 7)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// dateLayouts is tried in order. Layouts without a year (no "2006")
// get year defaulting + past-rolling applied after parse.
var dateLayouts = []struct {
	layout  string
	hasYear bool
	hasTime bool
}{
	{"2006-01-02 15:04", true, true},
	{"2006-01-02 3:04 PM", true, true},
	{"2006-01-02 3 PM", true, true},
	{"2006-01-02 3pm", true, true},
	{"2006-01-02", true, false},
	{"01/02/2006 15:04", true, true},
	{"01/02/2006 3:04 PM", true, true},
	{"01/02/2006", true, false},
	{"01/02 15:04", false, true},
	{"01/02 3:04 PM", false, true},
	{"01/02 3pm", false, true},
	{"01/02 3am", false, true},
	{"01/02", false, false},
	{"Jan 2 2006 15:04", true, true},
	{"Jan 2 2006", true, false},
	{"Jan 2 15:04", false, true},
	{"Jan 2 3:04 PM", false, true},
	{"Jan 2 3pm", false, true},
	{"Jan 2", false, false},
	{"2 Jan 2006 15:04", true, true},
	{"2 Jan 2006", true, false},
	{"2 Jan 15:04", false, true},
	{"2 Jan 3pm", false, true},
	{"2 Jan", false, false},
}

func parseLayouts(s string, now time.Time) (time.Time, error) {
	if t, ok := parseTimeAlone(s, now); ok {
		return t, nil
	}
	for _, l := range dateLayouts {
		t, err := time.ParseInLocation(l.layout, s, now.Location())
		if err != nil {
			continue
		}
		if !l.hasYear {
			t = time.Date(now.Year(), t.Month(), t.Day(),
				t.Hour(), t.Minute(), 0, 0, now.Location())
			if t.Before(now) {
				t = t.AddDate(1, 0, 0)
			}
		}
		return t, nil
	}
	return time.Time{}, ErrUnrecognized
}

// parseTimeAlone matches H[:MM][am/pm] / HH:MM and rolls to tomorrow
// when the result is before now.
func parseTimeAlone(s string, now time.Time) (time.Time, bool) {
	hh, mm, ok := parseTimeOnly(s)
	if !ok {
		return time.Time{}, false
	}
	t := atHM(now, hh, mm)
	if t.Before(now) {
		t = t.AddDate(0, 0, 1)
	}
	return t, true
}
```

- [ ] **Step 4: Run, confirm green**

```
go test ./internal/compose -run TestParseSchedule -v
```

If a case fails because Go's `time.Parse` rejects a layout literal, adjust `timeOnlyLayouts` / `dateLayouts` until all green. Some `3pm`-style literals don't round-trip — fall through `parseTimeAlone` first and let the layout sweep catch the date variant.

- [ ] **Step 5: Commit**

```bash
git add internal/compose/scheduleparse.go internal/compose/scheduleparse_test.go
git commit -m "Pass 10b: compose.ParseSchedule"
```

---

## Task 4: Schedule picker UI

**Files:**
- Create: `internal/ui/compose/schedulepicker.go`
- Create: `internal/ui/compose/schedulepicker_test.go`
- Modify: `internal/ui/compose/msgs.go` (add 3 messages)

- [ ] **Step 1: Define messages**

Append to `internal/ui/compose/msgs.go`:

```go
// ScheduleAcceptedMsg is emitted when the user picks a schedule preset
// or commits a parsed custom time.
type ScheduleAcceptedMsg struct{ When time.Time }

// ScheduleCancelledMsg is emitted when the user dismisses the picker.
type ScheduleCancelledMsg struct{}

// OpenScheduleMsg requests the App open the picker pre-filled. Used by
// the outbox view's "s reschedule" action.
type OpenScheduleMsg struct {
	OpID    int64
	Initial string // formatted "2006-01-02 15:04"
}
```

Add `"time"` to the imports.

- [ ] **Step 2: Write failing tests**

```go
// internal/ui/compose/schedulepicker_test.go
package compose

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
)

func TestSchedulePicker_PresetCommitsTime(t *testing.T) {
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.Local) // Sat
	p := NewSchedulePicker(theme.OneDark(), now, "")
	p.MoveDown() // tomorrow afternoon
	m, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = m
	if cmd == nil {
		t.Fatal("preset Enter: cmd nil")
	}
	got, ok := cmd().(ScheduleAcceptedMsg)
	if !ok {
		t.Fatalf("cmd: %T, want ScheduleAcceptedMsg", cmd())
	}
	want := time.Date(2026, 5, 10, 13, 0, 0, 0, time.Local)
	if !got.When.Equal(want) {
		t.Errorf("got %v, want %v", got.When, want)
	}
}

func TestSchedulePicker_CustomExpandsAndParses(t *testing.T) {
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.Local)
	p := NewSchedulePicker(theme.OneDark(), now, "")
	for i := 0; i < 3; i++ {
		p.MoveDown()
	}
	m, _ := p.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand
	p = m.(SchedulePicker)
	if !p.customOpen {
		t.Fatal("custom row should be open")
	}
	for _, r := range "tomorrow 3pm" {
		m, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		p = m.(SchedulePicker)
	}
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("custom Enter: cmd nil")
	}
	got, ok := cmd().(ScheduleAcceptedMsg)
	if !ok {
		t.Fatalf("cmd: %T", cmd())
	}
	want := time.Date(2026, 5, 10, 15, 0, 0, 0, time.Local)
	if !got.When.Equal(want) {
		t.Errorf("got %v, want %v", got.When, want)
	}
}

func TestSchedulePicker_CustomShowsParseError(t *testing.T) {
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.Local)
	p := NewSchedulePicker(theme.OneDark(), now, "")
	for i := 0; i < 3; i++ {
		p.MoveDown()
	}
	m, _ := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p = m.(SchedulePicker)
	for _, r := range "garbage" {
		m, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		p = m.(SchedulePicker)
	}
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("parse error: cmd should be nil")
	}
	if !strings.Contains(p.View(), "not a recognized date") {
		t.Errorf("view should show parse hint:\n%s", p.View())
	}
}

func TestSchedulePicker_EscCancels(t *testing.T) {
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.Local)
	p := NewSchedulePicker(theme.OneDark(), now, "")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	if _, ok := cmd().(ScheduleCancelledMsg); !ok {
		t.Errorf("got %T, want ScheduleCancelledMsg", cmd())
	}
}
```

- [ ] **Step 3: Run, confirm fail**

```
go test ./internal/ui/compose -run TestSchedulePicker -v
```

- [ ] **Step 4: Implement**

```go
// internal/ui/compose/schedulepicker.go
package compose

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// SchedulePicker is the modal sub-model for picking a send time. It
// owns three preset rows plus a Custom row that, on Enter, expands a
// textinput with a free-form parser.
type SchedulePicker struct {
	now        time.Time
	cursor     int  // 0..3 (3 = Custom)
	customOpen bool
	input      textinput.Model
	parseErr   string
	styles     Styles
	width      int
	height     int
}

// NewSchedulePicker builds the picker against now. Initial seeds the
// Custom-row textinput when non-empty (used by reschedule).
func NewSchedulePicker(t *theme.CompiledTheme, now time.Time, initial string) SchedulePicker {
	in := textinput.New()
	in.Prompt = "  "
	in.Placeholder = "tomorrow 3pm"
	in.SetValue(initial)
	in.CharLimit = 64
	p := SchedulePicker{
		now:    now,
		styles: NewStyles(t),
		input:  in,
	}
	if initial != "" {
		p.cursor = 3
		p.customOpen = true
		p.input.Focus()
	}
	return p
}

func (p SchedulePicker) presets() []presetRow {
	return []presetRow{
		{"Tomorrow morning", "8:00 AM", atHM(p.now.AddDate(0, 0, 1), 8, 0)},
		{"Tomorrow afternoon", "1:00 PM", atHM(p.now.AddDate(0, 0, 1), 13, 0)},
		{"Monday morning", "8:00 AM", nextMonday8(p.now)},
	}
}

type presetRow struct {
	label string
	time  string
	when  time.Time
}

// nextMonday8 returns the upcoming Monday at 08:00, advancing one
// week if today is Monday (matches Gmail).
func nextMonday8(now time.Time) time.Time {
	delta := (int(time.Monday) - int(now.Weekday()) + 7) % 7
	if delta == 0 {
		delta = 7
	}
	d := now.AddDate(0, 0, delta)
	return atHM(d, 8, 0)
}

func atHM(t time.Time, h, m int) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), h, m, 0, 0, t.Location())
}

func (p *SchedulePicker) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
		p.customOpen = false
	}
}

func (p *SchedulePicker) MoveDown() {
	if p.cursor < 3 {
		p.cursor++
		if p.cursor != 3 {
			p.customOpen = false
		}
	}
}

func (p SchedulePicker) Update(msg tea.Msg) (SchedulePicker, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	if p.customOpen {
		switch km.Type {
		case tea.KeyEnter:
			when, err := compose.ParseSchedule(p.input.Value(), p.now)
			if err != nil {
				p.parseErr = "not a recognized date — try \"tomorrow 3pm\" or \"2026-05-15 09:00\""
				return p, nil
			}
			return p, func() tea.Msg { return ScheduleAcceptedMsg{When: when} }
		case tea.KeyEsc:
			return p, func() tea.Msg { return ScheduleCancelledMsg{} }
		}
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		p.parseErr = ""
		return p, cmd
	}

	switch {
	case km.Type == tea.KeyUp || (km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == 'k'):
		p.MoveUp()
	case km.Type == tea.KeyDown || (km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == 'j'):
		p.MoveDown()
	case km.Type == tea.KeyEsc:
		return p, func() tea.Msg { return ScheduleCancelledMsg{} }
	case km.Type == tea.KeyEnter:
		if p.cursor == 3 {
			p.customOpen = true
			p.input.Focus()
			return p, textinput.Blink
		}
		when := p.presets()[p.cursor].when
		return p, func() tea.Msg { return ScheduleAcceptedMsg{When: when} }
	}
	return p, nil
}

func (p *SchedulePicker) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.input.Width = w - 8
}

func (p SchedulePicker) View() string {
	var b strings.Builder
	for i, r := range p.presets() {
		marker := "  "
		if i == p.cursor {
			marker = "▶ "
		}
		b.WriteString(marker + padLabel(r.label, 22) + r.time + "\n")
	}
	customMarker := "  "
	if p.cursor == 3 {
		customMarker = "▶ "
	}
	b.WriteString(customMarker + "Custom…\n")
	if p.customOpen {
		b.WriteString(p.input.View() + "\n")
		if p.parseErr != "" {
			b.WriteString(p.styles.SidebarFolder.Render(p.parseErr))
		}
	}
	body := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	footer := []string{"j/k nav  ⏎ pick  Esc cancel"}
	contentW := 44
	shell := uicore.ModalShell{}
	return shell.Box("Schedule send", body, footer, contentW)
}

func padLabel(s string, w int) string {
	if lipgloss.Width(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}
```

- [ ] **Step 5: Run, confirm green**

```
go test ./internal/ui/compose -run TestSchedulePicker -v
```

If `theme.OneDark()` constructor name differs, adjust the test to call the existing one (e.g. `theme.NewOneDark()` — match the file's existing convention).

- [ ] **Step 6: Commit**

```bash
git add internal/ui/compose/schedulepicker.go internal/ui/compose/schedulepicker_test.go internal/ui/compose/msgs.go
git commit -m "Pass 10b: schedule picker sub-model"
```

---

## Task 5: Compose `Ctrl+L` wiring

**Files:**
- Modify: `internal/ui/compose/model.go`

- [ ] **Step 1: Inspect current send flow**

```
grep -n "QueueOutbound\|ctrl+x\|KeyCtrlX\|focusBody\|c\.scheduledFor" internal/ui/compose/model.go
```

Identify (a) the field where compose holds its eventual `time.Time` (currently absent — `QueueOutbound` always passes 0), (b) where Ctrl+X dispatches send, (c) where overlays compose with the editor (mirroring AttachPicker's pattern).

- [ ] **Step 2: Add picker field + Ctrl+L route + send-with-time**

In `compose.Model`, add:

```go
schedulePicker *SchedulePicker // non-nil while open
scheduledFor   time.Time       // zero = send immediately
```

In the `Update` switch, before delegating to focused widget:

```go
if m.schedulePicker != nil {
    p, cmd := m.schedulePicker.Update(msg)
    m.schedulePicker = &p
    return m, cmd
}
```

Add a top-level branch handling `ScheduleAcceptedMsg` / `ScheduleCancelledMsg` / `OpenScheduleMsg`:

```go
case ScheduleAcceptedMsg:
    m.scheduledFor = msg.When
    m.schedulePicker = nil
    return m, m.dispatchSend()
case ScheduleCancelledMsg:
    m.schedulePicker = nil
    return m, nil
case OpenScheduleMsg:
    p := NewSchedulePicker(m.theme, time.Now(), msg.Initial)
    p.SetSize(m.width, m.height)
    m.schedulePicker = &p
    return m, nil
```

Add the Ctrl+L key route in the existing keymap dispatch:

```go
case key.Matches(km, m.keys.Schedule): // ^L
    p := NewSchedulePicker(m.theme, time.Now(), "")
    p.SetSize(m.width, m.height)
    m.schedulePicker = &p
    return m, nil
```

Add the binding to `compose/bind.go` (`Schedule key.Binding` with `key.WithKeys("ctrl+l")`).

In the existing send dispatch (Ctrl+X path), replace the `0` literal with `m.scheduledFor.UnixNano()` (zero-time `UnixNano()` is a large negative; guard with a helper):

```go
func unixNanoOrZero(t time.Time) int64 {
    if t.IsZero() {
        return 0
    }
    return t.UnixNano()
}
```

`QueueOutbound(..., unixNanoOrZero(m.scheduledFor), draftID)`.

In `View`, when `m.schedulePicker != nil` overlay it via `uicore.PlaceOverlay` over the compose render — same pattern AttachPicker uses. Find that block and add a sibling branch.

- [ ] **Step 3: Verify build**

```
go build ./...
```

- [ ] **Step 4: Verify existing tests still pass**

```
go test ./internal/ui/compose/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/ui/compose/model.go internal/ui/compose/bind.go
git commit -m "Pass 10b: compose Ctrl+L route + send-with-scheduled-time"
```

---

## Task 6: Sidebar synthetic Outbox entry

**Files:**
- Modify: `internal/ui/sidebar/model.go`
- Modify: `internal/ui/sidebar/model_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/ui/sidebar/model_test.go (additions)
func TestSidebar_OutboxEntryHiddenWhenZero(t *testing.T) {
	m := newTestModel(t)
	m.SetOutboxCount(0)
	if strings.Contains(m.View(), "Outbox") {
		t.Errorf("View should omit Outbox when count is 0:\n%s", m.View())
	}
}

func TestSidebar_OutboxEntryAppearsAboveTrash(t *testing.T) {
	m := newTestModel(t) // includes Trash in Disposal
	m.SetOutboxCount(3)
	view := m.View()
	if !strings.Contains(view, "Outbox") {
		t.Fatalf("Outbox missing:\n%s", view)
	}
	if !strings.Contains(view, "3") {
		t.Errorf("badge missing:\n%s", view)
	}
	idxOutbox := strings.Index(view, "Outbox")
	idxTrash := strings.Index(view, "Trash")
	if idxOutbox >= idxTrash {
		t.Errorf("Outbox should render above Trash; got Outbox@%d Trash@%d", idxOutbox, idxTrash)
	}
}

func TestSidebar_SelectByCanonicalOutbox(t *testing.T) {
	m := newTestModel(t)
	m.SetOutboxCount(2)
	if !m.SelectByCanonical("Outbox") {
		t.Fatal("SelectByCanonical(Outbox) returned false")
	}
	if m.SelectedCanonical() != "Outbox" {
		t.Errorf("SelectedCanonical: got %q, want Outbox", m.SelectedCanonical())
	}
}
```

`newTestModel` should mirror existing patterns in `model_test.go`, including a Trash folder in the classified list.

- [ ] **Step 2: Run, confirm fail**

```
go test ./internal/ui/sidebar -run TestSidebar_Outbox -v
```

- [ ] **Step 3: Implement**

In `internal/ui/sidebar/model.go`:

Add field to `Model`:

```go
outboxCount int
```

Add method:

```go
// SetOutboxCount sets the depth of the synthetic Outbox entry. When n > 0,
// the entry renders at the top of the Disposal group; when 0, it is omitted.
func (s *Model) SetOutboxCount(n int) {
    if n < 0 {
        n = 0
    }
    s.outboxCount = n
}
```

Modify `buildEntries` (or wrap it) to inject the synthetic entry. Since `buildEntries` is called from `New` and `SetFolders` which don't know the count, the cleanest path is to compose at render time: keep `buildEntries` pure and make `View`/selection helpers consult `s.outboxCount`. Add a helper:

```go
// effectiveEntries returns s.entries with the synthetic Outbox row
// injected at the top of the Disposal group when outboxCount > 0.
func (s Model) effectiveEntries() []folderEntry {
    if s.outboxCount == 0 {
        return s.entries
    }
    out := make([]folderEntry, 0, len(s.entries)+1)
    inserted := false
    for _, e := range s.entries {
        if !inserted && e.cf.Group == mail.GroupDisposal {
            out = append(out, syntheticOutboxEntry(s.outboxCount, s.icons))
            inserted = true
        }
        out = append(out, e)
    }
    if !inserted {
        out = append(out, syntheticOutboxEntry(s.outboxCount, s.icons))
    }
    return out
}

func syntheticOutboxEntry(count int, icons uicore.IconSet) folderEntry {
    return folderEntry{
        cf: mail.ClassifiedFolder{
            Folder:      mail.Folder{Name: "Outbox", Unseen: count},
            Canonical:   "Outbox",
            DisplayName: "Outbox",
            Group:       mail.GroupDisposal,
        },
        icon: icons.Sent, // closest visual fit; a dedicated icon is post-1.0
    }
}
```

Update `View`, `SelectedFolder`, `SelectedCanonical`, `SelectedFolderInfo`, `SelectByCanonical`, `MoveUp`, `MoveDown`, `OrderedFolders`, `FolderNameByCanonical`, `FolderByProviderName` to call `s.effectiveEntries()` instead of `s.entries`.

The unread badge for the synthetic row reuses `Unseen` (set from the count). That gives the `(N)` appearance natively without further work.

- [ ] **Step 4: Run, confirm green**

```
go test ./internal/ui/sidebar -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/ui/sidebar/model.go internal/ui/sidebar/model_test.go
git commit -m "Pass 10b: sidebar SetOutboxCount + synthetic Disposal entry"
```

---

## Task 7: Outbox subpackage — model + view

**Files:**
- Create: `internal/ui/outbox/model.go`
- Create: `internal/ui/outbox/msgs.go`
- Create: `internal/ui/outbox/styles.go`
- Create: `internal/ui/outbox/model_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/ui/outbox/model_test.go
package outbox

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/theme"
)

func TestModel_RendersEmptyState(t *testing.T) {
	m := New(theme.OneDark())
	m.SetSize(80, 20)
	if !strings.Contains(m.View(), "Outbox is empty") {
		t.Errorf("empty state missing:\n%s", m.View())
	}
}

func TestModel_RendersRows(t *testing.T) {
	m := New(theme.OneDark())
	m.SetSize(80, 20)
	when := time.Now().Add(2 * time.Hour)
	m.SetRows([]cache.OutboxRow{{
		ID: 1, Kind: cache.KindSend, Subject: "deploy plan",
		To: []string{"a@x"}, ScheduledFor: when, Status: cache.OpPending,
	}})
	v := m.View()
	if !strings.Contains(v, "deploy plan") {
		t.Errorf("subject missing:\n%s", v)
	}
	if !strings.Contains(v, "a@x") {
		t.Errorf("recipient missing:\n%s", v)
	}
}

func TestModel_CancelEmitsMsg(t *testing.T) {
	m := New(theme.OneDark())
	m.SetSize(80, 20)
	m.SetRows([]cache.OutboxRow{
		{ID: 7, Kind: cache.KindSend, Subject: "x", ScheduledFor: time.Now().Add(time.Hour)},
	})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	got, ok := cmd().(CancelMsg)
	if !ok {
		t.Fatalf("got %T, want CancelMsg", cmd())
	}
	if got.OpID != 7 {
		t.Errorf("OpID: got %d, want 7", got.OpID)
	}
}

func TestModel_ReschedulePrefilledFromRow(t *testing.T) {
	m := New(theme.OneDark())
	m.SetSize(80, 20)
	when := time.Date(2026, 6, 1, 9, 0, 0, 0, time.Local)
	m.SetRows([]cache.OutboxRow{{ID: 9, ScheduledFor: when}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got, ok := cmd().(RescheduleMsg)
	if !ok {
		t.Fatalf("got %T, want RescheduleMsg", cmd())
	}
	if got.OpID != 9 {
		t.Errorf("OpID: got %d, want 9", got.OpID)
	}
	if got.Initial != "2026-06-01 09:00" {
		t.Errorf("Initial: got %q, want 2026-06-01 09:00", got.Initial)
	}
}

func TestModel_EditAsDraftEmitsMsg(t *testing.T) {
	m := New(theme.OneDark())
	m.SetSize(80, 20)
	d := &cache.DraftRow{DraftID: "d1"}
	m.SetRows([]cache.OutboxRow{{ID: 4, Draft: d}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	got, ok := cmd().(EditAsDraftMsg)
	if !ok {
		t.Fatalf("got %T, want EditAsDraftMsg", cmd())
	}
	if got.Draft == nil || got.Draft.DraftID != "d1" {
		t.Errorf("draft not threaded: %+v", got.Draft)
	}
}

func TestModel_EditAsDraftWithoutDraftIsInert(t *testing.T) {
	m := New(theme.OneDark())
	m.SetSize(80, 20)
	m.SetRows([]cache.OutboxRow{{ID: 4, Draft: nil}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil {
		return
	}
	if _, ok := cmd().(EditAsDraftMsg); ok {
		t.Errorf("e should be inert when Draft is nil")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```
go test ./internal/ui/outbox -v
```

- [ ] **Step 3: Implement messages**

```go
// internal/ui/outbox/msgs.go
package outbox

import "github.com/glw907/poplar/internal/cache"

// CancelMsg requests the App cancel the named outbox op.
type CancelMsg struct{ OpID int64 }

// RescheduleMsg requests the App open the schedule picker pre-filled
// with the row's current time. Initial is "2006-01-02 15:04".
type RescheduleMsg struct {
	OpID    int64
	Initial string
}

// EditAsDraftMsg requests the App cancel the op and open compose
// seeded from the linked draft.
type EditAsDraftMsg struct {
	OpID  int64
	Draft *cache.DraftRow
}

// CloseMsg requests the App return to the previous folder.
type CloseMsg struct{}
```

- [ ] **Step 4: Implement model**

```go
// internal/ui/outbox/model.go
package outbox

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/theme"
)

// Model is a read-only view over scheduled outbox rows.
type Model struct {
	rows   []cache.OutboxRow
	cursor int
	now    time.Time
	width  int
	height int
	styles Styles
}

// New builds the outbox view bound to the given theme.
func New(t *theme.CompiledTheme) Model {
	return Model{styles: NewStyles(t), now: time.Now()}
}

// SetSize updates the render dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetRows replaces the row set. Cursor preserved by op ID; on miss,
// clamps to nearest valid index.
func (m *Model) SetRows(rows []cache.OutboxRow) {
	prevID := int64(-1)
	if m.cursor < len(m.rows) {
		prevID = m.rows[m.cursor].ID
	}
	m.rows = rows
	m.cursor = 0
	for i, r := range rows {
		if r.ID == prevID {
			m.cursor = i
			break
		}
	}
	if m.cursor >= len(m.rows) && len(m.rows) > 0 {
		m.cursor = len(m.rows) - 1
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if len(m.rows) == 0 {
		switch km.Type {
		case tea.KeyEsc:
			return m, func() tea.Msg { return CloseMsg{} }
		}
		if km.Type == tea.KeyRunes && len(km.Runes) == 1 && km.Runes[0] == 'q' {
			return m, func() tea.Msg { return CloseMsg{} }
		}
		return m, nil
	}
	switch {
	case km.Type == tea.KeyEsc:
		return m, func() tea.Msg { return CloseMsg{} }
	case km.Type == tea.KeyRunes && len(km.Runes) == 1:
		switch km.Runes[0] {
		case 'j':
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case 'k':
			if m.cursor > 0 {
				m.cursor--
			}
		case 'q':
			return m, func() tea.Msg { return CloseMsg{} }
		case 'c':
			id := m.rows[m.cursor].ID
			return m, func() tea.Msg { return CancelMsg{OpID: id} }
		case 's':
			r := m.rows[m.cursor]
			return m, func() tea.Msg {
				return RescheduleMsg{OpID: r.ID, Initial: r.ScheduledFor.Format("2006-01-02 15:04")}
			}
		case 'e':
			r := m.rows[m.cursor]
			if r.Draft == nil {
				return m, nil
			}
			return m, func() tea.Msg { return EditAsDraftMsg{OpID: r.ID, Draft: r.Draft} }
		}
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	header := fmt.Sprintf("Outbox (%d)", len(m.rows))
	b.WriteString(m.styles.Header.Render(header) + "\n\n")
	if len(m.rows) == 0 {
		fill := strings.Repeat("\n", m.height/2-2)
		return b.String() + fill + m.styles.Empty.Render("Outbox is empty")
	}
	for i, r := range m.rows {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		row := fmt.Sprintf("%s%-18s  %-22s  %-30s  %s",
			marker,
			formatWhen(r.ScheduledFor, m.now),
			ansix.TruncateEllipsis(firstAddr(r.To), 22),
			ansix.TruncateEllipsis(r.Subject, 30),
			r.Status,
		)
		b.WriteString(row + "\n")
	}
	b.WriteString("\n  c cancel  s reschedule  e edit-as-draft  q close\n")
	return b.String()
}

func firstAddr(to []string) string {
	if len(to) == 0 {
		return ""
	}
	return to[0]
}

// formatWhen renders relative within 24h, absolute beyond.
func formatWhen(t, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := t.Sub(now)
	switch {
	case d < 0:
		return t.Format("Mon Jan 2 3:04 PM")
	case d < 24*time.Hour:
		return t.Format("Today 3:04 PM")
	case d < 48*time.Hour:
		return t.Format("Tomorrow 3:04 PM")
	default:
		return t.Format("Mon Jan 2 3:04 PM")
	}
}
```

- [ ] **Step 5: Implement styles**

```go
// internal/ui/outbox/styles.go
package outbox

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

type Styles struct {
	Header lipgloss.Style
	Row    lipgloss.Style
	Cursor lipgloss.Style
	Empty  lipgloss.Style
}

func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Header: lipgloss.NewStyle().Foreground(t.FgBright).Bold(true),
		Row:    lipgloss.NewStyle().Foreground(t.Fg),
		Cursor: lipgloss.NewStyle().Foreground(t.AccentPrimary),
		Empty:  lipgloss.NewStyle().Foreground(t.FgDim).Align(lipgloss.Center),
	}
}
```

(Adjust palette field names to match `theme.CompiledTheme`'s actual fields — `FgBright`, `Fg`, `FgDim`, `AccentPrimary` are the canonical ones per `internal/theme/`. If different, match the existing.)

- [ ] **Step 6: Run, confirm green**

```
go test ./internal/ui/outbox -v
```

- [ ] **Step 7: Commit**

```bash
git add internal/ui/outbox/
git commit -m "Pass 10b: outbox view subpackage"
```

---

## Task 8: App wiring — sidebar count + Outbox routing

**Files:**
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Locate the relevant call sites**

```
grep -n "OutboxCount\|sidebar.SetFolders\|cache.UpdateMsg\|messagelist\|RenderWithRightPane" internal/ui/app.go
```

Identify (a) where cache events refresh the sidebar, (b) where folder selection picks the right-pane view, (c) where the App receives its `*cache.Account`.

- [ ] **Step 2: Add Outbox model field + restoration tracking**

In `App` struct:

```go
outbox          *outbox.Model // non-nil while user is viewing the outbox
preOutboxFolder string         // canonical name to restore on close
```

Initialize the model in `NewApp` once the theme is in scope:

```go
ob := outbox.New(theme)
a.outbox = &ob
```

(`*Model` field; nil-pointer style toggling: when the user navigates *into* Outbox, set to a non-nil pointer; when they leave, set back to nil. Use a separate "current view" enum if the existing code already has one.)

- [ ] **Step 3: Refresh OutboxCount on cache events**

In the `cache.UpdateMsg` handler (or wherever sidebar refresh runs):

```go
n, _ := a.account.OutboxCount(ctx)  // existing method
a.sidebar.SetOutboxCount(n)
```

If `OutboxCount` doesn't exist, derive from `OutboxDepth`:

```go
d, _ := a.account.OutboxDepth(ctx)
a.sidebar.SetOutboxCount(d.Pending + d.Failed + d.Executing + d.Conflict)
```

When the Outbox view is currently active, also refresh its rows:

```go
if a.outboxActive {
    rows, _ := a.account.OutboxScheduled(ctx)
    a.outbox.SetRows(rows)
}
```

- [ ] **Step 4: Route folder selection to Outbox view**

In the J/K folder-change handler (or wherever a sidebar selection drives the right pane):

```go
if a.sidebar.SelectedCanonical() == "Outbox" {
    if !a.outboxActive {
        a.preOutboxFolder = a.previousCanonical // wherever you track this
    }
    a.outboxActive = true
    rows, _ := a.account.OutboxScheduled(ctx)
    a.outbox.SetRows(rows)
    a.outbox.SetSize(a.rightPaneWidth, a.rightPaneHeight)
    return a, nil
}
a.outboxActive = false
// ... existing folder load via cache.QueryFolder
```

In `View`, when `a.outboxActive`, render the Outbox view in place of the message list. Mirror the AccountTab right-pane composition.

- [ ] **Step 5: Forward keys to outbox + consume its msgs**

While `a.outboxActive`, route key messages to `a.outbox.Update(msg)`. Consume the messages in the App's `Update`:

```go
case outbox.CancelMsg:
    return a, cancelOpCmd(a.account, msg.OpID)
case outbox.RescheduleMsg:
    // open compose's schedule picker pre-filled, and remember the OpID
    a.pendingRescheduleOpID = msg.OpID
    return a, openSchedulePickerCmd(msg.Initial)
case outbox.EditAsDraftMsg:
    return a, editAsDraftCmd(a.account, msg.OpID, msg.Draft)
case outbox.CloseMsg:
    a.outboxActive = false
    a.sidebar.SelectByCanonical(a.preOutboxFolder)
    return a, loadFolderCmd(a.account, a.preOutboxFolder)
```

`cancelOpCmd` calls `cache.Account.CancelOps([opID])`. `editAsDraftCmd` calls `CancelOps` then synthesizes a `composeOpenWithDraftMsg` carrying the decoded `compose.Draft` (use `compose.DecodeDraft(msg.Draft.Payload)`).

For `RescheduleMsg`: open the schedule picker as a top-level overlay. Reuse the same `SchedulePicker` from `internal/ui/compose/`; its `OpenScheduleMsg` shape is the wire. On accept, the App consumes `compose.ScheduleAcceptedMsg` while in this mode and calls `cache.Account.RescheduleOp(pendingRescheduleOpID, when.UnixNano())` instead of routing the message into compose. Use a small flag on App (`schedulePickerOwner`) to disambiguate compose-side vs outbox-side picker accepts.

- [ ] **Step 6: Verify build + existing tests**

```
go build ./...
go test ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/ui/app.go internal/ui/cmds.go
git commit -m "Pass 10b: app routes Outbox selection + cancel/reschedule/edit"
```

---

## Task 9: Keybindings doc + footer hint

**Files:**
- Modify: `docs/poplar/keybindings.md`
- Modify: `internal/ui/compose/model.go` (footer hint addition)

- [ ] **Step 1: Add `Ctrl+L` row to compose section**

In `docs/poplar/keybindings.md` "## Compose" table, append:

```
| `^L`      | Schedule send (opens picker) |
```

- [ ] **Step 2: Add Outbox section**

After "## Cache & Outbox", append:

```markdown
## Outbox view

Active when the synthetic Outbox entry is selected in the sidebar
(visible only when the queue is non-empty).

| Key | Action |
|-----|--------|
| `j` / `k` | Cursor |
| `c` | Cancel scheduled send |
| `s` | Reschedule (opens picker pre-filled) |
| `e` | Edit as draft (cancels + opens compose) |
| `Esc` / `q` | Return to previous folder |
```

- [ ] **Step 3: Add footer hint**

In compose's footer-hint vocabulary (find the slice of hint structs near the existing `^O attach` / `^T tidy` entries), add:

```go
{Label: "^L later", Rank: 6},
```

- [ ] **Step 4: Verify**

```
go build ./...
make check
```

- [ ] **Step 5: Commit**

```bash
git add docs/poplar/keybindings.md internal/ui/compose/model.go
git commit -m "Pass 10b: keybindings.md + footer hint for Ctrl+L"
```

---

## Task 10: Live verification + pass-end ritual

**Files:** none (verification + ritual)

- [ ] **Step 1: tmux capture at 80×24 and 120×40**

Follow `.claude/docs/tmux-testing.md`. Spawn poplar against the
Fastmail account (`$FASTMAIL_API_TOKEN`), compose a draft, hit
`Ctrl+L`, exercise each preset and a custom string. Capture both
sizes. Verify:

- Picker overlays inside compose's frame, not over chrome
- Outbox synthetic entry appears in sidebar after first scheduled send
- `J`/`K` lands on Outbox; the right pane shows the new view
- `c`/`s`/`e` work; the row disappears or reschedules visibly
- Cancel returns the row to drafts (when `e`); cancel removes (when `c`)

- [ ] **Step 2: Run the Pass-end consolidation ritual**

Invoke the `poplar-pass` skill's pass-end checklist:

1. `/simplify` on the diff
2. Idiomatic-bubbletea review against `bubbletea-conventions.md` §10
3. Write **ADR-0184** at `docs/poplar/decisions/0184-schedule-send.md`
   covering: Ctrl+L choice, preset calibration vs Gmail, custom
   text-input deviation from the GUI matrix, "Outbox" naming over
   "Scheduled", synthetic Disposal entry approach, RescheduleOp /
   OutboxScheduled cache surface, edit-as-draft join.
4. Update `docs/poplar/decisions/INDEX.md` with the ADR-0184 row.
5. Update `docs/poplar/invariants.md`:
   - **Send + Append:** add `RescheduleOp` and `OutboxScheduled` to the cache method list
   - **Sidebar (UI invariants):** add the `SetOutboxCount` synthetic-entry rule
   - **Compose (UI invariants):** add the `Ctrl+L` schedule picker
6. Update `STATUS.md`: mark Pass 10b done; replace starter prompt
   with Pass 11 (List-Unsubscribe).
7. Archive plan + spec via `git mv` to
   `docs/superpowers/archive/plans/` and
   `docs/superpowers/archive/specs/`.
8. `make check` — green before commit.
9. Commit, push, `make install`.

- [ ] **Step 3: Verify install**

```bash
which poplar
poplar --version
```

---

## Self-review notes

- Spec coverage: Tasks 1–2 cover cache surface; 3 covers parser; 4–5 cover compose-side picker + send-with-time; 6 covers sidebar seam; 7 covers Outbox view; 8 covers App routing including reschedule/edit-as-draft; 9 covers docs + footer; 10 covers verification + ADR/invariants. Every spec section has a task.
- Type consistency: `cache.OutboxRow.Draft *cache.DraftRow` matches the existing `DraftRow` shape; `cache.OutboxRow.ScheduledFor` is `time.Time` matching the picker's `ScheduleAcceptedMsg.When`; the picker's `Initial` string format `"2006-01-02 15:04"` matches the outbox view's reschedule emit format and the parser's accepted layouts.
- Pass-split watch: 10 numbered tasks, fits the 8–12 budget. If task 8 (App wiring) sprawls past one diff, split into 10b.1 (Tasks 1–7 plus a minimal cancel-only Outbox view) and 10b.2 (Tasks 8–10 with reschedule + edit-as-draft).
