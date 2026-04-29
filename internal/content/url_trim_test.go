// SPDX-License-Identifier: MIT

package content

import "testing"

func TestTrimURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"host only", "https://example.com", "example.com"},
		{"host trailing slash", "https://example.com/", "example.com/"},
		{"single segment", "https://example.com/foo", "example.com/foo"},
		{"single segment trailing slash", "https://example.com/foo/", "example.com/foo/"},
		{"two segments", "https://example.com/foo/bar", "example.com/foo…"},
		{"segment plus query", "https://example.com/foo?q=1", "example.com/foo…"},
		{"segment plus fragment", "https://example.com/foo#frag", "example.com/foo…"},
		{"deep path with query and fragment", "https://example.com/a/b/c?x=1#frag", "example.com/a…"},
		{"http scheme", "http://example.com/foo/bar", "example.com/foo…"},
		{"mailto", "mailto:foo@example.com", "foo@example.com"},
		{"port preserved", "https://example.com:8080/foo/bar", "example.com:8080/foo…"},
		{"empty", "", ""},
		{
			// Google/Facebook tracking URLs: one giant opaque path segment
			// with no further separators. trimURL must cap the segment so
			// the inline form stays short.
			name: "single oversized opaque segment capped",
			in:   "https://c.gle/ADMV5J_kXq_U31YeshvtCqkmvPXvdYEr43xYb5dWsh1LqnSuUcpOZjaxn7HL0ItZG",
			want: "c.gle/ADMV5J_kXq_U31Ye…",
		},
		{
			"oversized first segment with later path capped",
			"https://example.com/AVeryLongOpaqueTokenSegment/next",
			"example.com/AVeryLongOpaqueT…",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := trimURL(tc.in)
			if got != tc.want {
				t.Fatalf("trimURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
