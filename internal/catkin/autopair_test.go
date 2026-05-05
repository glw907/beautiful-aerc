package catkin

import "testing"

func TestApplyAutoPair(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		cur       int
		r         rune
		wantSrc   string
		wantCur   int
	}{
		{"insert star", "foo", 3, '*', "foo**", 4},
		{"insert backtick mid", "ab", 1, '`', "a``b", 2},
		{"insert bracket", "x", 1, '[', "x[]", 2},
		{"step over closer", "*x*", 2, '*', "*x*", 3},
		{"expand emphasis to bold", "**", 1, '*', "****", 2},
		{"expand under to bold", "__", 1, '_', "____", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSrc, gotCur := applyAutoPair(tt.src, tt.cur, tt.r)
			if gotSrc != tt.wantSrc || gotCur != tt.wantCur {
				t.Fatalf("got (%q, %d) want (%q, %d)", gotSrc, gotCur, tt.wantSrc, tt.wantCur)
			}
		})
	}
}

func TestTryPairDelete(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		cur     int
		wantOK  bool
		wantSrc string
		wantCur int
	}{
		{"delete star pair", "**", 1, true, "", 0},
		{"delete bracket pair", "[]", 1, true, "", 0},
		{"no pair", "ab", 1, false, "", 0},
		{"at start", "**", 0, false, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSrc, gotCur, ok := tryPairDelete(tt.src, tt.cur)
			if ok != tt.wantOK {
				t.Fatalf("ok: got %v want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if gotSrc != tt.wantSrc || gotCur != tt.wantCur {
				t.Fatalf("got (%q, %d) want (%q, %d)", gotSrc, gotCur, tt.wantSrc, tt.wantCur)
			}
		})
	}
}

func TestInCodeContext(t *testing.T) {
	tests := []struct {
		name string
		src  string
		cur  int
		want bool
	}{
		{"prose", "hello world", 5, false},
		{"in inline code", "foo `bar` baz", 6, true},
		{"after inline close", "foo `bar` baz", 10, false},
		{"inside fence", "```\nfoo\n```", 5, true},
		{"indented code", "    code", 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inCodeContext(tt.src, tt.cur); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
