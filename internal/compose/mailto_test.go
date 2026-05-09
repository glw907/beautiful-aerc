package compose

import (
	"testing"

	gomail "github.com/emersion/go-message/mail"
)

func TestSeedFromMailto(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		fromEmail string
		want      Draft
		wantErr   bool
	}{
		{
			name:      "address only",
			raw:       "mailto:unsub@list.example",
			fromEmail: "me@example.org",
			want: Draft{
				From: gomail.Address{Address: "me@example.org"},
				To:   []gomail.Address{{Address: "unsub@list.example"}},
			},
		},
		{
			name:      "subject + body",
			raw:       "mailto:u@list.example?subject=unsub&body=please%20unsub%20me",
			fromEmail: "me@example.org",
			want: Draft{
				From:    gomail.Address{Address: "me@example.org"},
				To:      []gomail.Address{{Address: "u@list.example"}},
				Subject: "unsub",
				Body:    "please unsub me",
			},
		},
		{
			name:      "multiple addresses — first wins",
			raw:       "mailto:a@list.example,b@list.example",
			fromEmail: "me@example.org",
			want: Draft{
				From: gomail.Address{Address: "me@example.org"},
				To:   []gomail.Address{{Address: "a@list.example"}},
			},
		},
		{
			name:      "not a mailto",
			raw:       "https://list.example/u",
			fromEmail: "me@example.org",
			wantErr:   true,
		},
		{
			name:      "missing scheme prefix",
			raw:       "u@list.example?subject=x",
			fromEmail: "me@example.org",
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := SeedFromMailto(tc.raw, tc.fromEmail)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.From != tc.want.From {
				t.Errorf("From = %v, want %v", d.From, tc.want.From)
			}
			if len(d.To) != len(tc.want.To) || (len(d.To) > 0 && d.To[0] != tc.want.To[0]) {
				t.Errorf("To = %v, want %v", d.To, tc.want.To)
			}
			if d.Subject != tc.want.Subject {
				t.Errorf("Subject = %q, want %q", d.Subject, tc.want.Subject)
			}
			if d.Body != tc.want.Body {
				t.Errorf("Body = %q, want %q", d.Body, tc.want.Body)
			}
		})
	}
}
