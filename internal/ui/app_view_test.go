package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

type testProgressTriple struct {
	attach, outbox, sync bool
	outboxPct            int
}

func (m *App) testProgressSources(attach, outbox bool, outboxPct int, sync bool) {
	m.testProgressOverride = &testProgressTriple{
		attach:    attach,
		outbox:    outbox,
		outboxPct: outboxPct,
		sync:      sync,
	}
}

func TestFrameProgressBarPriorityLadder(t *testing.T) {
	cases := []struct {
		name      string
		attach    bool
		outbox    bool
		outboxPct int
		sync      bool
		wantState tea.ProgressBarState
		wantValue int
	}{
		{"all idle", false, false, 0, false, tea.ProgressBarNone, 0},
		{"sync only", false, false, 0, true, tea.ProgressBarIndeterminate, 0},
		{"outbox only", false, true, 42, false, tea.ProgressBarDefault, 42},
		{"attach only", true, false, 0, false, tea.ProgressBarIndeterminate, 0},
		{"attach beats outbox", true, true, 90, true, tea.ProgressBarIndeterminate, 0},
		{"outbox beats sync", false, true, 50, true, tea.ProgressBarDefault, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := App{}
			m.testProgressSources(tc.attach, tc.outbox, tc.outboxPct, tc.sync)
			pb := m.frameProgressBar()
			if tc.wantState == tea.ProgressBarNone {
				if pb != nil {
					t.Fatalf("got %+v, want nil", pb)
				}
				return
			}
			if pb == nil {
				t.Fatalf("got nil, want state=%v", tc.wantState)
			}
			if pb.State != tc.wantState {
				t.Fatalf("state = %v, want %v", pb.State, tc.wantState)
			}
			if pb.State == tea.ProgressBarDefault && pb.Value != tc.wantValue {
				t.Fatalf("value = %d, want %d", pb.Value, tc.wantValue)
			}
		})
	}
}

func TestFrameProgressBarErrorDecay(t *testing.T) {
	frozen := time.Unix(1_700_000_000, 0)
	m := App{
		now:                func() time.Time { return frozen },
		progressErrorUntil: frozen.Add(2 * time.Second),
	}
	pb := m.frameProgressBar()
	if pb == nil || pb.State != tea.ProgressBarError {
		t.Fatalf("got %+v, want state=Error", pb)
	}
	m.progressErrorUntil = frozen.Add(-time.Second)
	if pb := m.frameProgressBar(); pb != nil {
		t.Fatalf("expired: got %+v, want nil", pb)
	}
}
