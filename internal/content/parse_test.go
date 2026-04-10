package content

import (
	"testing"
)

func spansEqual(t *testing.T, got, want []Span) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("span count: got %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("span[%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestParseSpans(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Span
	}{
		{
			name:  "plain text",
			input: "hello world",
			want:  []Span{Text{Content: "hello world"}},
		},
		{
			name:  "bold",
			input: "hello **world**",
			want:  []Span{Text{Content: "hello "}, Bold{Content: "world"}},
		},
		{
			name:  "italic",
			input: "hello *world*",
			want:  []Span{Text{Content: "hello "}, Italic{Content: "world"}},
		},
		{
			name:  "inline code",
			input: "use `fmt.Println`",
			want:  []Span{Text{Content: "use "}, Code{Content: "fmt.Println"}},
		},
		{
			name:  "link",
			input: "visit [example](https://example.com) today",
			want: []Span{
				Text{Content: "visit "},
				Link{Text: "example", URL: "https://example.com"},
				Text{Content: " today"},
			},
		},
		{
			name:  "mixed",
			input: "**bold** and *italic* and `code`",
			want: []Span{
				Bold{Content: "bold"},
				Text{Content: " and "},
				Italic{Content: "italic"},
				Text{Content: " and "},
				Code{Content: "code"},
			},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSpans(tt.input)
			spansEqual(t, got, tt.want)
		})
	}
}
