package main

import (
	"fmt"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		h    msgHeaders
		want string
	}{
		// github-ci: sender domain
		{
			name: "github notification from address",
			h:    msgHeaders{from: "notifications@github.com", subject: "Re: fix bug"},
			want: "github-ci",
		},
		{
			name: "github noreply from address",
			h:    msgHeaders{from: "noreply@github.com", subject: "PR merged"},
			want: "github-ci",
		},
		{
			name: "github list-id",
			h:    msgHeaders{from: "noreply@example.com", listID: "<reponame.github.com>", subject: "PR review"},
			want: "github-ci",
		},
		{
			name: "circleci notification",
			h:    msgHeaders{from: "builds@circleci.com", subject: "Build passed"},
			want: "github-ci",
		},
		{
			name: "travis ci notification",
			h:    msgHeaders{from: "builds@travis-ci.com", subject: "Build failed"},
			want: "github-ci",
		},
		{
			name: "dependabot sender",
			h:    msgHeaders{from: "dependabot[bot]@users.noreply.github.com", subject: "Bump dep"},
			want: "github-ci",
		},
		// calendar
		{
			name: "calendar content-type",
			h:    msgHeaders{from: "calendar@google.com", subject: "Event", contentType: "text/calendar; charset=utf-8"},
			want: "calendar",
		},
		{
			name: "calendar invitation subject",
			h:    msgHeaders{from: "calendar@google.com", subject: "Invitation: Team meeting @ 2pm"},
			want: "calendar",
		},
		{
			name: "calendar accepted subject",
			h:    msgHeaders{from: "alice@example.com", subject: "Accepted: Quarterly review"},
			want: "calendar",
		},
		// list-patch
		{
			name: "mailing list by list-id",
			h:    msgHeaders{from: "author@example.com", subject: "Re: some topic", listID: "<go-dev.googlegroups.com>"},
			want: "list-patch",
		},
		{
			name: "patch email subject",
			h:    msgHeaders{from: "author@example.com", subject: "[PATCH v2] fix: handle edge case"},
			want: "list-patch",
		},
		{
			name: "patch subject lowercase",
			h:    msgHeaders{from: "author@example.com", subject: "[patch 1/3] Add feature"},
			want: "list-patch",
		},
		// newsletter
		{
			name: "newsletter with list-unsubscribe",
			h:    msgHeaders{from: "newsletter@substack.com", subject: "Weekly tech digest", listUnsubscribe: "<https://unsubscribe.example.com>"},
			want: "newsletter",
		},
		{
			name: "newsletter editorial content",
			h:    msgHeaders{from: "digest@hacker.news", subject: "Hacker News Top Stories", listUnsubscribe: "<mailto:unsub@hn.com>"},
			want: "newsletter",
		},
		// marketing
		{
			name: "marketing sale subject",
			h:    msgHeaders{from: "deals@shop.com", subject: "50% off everything today!", listUnsubscribe: "<mailto:unsub@shop.com>"},
			want: "marketing",
		},
		{
			name: "marketing buy now",
			h:    msgHeaders{from: "promo@store.com", subject: "Buy now before it expires", listUnsubscribe: "<mailto:unsub@store.com>"},
			want: "marketing",
		},
		{
			name: "marketing flash sale",
			h:    msgHeaders{from: "offers@brand.com", subject: "Flash sale ends tonight!", listUnsubscribe: "<https://brand.com/unsub>"},
			want: "marketing",
		},
		// transactional
		{
			name: "order confirmation from noreply",
			h:    msgHeaders{from: "noreply@amazon.com", subject: "Order confirmation #12345"},
			want: "transactional",
		},
		{
			name: "no-reply sender any content",
			h:    msgHeaders{from: "no-reply@bank.com", subject: "Your statement is ready"},
			want: "transactional",
		},
		{
			name: "donotreply sender",
			h:    msgHeaders{from: "donotreply@service.com", subject: "Account activated"},
			want: "transactional",
		},
		// personal
		{
			name: "personal email",
			h:    msgHeaders{from: "alice@example.com", subject: "Coffee on Tuesday?"},
			want: "personal",
		},
		{
			name: "personal gmail",
			h:    msgHeaders{from: "bob@gmail.com", subject: "Re: the project"},
			want: "personal",
		},
		// unclassified
		{
			name: "automated sender not no-reply",
			h:    msgHeaders{from: "notifications@myapp.com", subject: "Your daily report"},
			want: "unclassified",
		},
		{
			name: "alerts sender no list headers",
			h:    msgHeaders{from: "alerts@monitoring.io", subject: "System health check"},
			want: "unclassified",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classify(tc.h)
			if got != tc.want {
				t.Errorf("classify(%+v) = %q, want %q", tc.h, got, tc.want)
			}
		})
	}
}

func TestIsGitHubCI(t *testing.T) {
	tests := []struct {
		from   string
		listID string
		want   bool
	}{
		{"notifications@github.com", "", true},
		{"noreply@github.com", "", true},
		{"builds@circleci.com", "", true},
		{"builds@travis-ci.com", "", true},
		{"", "<reporeport.github.com>", true},
		{"alice@example.com", "", false},
		{"noreply@shopify.com", "", false},
		{"info@company.com", "", false},
	}
	for _, tc := range tests {
		got := isGitHubCI(tc.from, tc.listID)
		if got != tc.want {
			t.Errorf("isGitHubCI(%q, %q) = %v, want %v", tc.from, tc.listID, got, tc.want)
		}
	}
}

func TestSelectCandidates(t *testing.T) {
	t.Run("caps at max", func(t *testing.T) {
		var cands []candidate
		for i := range 40 {
			cands = append(cands, candidate{
				id:        fmt.Sprintf("id%d", i),
				senderKey: fmt.Sprintf("sender%d@example.com", i),
			})
		}
		got := selectCandidates(cands, 25, 5)
		if len(got) != 25 {
			t.Errorf("got %d, want 25", len(got))
		}
	})

	t.Run("sender cap respected", func(t *testing.T) {
		// 10 messages from one sender
		var cands []candidate
		for i := range 10 {
			cands = append(cands, candidate{
				id:        fmt.Sprintf("id%d", i),
				senderKey: "alice@example.com",
			})
		}
		got := selectCandidates(cands, 25, 5)
		if len(got) != 5 {
			t.Errorf("got %d from one sender, want 5 (sender cap)", len(got))
		}
	})

	t.Run("prefers diversity", func(t *testing.T) {
		// 3 senders, 10 messages each
		var cands []candidate
		for i := range 30 {
			cands = append(cands, candidate{
				id:        fmt.Sprintf("id%d", i),
				senderKey: fmt.Sprintf("sender%d@example.com", i%3),
			})
		}
		got := selectCandidates(cands, 9, 5)
		senderCount := make(map[string]int)
		for _, c := range got {
			senderCount[c.senderKey]++
		}
		// With 3 senders and 9 slots, round-robin gives 3 from each.
		if len(senderCount) != 3 {
			t.Errorf("got %d distinct senders, want 3", len(senderCount))
		}
		for sk, n := range senderCount {
			if n > 5 {
				t.Errorf("sender %q appears %d times, exceeds cap of 5", sk, n)
			}
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := selectCandidates(nil, 25, 5)
		if len(got) != 0 {
			t.Errorf("got %d, want 0", len(got))
		}
	})
}

func TestIsMarketing(t *testing.T) {
	// isMarketing expects a lowercased subject, matching how classify calls it.
	tests := []struct {
		subj string
		want bool
	}{
		{"50% off all items", true},
		{"flash sale: limited time only", true},
		{"buy now or miss out", true},
		{"free shipping on orders over $50", true},
		{"weekly tech roundup", false},
		{"your invoice for march", false},
		{"re: meeting tomorrow", false},
	}
	for _, tc := range tests {
		got := isMarketing(tc.subj)
		if got != tc.want {
			t.Errorf("isMarketing(%q) = %v, want %v", tc.subj, got, tc.want)
		}
	}
}

func TestIsCalendar(t *testing.T) {
	// isCalendar expects lowercased inputs, matching how classify calls it.
	tests := []struct {
		ct   string
		subj string
		want bool
	}{
		{"text/calendar; method=request", "", true},
		{"multipart/mixed", "invitation: team standup", true},
		{"text/plain", "accepted: monthly review", true},
		{"text/plain", "re: project update", false},
		{"text/html", "your receipt #1234", false},
	}
	for _, tc := range tests {
		got := isCalendar(tc.ct, tc.subj)
		if got != tc.want {
			t.Errorf("isCalendar(%q, %q) = %v, want %v", tc.ct, tc.subj, got, tc.want)
		}
	}
}
