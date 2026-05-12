package mailimap

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailauth"
)

type fakeSMTP struct {
	sendErr  error
	sendCnt  atomic.Int32
	closeCnt atomic.Int32
	from     string
	to       []string
	body     []byte
}

func (f *fakeSMTP) SendMail(from string, to []string, body []byte) error {
	f.sendCnt.Add(1)
	f.from = from
	f.to = append([]string(nil), to...)
	f.body = append([]byte(nil), body...)
	return f.sendErr
}

func (f *fakeSMTP) Close() error {
	f.closeCnt.Add(1)
	return nil
}

func TestSend_LazyDial(t *testing.T) {
	dials := atomic.Int32{}
	fake := &fakeSMTP{}
	prev := smtpDial
	smtpDial = func(_ *Backend) (smtpClient, error) {
		dials.Add(1)
		return fake, nil
	}
	t.Cleanup(func() { smtpDial = prev })

	b := New(config.AccountConfig{Name: "t"}, nil)
	if err := b.Send(mail.Envelope{From: "f@x", Rcpts: []string{"t@x"}}, []byte("hi")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := b.Send(mail.Envelope{From: "f@x", Rcpts: []string{"u@x"}}, []byte("hi2")); err != nil {
		t.Fatalf("Send 2: %v", err)
	}
	if got := dials.Load(); got != 1 {
		t.Errorf("dials = %d, want 1 (lazy + cached)", got)
	}
	if got := fake.sendCnt.Load(); got != 2 {
		t.Errorf("send count = %d, want 2", got)
	}
}

func TestSend_DropsClientOnError(t *testing.T) {
	dials := atomic.Int32{}
	prev := smtpDial
	smtpDial = func(_ *Backend) (smtpClient, error) {
		dials.Add(1)
		return &fakeSMTP{sendErr: errors.New("boom")}, nil
	}
	t.Cleanup(func() { smtpDial = prev })

	b := New(config.AccountConfig{Name: "t"}, nil)
	if err := b.Send(mail.Envelope{From: "f@x", Rcpts: []string{"t@x"}}, []byte("hi")); err == nil {
		t.Fatal("expected error")
	}
	if err := b.Send(mail.Envelope{From: "f@x", Rcpts: []string{"t@x"}}, []byte("hi")); err == nil {
		t.Fatal("expected error 2")
	}
	if got := dials.Load(); got != 2 {
		t.Errorf("dials = %d, want 2 (error drops cache, redials)", got)
	}
}

// fakeAuther implements smtpAuther. authErrs is consumed in order;
// each Auth call advances one slot.
type fakeAuther struct {
	authErrs []error
	calls    atomic.Int32
}

func (f *fakeAuther) Auth(_ sasl.Client) error {
	i := f.calls.Add(1) - 1
	if int(i) >= len(f.authErrs) {
		return nil
	}
	return f.authErrs[i]
}

// TestSmtpAuth_StaleTokenRetry verifies that an OAuth-backed
// smtpAuth retries once with a refreshed token when the first Auth
// call returns SMTP 535.
func TestSmtpAuth_StaleTokenRetry(t *testing.T) {
	tokenURL, reqs := newFakeTokenSrv(t)
	store := newMapStore("acct", "refresh-1")
	c := mailauth.NewClient(
		mailauth.Config{ClientID: "id", TokenURL: tokenURL},
		store, "acct", mailauth.BackendKeyring,
	)
	b := &Backend{
		cfg:   config.AccountConfig{Name: "acct", Email: "u@x", SMTP: config.SMTPConfig{Auth: "xoauth2"}},
		oauth: c,
	}

	au := &fakeAuther{authErrs: []error{
		&gosmtp.SMTPError{Code: 535, Message: "auth failed"},
	}}
	if err := smtpAuth(context.Background(), au, b); err != nil {
		t.Fatalf("smtpAuth: %v", err)
	}
	if got := au.calls.Load(); got != 2 {
		t.Errorf("Auth calls = %d, want 2 (initial + retry)", got)
	}
	if got := reqs.Load(); got < 2 {
		t.Errorf("token refreshes = %d, want >= 2 (Token + ForceRefresh)", got)
	}
}

// TestSmtpAuth_RetrySurfacesPersistentErrAuth verifies that when the
// retry also fails, smtpAuth returns the classified ErrAuth.
func TestSmtpAuth_RetrySurfacesPersistentErrAuth(t *testing.T) {
	tokenURL, _ := newFakeTokenSrv(t)
	store := newMapStore("acct", "refresh-1")
	c := mailauth.NewClient(
		mailauth.Config{ClientID: "id", TokenURL: tokenURL},
		store, "acct", mailauth.BackendKeyring,
	)
	b := &Backend{
		cfg:   config.AccountConfig{Name: "acct", Email: "u@x", SMTP: config.SMTPConfig{Auth: "xoauth2"}},
		oauth: c,
	}

	au := &fakeAuther{authErrs: []error{
		&gosmtp.SMTPError{Code: 535, Message: "auth failed"},
		&gosmtp.SMTPError{Code: 535, Message: "auth failed"},
	}}
	err := smtpAuth(context.Background(), au, b)
	if err == nil {
		t.Fatal("expected error after retry")
	}
	if !errors.Is(err, mail.ErrAuth) {
		t.Errorf("err = %v, want mail.ErrAuth", err)
	}
}

// TestSmtpAuth_NoOAuthNoRetry verifies that without an OAuth client,
// a 535 surfaces immediately as classified ErrAuth without a retry.
func TestSmtpAuth_NoOAuthNoRetry(t *testing.T) {
	b := &Backend{
		cfg: config.AccountConfig{
			Name:  "acct",
			Email: "u@x",
			SMTP: config.SMTPConfig{
				Auth:     "xoauth2",
				Password: "static-token",
			},
		},
	}
	au := &fakeAuther{authErrs: []error{
		&gosmtp.SMTPError{Code: 535, Message: "auth failed"},
	}}
	err := smtpAuth(context.Background(), au, b)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, mail.ErrAuth) {
		t.Errorf("err = %v, want mail.ErrAuth", err)
	}
	if got := au.calls.Load(); got != 1 {
		t.Errorf("Auth calls = %d, want 1 (no retry without OAuth)", got)
	}
}

// TestClassifyErr_SMTPAuthCodes covers the SMTP-error mapping added
// for the SMTP retry path.
func TestClassifyErr_SMTPAuthCodes(t *testing.T) {
	for _, code := range []int{530, 535, 538} {
		err := classifyErr(&gosmtp.SMTPError{Code: code, Message: "x"})
		if !errors.Is(err, mail.ErrAuth) {
			t.Errorf("code %d: errors.Is(_, mail.ErrAuth) = false", code)
		}
	}
	other := classifyErr(&gosmtp.SMTPError{Code: 421, Message: "shutting down"})
	if errors.Is(other, mail.ErrAuth) {
		t.Errorf("code 421 unexpectedly classified as ErrAuth")
	}
}

func TestAppend_GoesThroughCmdConnection(t *testing.T) {
	cmd := newFakeClient()
	b := New(config.AccountConfig{Name: "t"}, nil)
	b.cmd = cmd

	if err := b.Append("Sent", []byte("rfc822 bytes"), mail.FlagSeen); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if len(cmd.appendCalls) != 1 {
		t.Fatalf("appendCalls = %d, want 1", len(cmd.appendCalls))
	}
	c := cmd.appendCalls[0]
	if c.folder != "Sent" || string(c.mime) != "rfc822 bytes" || len(c.flags) != 1 || c.flags[0] != `\Seen` {
		t.Errorf("unexpected append call: %+v", c)
	}
}
