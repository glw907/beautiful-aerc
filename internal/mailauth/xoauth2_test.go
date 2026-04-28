// SPDX-License-Identifier: MIT

package mailauth

import "testing"

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
