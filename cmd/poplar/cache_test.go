package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestCacheStats_OutputFormat(t *testing.T) {
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
		got, err := parseDuration(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseDuration(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("parseDuration(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRunEvict_NoMatchingAccount(t *testing.T) {
	_, cfgPath := loadReauthTestConfig(t)
	t.Setenv("POPLAR_CONFIG", cfgPath)

	var buf bytes.Buffer
	err := runEvict(context.Background(), &buf, time.Now(), "no-such-account")
	if err == nil {
		t.Fatal("expected error for unknown account scope, got nil")
	}
	if !strings.Contains(err.Error(), "no-such-account") {
		t.Errorf("err = %v, want it to name the missing account", err)
	}
}
