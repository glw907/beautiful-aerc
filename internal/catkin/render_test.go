package catkin

import (
	"strings"
	"testing"
)

func TestRenderPlainSingleLine(t *testing.T) {
	got := Render("hello", 20, 5, 0, 5)
	want := "hello█"
	// Render pads to height; only check the first row.
	first := strings.SplitN(got, "\n", 2)[0]
	if first != want {
		t.Errorf("Render plain:\nfirst row %q\nwant      %q", first, want)
	}
}

func TestRenderDisplayWrapsLongLine(t *testing.T) {
	src := "abcdefghijklmnopqrst"
	// cursor at last rune (offset 19) lands in the second chunk at col 9.
	got := Render(src, 10, 5, 0, 19)
	rows := strings.Split(got, "\n")
	if rows[0] != "abcdefghij" || rows[1] != "klmnopqrs█" {
		t.Errorf("Render display-wrap rows:\n[0]=%q\n[1]=%q", rows[0], rows[1])
	}
}
