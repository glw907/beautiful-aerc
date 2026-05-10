package mailimap

import (
	"context"
	"errors"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func withProbeDial(t *testing.T, fn probeDialFn) {
	t.Helper()
	prev := probeDial
	probeDial = fn
	t.Cleanup(func() { probeDial = prev })
}

func okDial(cli imapClient) probeDialFn {
	return func(cfg config.AccountConfig) (imapClient, []mail.ProbeStep, error) {
		return cli, []mail.ProbeStep{
			{Label: "Connecting", Status: mail.ProbeOK, Detail: "imap.example:993"},
			{Label: "TLS handshake", Status: mail.ProbeOK},
			{Label: "AUTHENTICATE", Status: mail.ProbeOK, Detail: "plain"},
		}, nil
	}
}

func TestProbe_HappyPath(t *testing.T) {
	cli := newFakeClient()
	cli.caps["UIDPLUS"] = true
	cli.folderSummary["INBOX"] = mail.Folder{Name: "INBOX", Exists: 1247}
	withProbeDial(t, okDial(cli))

	r := Probe(context.Background(), config.AccountConfig{
		Name: "t", Backend: "imap", Email: "u@x", Host: "imap.example", Port: 993,
	})

	wantLabels := []string{"Connecting", "TLS handshake", "AUTHENTICATE", "CAPABILITY (UIDPLUS)", "STATUS INBOX"}
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
	if got := r.Steps[4].Detail; got != "1247 messages" {
		t.Errorf("STATUS INBOX detail = %q, want %q", got, "1247 messages")
	}
	if r.Err != nil {
		t.Errorf("Err = %v, want nil", r.Err)
	}
	if !r.OK() {
		t.Errorf("OK() = false, want true")
	}
}

func TestProbe_DialFailureStopsTranscript(t *testing.T) {
	withProbeDial(t, func(cfg config.AccountConfig) (imapClient, []mail.ProbeStep, error) {
		return nil, []mail.ProbeStep{
			{Label: "Connecting", Status: mail.ProbeFail, Detail: "i/o timeout"},
		}, errors.New("dial: i/o timeout")
	})

	r := Probe(context.Background(), config.AccountConfig{Host: "x", Port: 993})
	if len(r.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1 (%+v)", len(r.Steps), r.Steps)
	}
	if r.Err == nil {
		t.Fatal("Err = nil, want non-nil")
	}
	if r.OK() {
		t.Error("OK() = true, want false")
	}
}

func TestProbe_MissingUIDPLUSFails(t *testing.T) {
	cli := newFakeClient() // caps map empty
	withProbeDial(t, okDial(cli))

	r := Probe(context.Background(), config.AccountConfig{Host: "x", Port: 993})
	if len(r.Steps) != 4 {
		t.Fatalf("len(Steps) = %d, want 4 (%+v)", len(r.Steps), r.Steps)
	}
	if r.Steps[3].Status != mail.ProbeFail {
		t.Errorf("CAPABILITY step status = %v, want ProbeFail", r.Steps[3].Status)
	}
	if r.Err == nil {
		t.Fatal("Err = nil, want non-nil")
	}
}
