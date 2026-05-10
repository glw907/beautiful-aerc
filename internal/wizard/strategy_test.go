package wizard

import (
	"testing"

	"github.com/glw907/poplar/internal/config"
)

func TestSelectStrategy(t *testing.T) {
	cases := []struct {
		preset string
		want   config.CredentialStrategy
	}{
		{"fastmail", config.StrategyAPIToken},
		{"gmail", config.StrategyOAuth},
		{"yahoo", config.StrategyAppPassword},
		{"imap", config.StrategyPlainIMAP},
		{"jmap", config.StrategyPlainJMAP},
	}
	for _, c := range cases {
		got, err := SelectStrategy(c.preset)
		if err != nil {
			t.Fatalf("SelectStrategy(%q): %v", c.preset, err)
		}
		if got != c.want {
			t.Errorf("SelectStrategy(%q) = %v, want %v", c.preset, got, c.want)
		}
	}
}

func TestSelectStrategyUnknown(t *testing.T) {
	if _, err := SelectStrategy("totally-not-a-provider"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
