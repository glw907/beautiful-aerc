package sync

import (
	"context"
	"fmt"
	"maps"
	"math/rand/v2"
	"testing"
	"testing/synctest"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestConvergence runs QA-4's trials: repeated sync cycles against a
// scripted server that creates, updates, and destroys records at
// random and, once per trial, resets its state token mid-stream. It
// asserts the local store always converges to exactly the server's
// modeled final state, reset included, across every trial.
func TestConvergence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const trials = 20
		for trial := range trials {
			runConvergenceTrial(t, int64(trial))
		}
	})
}

func runConvergenceTrial(t *testing.T, seed int64) {
	t.Helper()

	rng := rand.New(rand.NewPCG(uint64(seed), 0)) //nolint:gosec // G404: trial scripting, not a security decision
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	model := map[string]string{}
	// resetAt names the call count, among the calls carrying a
	// non-empty token, that reports a state reset; the trial's first
	// call always carries an empty token (the initial baseline), so
	// resetAt starts past it.
	resetAt := 2 + rng.IntN(3)

	calls := 0
	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(_ context.Context, _ backend.ObjectKind, token string, _ int) (backend.ChangeSet, error) {
		calls++
		if token == "" {
			cs := backend.ChangeSet{NewToken: fmt.Sprintf("tok-%d", calls)}
			for id, subject := range model {
				cs.Created = append(cs.Created, backend.Record{ID: id, Fields: backend.MessageFields{Subject: subject}})
			}
			return cs, nil
		}
		if calls == resetAt {
			return backend.ChangeSet{}, backend.ErrStateReset
		}

		id := fmt.Sprintf("m%d", rng.IntN(5))
		cs := backend.ChangeSet{NewToken: fmt.Sprintf("tok-%d", calls)}
		switch {
		case model[id] == "" || rng.IntN(3) == 0:
			subject := fmt.Sprintf("created-%d-%d", seed, calls)
			model[id] = subject
			cs.Created = []backend.Record{{ID: id, Fields: backend.MessageFields{Subject: subject}}}
		case rng.IntN(2) == 0:
			subject := fmt.Sprintf("updated-%d-%d", seed, calls)
			model[id] = subject
			cs.Updated = []backend.Record{{ID: id, Fields: backend.MessageFields{Subject: subject}}}
		default:
			delete(model, id)
			cs.Destroyed = []string{id}
		}
		return cs, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	const cycles = 6
	for range cycles {
		if err := worker.SyncKind(context.Background(), backend.ObjectKindMessage); err != nil {
			t.Fatalf("trial %d: SyncKind: %v", seed, err)
		}
	}

	if got := storeModel(t, w, accountID); !maps.Equal(got, model) {
		t.Fatalf("trial %d: store state = %v, want %v", seed, got, model)
	}
}
