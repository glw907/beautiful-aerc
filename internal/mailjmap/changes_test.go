package mailjmap

import (
	"context"
	"errors"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"

	"github.com/glw907/poplar/internal/mail"
)

// Changes(since=nil) routes through the baseline-pull path: page
// Email/query, then Email/get with empty IDs to capture state.

func TestChanges_BaselinePull_SinglePage(t *testing.T) {
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			if got, want := len(req.Calls), 2; got != want {
				t.Fatalf("calls = %d, want %d (query + state probe)", got, want)
			}
			if _, ok := req.Calls[0].Args.(*email.Query); !ok {
				t.Fatalf("call[0] args = %T, want *email.Query", req.Calls[0].Args)
			}
			g, ok := req.Calls[1].Args.(*email.Get)
			if !ok {
				t.Fatalf("call[1] args = %T, want *email.Get", req.Calls[1].Args)
			}
			if len(g.IDs) != 1 || g.IDs[0] != stateProbeID {
				t.Errorf("state probe IDs = %v, want [%q]", g.IDs, stateProbeID)
			}
			return fakeResponse(
				&jmap.Invocation{
					Name: "Email/query",
					Args: &email.QueryResponse{
						IDs:   []jmap.ID{"e-1", "e-2", "e-3"},
						Total: 3,
					},
				},
				&jmap.Invocation{
					Name: "Email/get",
					Args: &email.GetResponse{State: "state-abc"},
				},
			), nil
		},
	}
	folders := map[string]folderEntry{
		"Inbox": {id: "mb-1", folder: mail.Folder{Name: "Inbox"}},
	}
	b := newTestBackend(fake, "acct-1", folders)

	delta, token, err := b.Changes(context.Background(), "Inbox", nil)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if got, want := len(delta.Added), 3; got != want {
		t.Errorf("Added len = %d, want %d", got, want)
	}
	if string(token) != "state-abc" {
		t.Errorf("token = %q, want %q", string(token), "state-abc")
	}
}

func TestChanges_BaselinePull_MultiPage(t *testing.T) {
	page := 0
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			q := req.Calls[0].Args.(*email.Query)
			page++
			switch page {
			case 1:
				if q.Position != 0 {
					t.Errorf("page 1 Position = %d, want 0", q.Position)
				}
				ids := make([]jmap.ID, baselinePullPageSize)
				for i := range ids {
					ids[i] = jmap.ID("a")
				}
				return fakeResponse(
					&jmap.Invocation{Name: "Email/query", Args: &email.QueryResponse{IDs: ids, Total: baselinePullPageSize + 2}},
					&jmap.Invocation{Name: "Email/get", Args: &email.GetResponse{State: "state-mid"}},
				), nil
			case 2:
				if q.Position != int64(baselinePullPageSize) {
					t.Errorf("page 2 Position = %d, want %d", q.Position, baselinePullPageSize)
				}
				return fakeResponse(
					&jmap.Invocation{Name: "Email/query", Args: &email.QueryResponse{IDs: []jmap.ID{"x", "y"}, Total: baselinePullPageSize + 2}},
					&jmap.Invocation{Name: "Email/get", Args: &email.GetResponse{State: "state-final"}},
				), nil
			}
			t.Fatalf("unexpected page %d", page)
			return nil, nil
		},
	}
	folders := map[string]folderEntry{
		"Inbox": {id: "mb-1", folder: mail.Folder{Name: "Inbox"}},
	}
	b := newTestBackend(fake, "acct-1", folders)

	delta, token, err := b.Changes(context.Background(), "Inbox", nil)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if got, want := len(delta.Added), baselinePullPageSize+2; got != want {
		t.Errorf("Added len = %d, want %d", got, want)
	}
	if string(token) != "state-final" {
		t.Errorf("token = %q, want %q", string(token), "state-final")
	}
}

func TestChanges_BaselinePull_EmptyMailbox(t *testing.T) {
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			return fakeResponse(
				&jmap.Invocation{Name: "Email/query", Args: &email.QueryResponse{IDs: nil, Total: 0}},
				&jmap.Invocation{Name: "Email/get", Args: &email.GetResponse{State: "state-empty"}},
			), nil
		},
	}
	folders := map[string]folderEntry{
		"Inbox": {id: "mb-1", folder: mail.Folder{Name: "Inbox"}},
	}
	b := newTestBackend(fake, "acct-1", folders)

	delta, token, err := b.Changes(context.Background(), "Inbox", nil)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if len(delta.Added) != 0 {
		t.Errorf("Added len = %d, want 0", len(delta.Added))
	}
	if string(token) != "state-empty" {
		t.Errorf("token = %q, want %q", string(token), "state-empty")
	}
}

func TestChanges_BaselinePull_UnknownFolder(t *testing.T) {
	b := newTestBackend(&fakeClient{}, "acct-1", nil)
	_, _, err := b.Changes(context.Background(), "Nope", nil)
	if err == nil {
		t.Fatal("expected error for unknown folder")
	}
}

func TestChanges_Incremental_StillUsesEmailChanges(t *testing.T) {
	var capturedName string
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedName = req.Calls[0].Args.(interface{ Name() string }).Name()
			return fakeResponse(&jmap.Invocation{
				Name: "Email/changes",
				Args: &email.ChangesResponse{
					NewState:       "state-2",
					Created:        []jmap.ID{"new-1"},
					HasMoreChanges: false,
				},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-1", map[string]folderEntry{
		"Inbox": {id: "mb-1"},
	})

	delta, token, err := b.Changes(context.Background(), "Inbox", mail.SyncToken("state-1"))
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if capturedName != "Email/changes" {
		t.Errorf("invoked %q, want Email/changes", capturedName)
	}
	if len(delta.Added) != 1 || delta.Added[0] != "new-1" {
		t.Errorf("Added = %v, want [new-1]", delta.Added)
	}
	if string(token) != "state-2" {
		t.Errorf("token = %q, want state-2", string(token))
	}
}

func TestChanges_BaselinePull_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := newTestBackend(&fakeClient{}, "acct-1", map[string]folderEntry{
		"Inbox": {id: "mb-1"},
	})
	_, _, err := b.Changes(ctx, "Inbox", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
