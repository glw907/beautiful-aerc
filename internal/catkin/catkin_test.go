package catkin

import "testing"

func TestNewReturnsUsableModel(t *testing.T) {
	m := New()
	if got := m.Value(); got != "" {
		t.Errorf("new model: Value() = %q, want empty", got)
	}
	m = m.WithValue("hello")
	if got := m.Value(); got != "hello" {
		t.Errorf("after WithValue: Value() = %q, want %q", got, "hello")
	}
}

func TestWithWidthDoesNotReflow(t *testing.T) {
	src := "> this is a moderately long quoted paragraph that exceeds twenty cells"
	m := New().WithValue(src).WithSize(80, 10)
	m = m.WithWidth(20)
	if got := m.Value(); got != src {
		t.Errorf("WithWidth must not mutate source:\n got %q\nwant %q", got, src)
	}
}
