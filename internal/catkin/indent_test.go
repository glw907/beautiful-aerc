package catkin

import "testing"

func TestIndentTab(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		cur     int
		wantSrc string
		wantCur int
	}{
		{
			name:    "list line gains two leading spaces",
			src:     "- one",
			cur:     2,
			wantSrc: "  - one",
			wantCur: 4,
		},
		{
			name:    "task line indents",
			src:     "- [ ] one",
			cur:     6,
			wantSrc: "  - [ ] one",
			wantCur: 8,
		},
		{
			name:    "paragraph inserts two spaces at cursor",
			src:     "abcdef",
			cur:     3,
			wantSrc: "abc  def",
			wantCur: 5,
		},
		{
			name:    "paragraph at start",
			src:     "abc",
			cur:     0,
			wantSrc: "  abc",
			wantCur: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSrc, gotCur := indentTab(tt.src, tt.cur)
			if gotSrc != tt.wantSrc || gotCur != tt.wantCur {
				t.Errorf("indentTab:\n  got  (%q, %d)\n  want (%q, %d)", gotSrc, gotCur, tt.wantSrc, tt.wantCur)
			}
		})
	}
}

func TestIndentShiftTab(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		cur         int
		wantSrc     string
		wantCur     int
		wantHandled bool
	}{
		{
			name:        "outdent indented list",
			src:         "  - one",
			cur:         4,
			wantSrc:     "- one",
			wantCur:     2,
			wantHandled: true,
		},
		{
			name:        "depth-zero list ignored",
			src:         "- one",
			cur:         2,
			wantSrc:     "- one",
			wantCur:     2,
			wantHandled: false,
		},
		{
			name:        "paragraph ignored",
			src:         "abc",
			cur:         1,
			wantSrc:     "abc",
			wantCur:     1,
			wantHandled: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSrc, gotCur, gotHandled := indentShiftTab(tt.src, tt.cur)
			if gotSrc != tt.wantSrc || gotCur != tt.wantCur || gotHandled != tt.wantHandled {
				t.Errorf("indentShiftTab:\n  got  (%q, %d, %v)\n  want (%q, %d, %v)",
					gotSrc, gotCur, gotHandled, tt.wantSrc, tt.wantCur, tt.wantHandled)
			}
		})
	}
}
