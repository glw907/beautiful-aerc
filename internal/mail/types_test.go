// SPDX-License-Identifier: MIT

package mail

import "testing"

func TestDispositionString(t *testing.T) {
	cases := []struct {
		d    Disposition
		want string
	}{
		{DispAttachment, "attachment"},
		{DispInline, "inline"},
	}
	for _, c := range cases {
		if got := c.d.String(); got != c.want {
			t.Errorf("Disposition(%d).String() = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestParseDisposition(t *testing.T) {
	cases := []struct {
		in      string
		want    Disposition
		wantErr bool
	}{
		{"attachment", DispAttachment, false},
		{"inline", DispInline, false},
		{"ATTACHMENT", DispAttachment, false},
		{"", 0, true},
		{"bogus", 0, true},
	}
	for _, c := range cases {
		got, err := ParseDisposition(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("ParseDisposition(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseDisposition(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
