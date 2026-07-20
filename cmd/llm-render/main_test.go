package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	principles := "Render email as markdown.\n"
	content := "<p>Hello world</p>"

	got := buildPrompt(principles, content)

	if !strings.Contains(got, principles) {
		t.Error("buildPrompt: principles not present in prompt")
	}
	if !strings.Contains(got, content) {
		t.Errorf("buildPrompt: email content not present in prompt")
	}
	// The task forbids leaking ideal renders, so verify no "ideal" or "example render" text
	lower := strings.ToLower(got)
	for _, banned := range []string{"ideal render", "example output", "correct answer"} {
		if strings.Contains(lower, banned) {
			t.Errorf("buildPrompt: prompt contains banned phrase %q", banned)
		}
	}
}

func TestTokenEst(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{name: "empty", output: "", want: 0},
		{name: "four chars", output: "abcd", want: 1},
		{name: "eight chars", output: "abcdefgh", want: 2},
		{name: "three chars rounds down", output: "abc", want: 0},
		{name: "five chars", output: "abcde", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenEst(tt.output)
			if got != tt.want {
				t.Errorf("tokenEst(%q) = %d, want %d", tt.output, got, tt.want)
			}
		})
	}
}

func TestMergeEntry(t *testing.T) {
	existing := json.RawMessage(`{"class":"github-ci","lynx":{"ms":10,"bytes":100}}`)
	stat := llmStat{MS: 500, Tokens: 42}

	got, err := mergeEntry(existing, stat)
	if err != nil {
		t.Fatalf("mergeEntry: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if _, ok := m["class"]; !ok {
		t.Error("mergeEntry: class field missing after merge")
	}
	if _, ok := m["lynx"]; !ok {
		t.Error("mergeEntry: lynx field missing after merge")
	}
	if _, ok := m["llm"]; !ok {
		t.Error("mergeEntry: llm field missing after merge")
	}

	var ls llmStat
	if err := json.Unmarshal(m["llm"], &ls); err != nil {
		t.Fatalf("unmarshal llm: %v", err)
	}
	if ls.MS != 500 {
		t.Errorf("llmStat.MS = %d, want 500", ls.MS)
	}
	if ls.Tokens != 42 {
		t.Errorf("llmStat.Tokens = %d, want 42", ls.Tokens)
	}
}

func TestMergeEntryError(t *testing.T) {
	existing := json.RawMessage(`{"class":"newsletter"}`)
	stat := llmStat{Error: "timeout"}

	got, err := mergeEntry(existing, stat)
	if err != nil {
		t.Fatalf("mergeEntry: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var ls llmStat
	json.Unmarshal(m["llm"], &ls) //nolint:errcheck
	if ls.Error != "timeout" {
		t.Errorf("llmStat.Error = %q, want %q", ls.Error, "timeout")
	}
}

func TestRenderOutPath(t *testing.T) {
	tests := []struct {
		name  string
		dir   string
		class string
		id    string
		want  string
	}{
		{
			name:  "github-ci entry",
			dir:   "corpus/renders/llm",
			class: "github-ci",
			id:    "StnkqV7d8LyB",
			want:  "corpus/renders/llm/github-ci/StnkqV7d8LyB.md",
		},
		{
			name:  "newsletter entry",
			dir:   "/tmp/renders/llm",
			class: "newsletter",
			id:    "abc123",
			want:  "/tmp/renders/llm/newsletter/abc123.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderOutPath(tt.dir, tt.class, tt.id)
			if got != tt.want {
				t.Errorf("renderOutPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasValidLLMStat(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want bool
	}{
		{
			name: "nil raw",
			raw:  nil,
			want: false,
		},
		{
			name: "no llm key",
			raw:  json.RawMessage(`{"class":"github-ci","lynx":{"ms":10}}`),
			want: false,
		},
		{
			name: "llm with ms and no error",
			raw:  json.RawMessage(`{"class":"github-ci","llm":{"ms":500,"tokens":42}}`),
			want: true,
		},
		{
			name: "llm with zero ms",
			raw:  json.RawMessage(`{"class":"github-ci","llm":{"ms":0,"tokens":42}}`),
			want: false,
		},
		{
			name: "llm with error",
			raw:  json.RawMessage(`{"class":"github-ci","llm":{"ms":100,"tokens":0,"error":"timeout"}}`),
			want: false,
		},
		{
			name: "invalid json",
			raw:  json.RawMessage(`not json`),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasValidLLMStat(tt.raw)
			if got != tt.want {
				t.Errorf("hasValidLLMStat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractBodyLLM(t *testing.T) {
	tests := []struct {
		name             string
		eml              []byte
		wantType         string
		wantBodyContains string
		wantErr          bool
	}{
		{
			name: "html preferred",
			eml: []byte("From: a@x\r\nTo: b@y\r\n" +
				"Content-Type: multipart/alternative; boundary=B\r\n\r\n" +
				"--B\r\nContent-Type: text/plain\r\n\r\nPlain text\r\n" +
				"--B\r\nContent-Type: text/html\r\n\r\n<p>HTML wins</p>\r\n" +
				"--B--\r\n"),
			wantType:         "text/html",
			wantBodyContains: "HTML wins",
		},
		{
			name: "plain fallback",
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
				t.Errorf("content does not contain %q", tt.wantBodyContains)
			}
		})
	}
}
