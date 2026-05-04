// SPDX-License-Identifier: MIT

package mailimap

import "testing"

func TestLooksSelfHosted(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"192.168.1.10", true},
		{"10.0.0.5", true},
		{"172.16.4.7", true},
		{"127.0.0.1", true},
		{"mail.local", true},
		{"imap.fastmail.com", false},
		{"outlook.office365.com", false},
		{"8.8.8.8", false},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			if got := looksSelfHosted(tc.host); got != tc.want {
				t.Errorf("looksSelfHosted(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}
