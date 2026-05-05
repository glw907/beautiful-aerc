package catkin

import "testing"

func TestSmartEnter(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		cur     int
		wantSrc string
		wantCur int
	}{
		{
			name:    "plain newline",
			src:     "abc",
			cur:     3,
			wantSrc: "abc\n",
			wantCur: 4,
		},
		{
			name:    "split paragraph",
			src:     "abcdef",
			cur:     3,
			wantSrc: "abc\ndef",
			wantCur: 4,
		},
		{
			name:    "continue dash list",
			src:     "- one",
			cur:     5,
			wantSrc: "- one\n- ",
			wantCur: 8,
		},
		{
			name:    "continue ordered list increments",
			src:     "1. one",
			cur:     6,
			wantSrc: "1. one\n2. ",
			wantCur: 10,
		},
		{
			name:    "continue ordered list at 9 → 10",
			src:     "9. nine",
			cur:     7,
			wantSrc: "9. nine\n10. ",
			wantCur: 12,
		},
		{
			name:    "continue quote prefix",
			src:     "> hello",
			cur:     7,
			wantSrc: "> hello\n> ",
			wantCur: 10,
		},
		{
			name:    "continue task list keeps unchecked box",
			src:     "- [ ] todo",
			cur:     10,
			wantSrc: "- [ ] todo\n- [ ] ",
			wantCur: 17,
		},
		{
			name:    "terminate empty dash list",
			src:     "- one\n- ",
			cur:     8,
			wantSrc: "- one\n",
			wantCur: 6,
		},
		{
			name:    "terminate empty quote",
			src:     "> ",
			cur:     2,
			wantSrc: "",
			wantCur: 0,
		},
		{
			name:    "trailing single space is stripped",
			src:     "abc ",
			cur:     4,
			wantSrc: "abc\n",
			wantCur: 4,
		},
		{
			name:    "double trailing space preserved",
			src:     "abc  ",
			cur:     5,
			wantSrc: "abc  \n",
			wantCur: 6,
		},
		{
			name:    "triple trailing space stripped",
			src:     "abc   ",
			cur:     6,
			wantSrc: "abc\n",
			wantCur: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSrc, gotCur := smartEnter(tt.src, tt.cur)
			if gotSrc != tt.wantSrc || gotCur != tt.wantCur {
				t.Errorf("smartEnter:\n  got  (%q, %d)\n  want (%q, %d)", gotSrc, gotCur, tt.wantSrc, tt.wantCur)
			}
		})
	}
}
