// SPDX-License-Identifier: MIT

package config

import "testing"

func TestSuggestProvider(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"yahho", "yahoo"},
		{"fastmial", "fastmail"},
		{"icoud", "icloud"},
		{"protonmial", "protonmail"},
		{"qwertz", ""},
		{"yahoo", "yahoo"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := suggestProvider(tc.input)
			if got != tc.want {
				t.Errorf("suggestProvider(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
