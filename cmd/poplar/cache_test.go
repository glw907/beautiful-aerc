// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestCacheStats_OutputFormat(t *testing.T) {
	// formatStatsLine is a pure helper used by the cache stats
	// subcommand. It takes a stats struct and returns the tab-aligned
	// row. Test it in isolation; integration with real accounts is
	// covered by manual smoke at install.
	row := formatStatsLine(statsRow{
		Account:       "fastmail",
		HeadersCount:  1247,
		BodiesCount:   342,
		BodiesBytes:   19_300_000,
		OutboxPending: 3,
		OutboxOther:   0,
		DBBytes:       44_200_000,
	})
	for _, want := range []string{"fastmail", "1,247", "342", "18.4 MB", "3 pending", "42.2 MB"} {
		if !strings.Contains(row, want) {
			t.Errorf("row missing %q\nrow=%q", want, row)
		}
	}
}

func TestCacheStats_Header(t *testing.T) {
	var buf bytes.Buffer
	writeStatsHeader(&buf)
	got := buf.String()
	for _, want := range []string{"ACCOUNT", "HEADERS", "BODIES", "OUTBOX", "DB SIZE"} {
		if !strings.Contains(got, want) {
			t.Errorf("header missing %q\nheader=%q", want, got)
		}
	}
}

func TestHumanizeBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1500, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{int64(1.5 * 1024 * 1024), "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, c := range cases {
		got := humanizeBytes(c.in)
		if got != c.want {
			t.Errorf("humanizeBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseEvictDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"24h", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"1.5h", 90 * time.Minute, false},
		{"-1d", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		got, err := parseEvictDuration(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseEvictDuration(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("parseEvictDuration(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRunEvict_NoMatchingAccount(t *testing.T) {
	// This test depends on loadAccounts succeeding. The dev config has
	// accounts with empty names which causes loadAccounts to error, so
	// we skip rather than fail with the wrong error.
	if _, _, err := loadAccounts(); err != nil {
		t.Skipf("loadAccounts error (dev config): %v", err)
	}
	var buf bytes.Buffer
	err := runEvict(context.Background(), &buf, time.Now(), "no-such-account")
	if err == nil {
		t.Errorf("expected error for unknown account scope")
	}
}
