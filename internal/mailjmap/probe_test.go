package mailjmap

import (
	"context"
	"errors"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func withProbeAuth(t *testing.T, fn probeAuthFn) {
	t.Helper()
	prev := probeAuth
	probeAuth = fn
	t.Cleanup(func() { probeAuth = prev })
}

func TestProbe_HappyPath(t *testing.T) {
	cli := &fakeClient{
		respond: func(*jmap.Request) (*jmap.Response, error) {
			return fakeResponse(&jmap.Invocation{
				Args: &mailbox.GetResponse{List: []*mailbox.Mailbox{
					{ID: "a"}, {ID: "b"}, {ID: "c"},
				}},
			}), nil
		},
	}
	session := &jmap.Session{
		PrimaryAccounts: map[jmap.URI]jmap.ID{jmapmail.URI: "acct"},
	}
	withProbeAuth(t, func(context.Context, config.AccountConfig) (jmapClient, *jmap.Session, error) {
		return cli, session, nil
	})

	cfg := config.AccountConfig{
		Name: "fm", Backend: "jmap",
		Source: "https://api.fastmail.com/jmap/session",
	}
	r := Probe(context.Background(), cfg)

	wantLabels := []string{"Resolving session URL", "Authenticate", "mailbox/get"}
	if len(r.Steps) != len(wantLabels) {
		t.Fatalf("len(Steps) = %d, want %d (%+v)", len(r.Steps), len(wantLabels), r.Steps)
	}
	for i, want := range wantLabels {
		if r.Steps[i].Label != want {
			t.Errorf("Steps[%d].Label = %q, want %q", i, r.Steps[i].Label, want)
		}
		if r.Steps[i].Status != mail.ProbeOK {
			t.Errorf("Steps[%d].Status = %v, want ProbeOK", i, r.Steps[i].Status)
		}
	}
	if got := r.Steps[2].Detail; got != "3 mailboxes" {
		t.Errorf("mailbox/get detail = %q, want %q", got, "3 mailboxes")
	}
	if !r.OK() {
		t.Errorf("OK() = false, want true")
	}
}

func TestProbe_EmptySessionURLFails(t *testing.T) {
	r := Probe(context.Background(), config.AccountConfig{})
	if len(r.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(r.Steps))
	}
	if r.Steps[0].Status != mail.ProbeFail {
		t.Errorf("Steps[0].Status = %v, want ProbeFail", r.Steps[0].Status)
	}
	if r.Err == nil {
		t.Fatal("Err = nil, want non-nil")
	}
}

func TestProbe_AuthFailureStopsTranscript(t *testing.T) {
	withProbeAuth(t, func(context.Context, config.AccountConfig) (jmapClient, *jmap.Session, error) {
		return nil, nil, errors.New("HTTP 401: unauthorized")
	})

	cfg := config.AccountConfig{Source: "https://example.com/jmap/session"}
	r := Probe(context.Background(), cfg)
	if len(r.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2 (%+v)", len(r.Steps), r.Steps)
	}
	if r.Steps[0].Status != mail.ProbeOK {
		t.Errorf("Steps[0].Status = %v, want ProbeOK", r.Steps[0].Status)
	}
	if r.Steps[1].Status != mail.ProbeFail {
		t.Errorf("Steps[1].Status = %v, want ProbeFail", r.Steps[1].Status)
	}
	if r.Err == nil {
		t.Fatal("Err = nil, want non-nil")
	}
}

func TestProbe_MailboxGetFailureCaptured(t *testing.T) {
	cli := &fakeClient{
		respond: func(*jmap.Request) (*jmap.Response, error) {
			return nil, errors.New("HTTP 500")
		},
	}
	session := &jmap.Session{
		PrimaryAccounts: map[jmap.URI]jmap.ID{jmapmail.URI: "acct"},
	}
	withProbeAuth(t, func(context.Context, config.AccountConfig) (jmapClient, *jmap.Session, error) {
		return cli, session, nil
	})

	cfg := config.AccountConfig{Source: "https://example.com/jmap/session"}
	r := Probe(context.Background(), cfg)
	if len(r.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3 (%+v)", len(r.Steps), r.Steps)
	}
	if r.Steps[2].Status != mail.ProbeFail {
		t.Errorf("Steps[2].Status = %v, want ProbeFail", r.Steps[2].Status)
	}
}
