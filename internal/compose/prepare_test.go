package compose

import (
	"strings"
	"testing"
)

func TestPrepare(t *testing.T) {
	tests := []struct {
		name  string
		input string
		opts  Options
		want  string
	}{
		{
			name: "new compose with empty To",
			input: join(
				"From: geoff@907.life",
				"To:",
				"Subject:",
				"",
				"",
			),
			opts: Options{InjectCcBcc: true},
			want: join(
				"From: geoff@907.life",
				"To:",
				"Cc:",
				"Bcc:",
				"Subject:",
				"",
				"",
			),
		},
		{
			name: "reply with quoted text",
			input: join(
				"From: geoff@907.life",
				"To: Alice <alice@example.com>",
				"Subject: Re: Hello",
				"",
				"",
				"> This is a quoted line that was wrapped oddly by the",
				"> original sender's client and should be",
				"> reflowed.",
			),
			opts: Options{InjectCcBcc: true},
			// NOTE: net/mail quotes "Alice". Reflow wraps at 72 chars.
			want: join(
				"From: geoff@907.life",
				`To: "Alice" <alice@example.com>`,
				"Cc:",
				"Bcc:",
				"Subject: Re: Hello",
				"",
				"",
				"> This is a quoted line that was wrapped oddly by the original sender's",
				"> client and should be reflowed.",
			),
		},
		{
			name: "forward with empty To and quoted text",
			input: join(
				"From: geoff@907.life",
				"To:",
				"Subject: Fwd: News",
				"",
				"",
				"> Forwarded content.",
			),
			opts: Options{InjectCcBcc: true},
			want: join(
				"From: geoff@907.life",
				"To:",
				"Cc:",
				"Bcc:",
				"Subject: Fwd: News",
				"",
				"",
				"> Forwarded content.",
			),
		},
		{
			name: "multi-recipient folding",
			input: join(
				"From: geoff@907.life",
				"To: Alice <alice@example.com>, Bob <bob@example.com>, Charlie <charlie@example.com>",
				"Subject: Group",
				"",
				"Hello everyone.",
			),
			opts: Options{InjectCcBcc: true},
			// NOTE: net/mail quotes these names. Also, the folding depends on
			// actual string widths. Run the test, check actual output, adjust.
			want: join(
				"From: geoff@907.life",
				`To: "Alice" <alice@example.com>, "Bob" <bob@example.com>,`,
				`    "Charlie" <charlie@example.com>`,
				"Cc:",
				"Bcc:",
				"Subject: Group",
				"",
				"Hello everyone.",
			),
		},
		{
			name: "folded continuation header unfolded first",
			input: join(
				"From: geoff@907.life",
				"To: alice@example.com,",
				" bob@example.com",
				"Subject: Hi",
				"",
				"Body.",
			),
			opts: Options{InjectCcBcc: true},
			want: join(
				"From: geoff@907.life",
				"To: alice@example.com, bob@example.com",
				"Cc:",
				"Bcc:",
				"Subject: Hi",
				"",
				"Body.",
			),
		},
		{
			name: "no-cc-bcc flag",
			input: join(
				"From: geoff@907.life",
				"To: alice@dom",
				"Subject: Hi",
				"",
				"Body.",
			),
			opts: Options{InjectCcBcc: false},
			want: join(
				"From: geoff@907.life",
				"To: alice@dom",
				"Subject: Hi",
				"",
				"Body.",
			),
		},
		{
			name:  "malformed input no blank line",
			input: "From: alice@dom\nTo: bob@dom\nBody text\n",
			opts:  Options{InjectCcBcc: true},
			want:  "From: alice@dom\nTo: bob@dom\nBody text\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(Prepare([]byte(tt.input), tt.opts))
			if got != tt.want {
				t.Errorf("Prepare() mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, tt.want)
			}
		})
	}
}

// join builds a newline-terminated string from lines.
func join(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}
