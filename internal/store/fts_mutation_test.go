package store

import (
	"context"
	"database/sql"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// messageRecord is TestMutationSearchConsistency's in-memory model of
// one message: the reference this test checks message_fts against
// after every mutation, since the model never runs through FTS5 at
// all.
type messageRecord struct {
	subject, searchText string
	mailbox             int64
	alive               bool
}

// TestMutationSearchConsistency drives a seeded, randomized sequence
// of message inserts, updates, deletes, and mailbox moves through the
// writer, asserting after every step that message_fts's search
// results match an independent in-memory model of what should be
// indexed. Moves touch only message_mailbox and carry no FTS
// maintenance of their own; they are in the mix to prove an unrelated
// write never disturbs the index. The script closes with FTS5's own
// 'integrity-check' command: row-count parity between message and
// message_fts can hold while individual terms have rotted, which no
// per-step search assertion here would catch.
func TestMutationSearchConsistency(t *testing.T) {
	w, _ := newTestWriter(t, DefaultWriterConfig())
	seedAccountAndMailbox(t, w)
	if err := w.submit(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO mailbox (id, account_id, name) VALUES (2, 1, 'Archive')`)
		return err
	}); err != nil {
		t.Fatalf("seed second mailbox: %v", err)
	}

	vocab := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"}
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: a fixed seed makes this test's randomized sequence reproducible, not a security-sensitive use
	randomText := func(words int) string {
		picked := make([]string, words)
		for i := range picked {
			picked[i] = vocab[rng.IntN(len(vocab))]
		}
		return strings.Join(picked, " ")
	}

	model := map[int64]*messageRecord{}
	var nextID int64 = 1

	const steps = 300
	for step := range steps {
		ids := aliveMessageIDs(model)

		op := 0
		if len(ids) > 0 {
			op = rng.IntN(4)
		}
		switch op {
		case 0:
			id := nextID
			nextID++
			subject, searchText := randomText(2), randomText(3)
			mailbox := int64(1 + rng.IntN(2))
			if err := w.submit(context.Background(), func(tx *sql.Tx) error {
				if err := reindexMessage(tx, id, subject, searchText); err != nil {
					return err
				}
				if _, err := tx.Exec(`INSERT INTO message (id, account_id, received_at, subject, search_text) VALUES (?, 1, ?, ?, ?)`,
					id, id, subject, searchText); err != nil {
					return err
				}
				_, err := tx.Exec(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at) VALUES (?, ?, ?)`, id, mailbox, id)
				return err
			}); err != nil {
				t.Fatalf("step %d: insert message %d: %v", step, id, err)
			}
			model[id] = &messageRecord{subject: subject, searchText: searchText, mailbox: mailbox, alive: true}

		case 1:
			id := ids[rng.IntN(len(ids))]
			subject, searchText := randomText(2), randomText(3)
			if err := w.submit(context.Background(), func(tx *sql.Tx) error {
				if err := reindexMessage(tx, id, subject, searchText); err != nil {
					return err
				}
				_, err := tx.Exec(`UPDATE message SET subject = ?, search_text = ? WHERE id = ?`, subject, searchText, id)
				return err
			}); err != nil {
				t.Fatalf("step %d: update message %d: %v", step, id, err)
			}
			model[id].subject, model[id].searchText = subject, searchText

		case 2:
			id := ids[rng.IntN(len(ids))]
			if err := w.submit(context.Background(), func(tx *sql.Tx) error {
				_, err := tx.Exec(`DELETE FROM message WHERE id = ?`, id)
				return err
			}); err != nil {
				t.Fatalf("step %d: delete message %d: %v", step, id, err)
			}
			model[id].alive = false

		default:
			id := ids[rng.IntN(len(ids))]
			to := int64(2)
			if model[id].mailbox == 2 {
				to = 1
			}
			if err := w.submit(context.Background(), func(tx *sql.Tx) error {
				if _, err := tx.Exec(`DELETE FROM message_mailbox WHERE message_id = ?`, id); err != nil {
					return err
				}
				_, err := tx.Exec(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at) VALUES (?, ?, ?)`, id, to, id)
				return err
			}); err != nil {
				t.Fatalf("step %d: move message %d: %v", step, id, err)
			}
			model[id].mailbox = to
		}

		term := vocab[rng.IntN(len(vocab))]
		want := expectedSearchMatches(model, term)
		got := searchIDs(t, w.db, term)
		if !slices.Equal(got, want) {
			t.Fatalf("step %d: search(%q) = %v, want %v", step, term, got, want)
		}
	}

	if _, err := w.db.Exec(`INSERT INTO message_fts(message_fts) VALUES ('integrity-check')`); err != nil {
		t.Fatalf("integrity-check: %v", err)
	}
}

// aliveMessageIDs returns model's alive message ids in ascending
// order, so a caller indexing into it with a seeded rng gets the same
// pick across runs regardless of map iteration order.
func aliveMessageIDs(model map[int64]*messageRecord) []int64 {
	var ids []int64
	for id, rec := range model {
		if rec.alive {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// expectedSearchMatches returns the ids of model's alive messages
// whose subject or search_text carries term as a whole word, the
// same match FTS5's default unicode61 tokenizer gives a plain
// (non-prefix) MATCH query over this test's lowercase, single-word
// vocabulary.
func expectedSearchMatches(model map[int64]*messageRecord, term string) []int64 {
	var ids []int64
	for id, rec := range model {
		if !rec.alive {
			continue
		}
		if slices.Contains(strings.Fields(rec.subject), term) || slices.Contains(strings.Fields(rec.searchText), term) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}
