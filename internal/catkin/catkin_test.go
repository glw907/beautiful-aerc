package catkin

import "testing"

func TestNewReturnsUsableModel(t *testing.T) {
	m := New()
	if got := m.Value(); got != "" {
		t.Errorf("new model: Value() = %q, want empty", got)
	}
	m.SetValue("hello")
	if got := m.Value(); got != "hello" {
		t.Errorf("after SetValue: Value() = %q, want %q", got, "hello")
	}
}
