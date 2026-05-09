package content

import (
	"net/textproto"
	"testing"
)

func TestParseListUnsubscribe(t *testing.T) {
	cases := []struct {
		name string
		hdrs map[string][]string
		want Unsubscribe
	}{
		{
			name: "absent",
			hdrs: map[string][]string{},
			want: Unsubscribe{},
		},
		{
			name: "mailto only",
			hdrs: map[string][]string{
				"List-Unsubscribe": {"<mailto:unsub@list.example>"},
			},
			want: Unsubscribe{Mailto: "mailto:unsub@list.example"},
		},
		{
			name: "https without post header",
			hdrs: map[string][]string{
				"List-Unsubscribe": {"<https://list.example/unsub?id=42>"},
			},
			want: Unsubscribe{HTTP: "https://list.example/unsub?id=42"},
		},
		{
			name: "https with one-click post",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<https://list.example/u?id=42>"},
				"List-Unsubscribe-Post": {"List-Unsubscribe=One-Click"},
			},
			want: Unsubscribe{OneClick: "https://list.example/u?id=42"},
		},
		{
			name: "https + mailto with one-click post (RFC 8058 canonical)",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<https://list.example/u?id=42>, <mailto:u@list.example?subject=unsub>"},
				"List-Unsubscribe-Post": {"List-Unsubscribe=One-Click"},
			},
			want: Unsubscribe{
				OneClick: "https://list.example/u?id=42",
				Mailto:   "mailto:u@list.example?subject=unsub",
			},
		},
		{
			name: "http (non-TLS) does not promote",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<http://list.example/u?id=42>"},
				"List-Unsubscribe-Post": {"List-Unsubscribe=One-Click"},
			},
			want: Unsubscribe{HTTP: "http://list.example/u?id=42"},
		},
		{
			name: "post header with non-one-click value",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<https://list.example/u?id=42>"},
				"List-Unsubscribe-Post": {"List-Unsubscribe=Confirm"},
			},
			want: Unsubscribe{HTTP: "https://list.example/u?id=42"},
		},
		{
			name: "case-insensitive post key",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<https://list.example/u?id=42>"},
				"List-Unsubscribe-Post": {"list-unsubscribe=One-Click"},
			},
			want: Unsubscribe{OneClick: "https://list.example/u?id=42"},
		},
		{
			name: "missing brackets tolerated",
			hdrs: map[string][]string{
				"List-Unsubscribe": {"mailto:u@list.example, https://list.example/u"},
			},
			want: Unsubscribe{
				Mailto: "mailto:u@list.example",
				HTTP:   "https://list.example/u",
			},
		},
		{
			name: "multiple https — first wins for HTTP",
			hdrs: map[string][]string{
				"List-Unsubscribe": {"<https://a.example/u>, <https://b.example/u>"},
			},
			want: Unsubscribe{HTTP: "https://a.example/u"},
		},
		{
			name: "multiple https — first promoted to OneClick",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<https://a.example/u>, <https://b.example/u>"},
				"List-Unsubscribe-Post": {"List-Unsubscribe=One-Click"},
			},
			want: Unsubscribe{OneClick: "https://a.example/u"},
		},
		{
			name: "garbage values ignored",
			hdrs: map[string][]string{
				"List-Unsubscribe": {"   <not-a-uri>, , <ftp://x.example/y>  "},
			},
			want: Unsubscribe{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := textproto.MIMEHeader{}
			for k, vs := range tc.hdrs {
				for _, v := range vs {
					h.Add(k, v)
				}
			}
			got := ParseListUnsubscribe(h)
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestUnsubscribeAvailable(t *testing.T) {
	cases := []struct {
		name string
		u    Unsubscribe
		want bool
	}{
		{"empty", Unsubscribe{}, false},
		{"oneclick", Unsubscribe{OneClick: "https://x"}, true},
		{"mailto", Unsubscribe{Mailto: "mailto:x"}, true},
		{"http", Unsubscribe{HTTP: "https://x"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.u.Available(); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
