// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSize(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"1024", 1024, false},
		{"1KB", 1024, false},
		{"2MB", 2 * 1024 * 1024, false},
		{"2GB", 2 * 1024 * 1024 * 1024, false},
		{"1TB", 1 * 1024 * 1024 * 1024 * 1024, false},
		{"1.5GB", int64(1.5 * 1024 * 1024 * 1024), false},
		{"-1", 0, true},
		{"abc", 0, true},
		{"5XB", 0, true},
	}
	for _, c := range cases {
		got, err := parseSize(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseSize(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("parseSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestLoadCache(t *testing.T) {
	cases := []struct {
		name    string
		toml    string
		want    CacheConfig
		wantErr bool
	}{
		{
			name: "default when missing",
			toml: ``,
			want: CacheConfig{
				MaxSize:           2 * 1024 * 1024 * 1024,
				MaxAttachmentSize: 2 * 1024 * 1024 * 1024,
			},
		},
		{
			name: "explicit 1GB",
			toml: `[cache]` + "\n" + `max-size = "1GB"`,
			want: CacheConfig{
				MaxSize:           1024 * 1024 * 1024,
				MaxAttachmentSize: 2 * 1024 * 1024 * 1024,
			},
		},
		{
			name: "zero disables",
			toml: `[cache]` + "\n" + `max-size = "0"`,
			want: CacheConfig{
				MaxSize:           0,
				MaxAttachmentSize: 2 * 1024 * 1024 * 1024,
			},
		},
		{
			name:    "garbage rejected",
			toml:    `[cache]` + "\n" + `max-size = "five gigs"`,
			wantErr: true,
		},
		{
			name: "explicit max-attachment-size",
			toml: `[cache]` + "\n" +
				`max-size = "1GB"` + "\n" +
				`max-attachment-size = "500MB"`,
			want: CacheConfig{
				MaxSize:           1024 * 1024 * 1024,
				MaxAttachmentSize: 500 * 1024 * 1024,
			},
		},
		{
			name: "default attachment cap when section absent",
			toml: ``,
			want: CacheConfig{
				MaxSize:           2 * 1024 * 1024 * 1024,
				MaxAttachmentSize: 2 * 1024 * 1024 * 1024,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(c.toml), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			got, err := LoadCache(path)
			if (err != nil) != c.wantErr {
				t.Fatalf("LoadCache err=%v, wantErr=%v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}
