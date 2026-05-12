package mailauth

import (
	"strings"
	"testing"
)

func TestXOAuth2_Challenge(t *testing.T) {
	c := NewXoauth2Client("alice@example.com", "FAKE_TOKEN")

	mech, ir, err := c.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if mech != "XOAUTH2" {
		t.Errorf("mech = %q, want %q", mech, "XOAUTH2")
	}
	want := "user=alice@example.com\x01auth=Bearer FAKE_TOKEN\x01\x01"
	if string(ir) != want {
		t.Errorf("ir = %q, want %q", string(ir), want)
	}
}

func TestXOAuth2_NextDecodesError(t *testing.T) {
	c := NewXoauth2Client("alice@example.com", "FAKE")
	challenge := []byte(`{"status":"401","schemes":"Bearer","scope":"mail"}`)

	resp, err := c.Next(challenge)
	if resp != nil {
		t.Errorf("resp = %v, want nil", resp)
	}
	xerr, ok := err.(*Xoauth2Error)
	if !ok {
		t.Fatalf("err type = %T, want *Xoauth2Error", err)
	}
	if xerr.Status != "401" {
		t.Errorf("Status = %q, want 401", xerr.Status)
	}
	if !strings.Contains(xerr.Error(), "401") {
		t.Errorf("Error() = %q, want it to mention status", xerr.Error())
	}
}

func TestXOAuth2_NextRejectsBadJSON(t *testing.T) {
	c := NewXoauth2Client("alice@example.com", "FAKE")
	resp, err := c.Next([]byte("not-json"))
	if resp != nil {
		t.Errorf("resp = %v, want nil", resp)
	}
	if err == nil {
		t.Fatal("err = nil, want unmarshal error")
	}
	if _, isX := err.(*Xoauth2Error); isX {
		t.Errorf("err is *Xoauth2Error, want unmarshal error")
	}
}
