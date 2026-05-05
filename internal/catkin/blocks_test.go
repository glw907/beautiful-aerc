package catkin

import (
	"reflect"
	"testing"
)

func TestClassifyTable(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []LineContext
	}{
		{
			name:  "blank line",
			lines: []string{""},
			want:  []LineContext{{Kind: BlockBlank}},
		},
		{
			name:  "plain paragraph",
			lines: []string{"hello world"},
			want:  []LineContext{{Kind: BlockParagraph, PostPrefix: "hello world"}},
		},
		{
			name:  "ATX heading h1",
			lines: []string{"# Title"},
			want:  []LineContext{{Kind: BlockHeading, HeadingLevel: 1, PostPrefix: "Title"}},
		},
		{
			name:  "ATX heading h3",
			lines: []string{"### Sub"},
			want:  []LineContext{{Kind: BlockHeading, HeadingLevel: 3, PostPrefix: "Sub"}},
		},
		{
			name:  "single quote prefix",
			lines: []string{"> hi"},
			want:  []LineContext{{Kind: BlockQuote, QuoteDepth: 1, PrefixWidth: 2, PostPrefix: "hi"}},
		},
		{
			name:  "nested quote depth 2",
			lines: []string{"> > nested"},
			want:  []LineContext{{Kind: BlockQuote, QuoteDepth: 2, PrefixWidth: 4, PostPrefix: "nested"}},
		},
		{
			name:  "dash list",
			lines: []string{"- item one"},
			want:  []LineContext{{Kind: BlockListItem, ListMarker: "-", PrefixWidth: 2, PostPrefix: "item one"}},
		},
		{
			name:  "ordered list",
			lines: []string{"1. first"},
			want:  []LineContext{{Kind: BlockListItem, ListMarker: "1.", PrefixWidth: 3, PostPrefix: "first"}},
		},
		{
			name:  "task list unchecked",
			lines: []string{"- [ ] todo"},
			want:  []LineContext{{Kind: BlockTaskItem, ListMarker: "- [ ]", PrefixWidth: 6, PostPrefix: "todo"}},
		},
		{
			name:  "task list checked",
			lines: []string{"- [x] done"},
			want:  []LineContext{{Kind: BlockTaskItem, ListMarker: "- [x]", PrefixWidth: 6, PostPrefix: "done"}},
		},
		{
			name:  "fenced code open and close",
			lines: []string{"```", "code", "```"},
			want: []LineContext{
				{Kind: BlockCodeFence, InsideFence: false, PostPrefix: "```"},
				{Kind: BlockCodeFence, InsideFence: true, PostPrefix: "code"},
				{Kind: BlockCodeFence, InsideFence: false, PostPrefix: "```"},
			},
		},
		{
			name:  "indented code (4-space)",
			lines: []string{"    indented"},
			want:  []LineContext{{Kind: BlockCodeIndent, PrefixWidth: 4, PostPrefix: "indented"}},
		},
		{
			name:  "table header + separator",
			lines: []string{"| a | b |", "| --- | --- |"},
			want: []LineContext{
				{Kind: BlockTable, PostPrefix: "| a | b |"},
				{Kind: BlockTable, PostPrefix: "| --- | --- |"},
			},
		},
		{
			name:  "quoted heading",
			lines: []string{"> # heading in quote"},
			want:  []LineContext{{Kind: BlockHeading, QuoteDepth: 1, HeadingLevel: 1, PrefixWidth: 2, PostPrefix: "heading in quote"}},
		},
		{
			name:  "quoted list",
			lines: []string{"> - item"},
			want:  []LineContext{{Kind: BlockListItem, QuoteDepth: 1, ListMarker: "-", PrefixWidth: 4, PostPrefix: "item"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.lines)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Classify(%q):\ngot  %#v\nwant %#v", tt.lines, got, tt.want)
			}
		})
	}
}
