package mailimap

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
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
