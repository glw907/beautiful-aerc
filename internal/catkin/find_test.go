package catkin

import "testing"

func TestFindAll(t *testing.T) {
	tests := []struct {
		name string
		src  string
		q    string
		ci   bool
		want []int
	}{
		{"simple", "foo bar foo", "foo", false, []int{0, 8}},
		{"none", "abc", "z", false, nil},
		{"empty", "abc", "", false, nil},
		{"case-insensitive", "FOO foo", "foo", true, []int{0, 4}},
		{"case-sensitive", "FOO foo", "foo", false, []int{4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findAll(tt.src, tt.q, tt.ci)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("idx %d: got %d want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFindStateFooterRows(t *testing.T) {
	tests := []struct {
		mode findMode
		want int
	}{
		{findIdle, 0},
		{findFind, 1},
		{findReplace, 2},
	}
	for _, tt := range tests {
		f := findState{mode: tt.mode}
		if got := f.footerRows(); got != tt.want {
			t.Errorf("mode %d: got %d want %d", tt.mode, got, tt.want)
		}
	}
}

func TestReplaceCurrent(t *testing.T) {
	m := New().WithSize(40, 10).WithValue("foo bar foo")
	m.find = findState{mode: findReplace, query: "foo", replacement: "baz", inputFocus: 1}
	m.find.recomputeMatches(m.buf.Value(), 0)

	m = m.replaceCurrent()
	if got := m.buf.Value(); got != "baz bar foo" {
		t.Errorf("after first replace: got %q", got)
	}

	m = m.replaceCurrent()
	if got := m.buf.Value(); got != "baz bar baz" {
		t.Errorf("after second replace: got %q", got)
	}
}

func TestReplaceAll(t *testing.T) {
	m := New().WithSize(40, 10).WithValue("a a a")
	m.find = findState{mode: findReplace, query: "a", replacement: "X", inputFocus: 1}
	m.find.recomputeMatches(m.buf.Value(), 0)
	m = m.replaceAll()
	if got := m.buf.Value(); got != "X X X" {
		t.Errorf("got %q want %q", got, "X X X")
	}
}
