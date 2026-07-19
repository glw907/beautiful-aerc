package main

import (
	"strings"
	"testing"
)

func TestExtractBody(t *testing.T) {
	tests := []struct {
		name             string
		eml              []byte
		wantType         string
		wantBodyContains string
		wantErr          bool
	}{
		{
			name: "html-only",
			eml: []byte("From: a@x\r\nTo: b@y\r\n" +
				"Content-Type: text/html; charset=utf-8\r\n\r\n" +
				"<p>HTML body</p>"),
			wantType:         "text/html",
			wantBodyContains: "HTML body",
		},
		{
			name: "prefer html over plain",
			eml: []byte("From: a@x\r\nTo: b@y\r\n" +
				"Content-Type: multipart/alternative; boundary=B\r\n\r\n" +
				"--B\r\nContent-Type: text/plain\r\n\r\nPlain text\r\n" +
				"--B\r\nContent-Type: text/html\r\n\r\n<p>HTML wins</p>\r\n" +
				"--B--\r\n"),
			wantType:         "text/html",
			wantBodyContains: "HTML wins",
		},
		{
			name: "fall back to plain",
			eml: []byte("From: a@x\r\nTo: b@y\r\n" +
				"Content-Type: text/plain; charset=utf-8\r\n\r\n" +
				"Plain only"),
			wantType:         "text/plain",
			wantBodyContains: "Plain only",
		},
		{
			name: "no text part",
			eml: []byte("From: a@x\r\nTo: b@y\r\n" +
				"Content-Type: application/octet-stream\r\n\r\n" +
				"binary data"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractBody(tt.eml)
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractBody() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.contentType != tt.wantType {
				t.Errorf("contentType = %q, want %q", got.contentType, tt.wantType)
			}
			if !strings.Contains(string(got.content), tt.wantBodyContains) {
				t.Errorf("content %q does not contain %q", string(got.content), tt.wantBodyContains)
			}
		})
	}
}

func TestRenderOutPath(t *testing.T) {
	tests := []struct {
		name  string
		dir   string
		arm   string
		class string
		id    string
		want  string
	}{
		{
			name:  "lynx arm",
			dir:   "/tmp/renders",
			arm:   "lynx",
			class: "github-ci",
			id:    "StnkqV7d8LyB",
			want:  "/tmp/renders/lynx/github-ci/StnkqV7d8LyB.md",
		},
		{
			name:  "w3m arm",
			dir:   "corpus/renders",
			arm:   "w3m",
			class: "marketing",
			id:    "abc123",
			want:  "corpus/renders/w3m/marketing/abc123.md",
		},
		{
			name:  "legacy arm",
			dir:   "corpus/renders",
			arm:   "legacy",
			class: "personal",
			id:    "xyz-789",
			want:  "corpus/renders/legacy/personal/xyz-789.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderOutPath(tt.dir, tt.arm, tt.class, tt.id)
			if got != tt.want {
				t.Errorf("renderOutPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
