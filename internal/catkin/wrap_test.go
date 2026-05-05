package catkin

import "testing"

func TestWrapWordBold(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		cur     int
		wantSrc string
		wantCur int
	}{
		{
			name:    "wrap word at cursor inside",
			src:     "the quick brown",
			cur:     6,
			wantSrc: "the **quick** brown",
			wantCur: 13,
		},
		{
			name:    "wrap word at end-of-word boundary",
			src:     "abc",
			cur:     3,
			wantSrc: "**abc**",
			wantCur: 7,
		},
		{
			name:    "empty buffer inserts empty wrapper",
			src:     "",
			cur:     0,
			wantSrc: "****",
			wantCur: 2,
		},
		{
			name:    "cursor in whitespace inserts empty wrapper",
			src:     "a  b",
			cur:     2,
			wantSrc: "a **** b",
			wantCur: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSrc, gotCur := wrapWord(tt.src, tt.cur, "**")
			if gotSrc != tt.wantSrc || gotCur != tt.wantCur {
				t.Errorf("wrapWord:\n  got  (%q, %d)\n  want (%q, %d)", gotSrc, gotCur, tt.wantSrc, tt.wantCur)
			}
		})
	}
}

func TestInsertLinkSkeleton(t *testing.T) {
	gotSrc, gotCur := insertLinkSkeleton("hello ", 6)
	wantSrc, wantCur := "hello [](url)", 7
	if gotSrc != wantSrc || gotCur != wantCur {
		t.Errorf("insertLinkSkeleton:\n  got  (%q, %d)\n  want (%q, %d)", gotSrc, gotCur, wantSrc, wantCur)
	}
}
