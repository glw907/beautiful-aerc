package tidytext

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	rules := []struct {
		name  string
		value bool
	}{
		{"spelling", cfg.Rules.Spelling},
		{"grammar", cfg.Rules.Grammar},
		{"punctuation", cfg.Rules.Punctuation},
		{"whitespace", cfg.Rules.Whitespace},
		{"capitalization", cfg.Rules.Capitalization},
		{"repeated_words", cfg.Rules.RepeatedWords},
		{"missing_punctuation", cfg.Rules.MissingPunctuation},
	}
	for _, r := range rules {
		if !r.value {
			t.Errorf("Rules.%s default = false, want true", r.name)
		}
	}
	if cfg.Rules.OxfordComma != "ignore" {
		t.Errorf("Rules.OxfordComma default = %q, want %q", cfg.Rules.OxfordComma, "ignore")
	}
	if cfg.Style.Ellipsis != "character" {
		t.Errorf("Style.Ellipsis default = %q, want %q", cfg.Style.Ellipsis, "character")
	}
	if cfg.Style.TimeFormat != "ignore" {
		t.Errorf("Style.TimeFormat default = %q, want %q", cfg.Style.TimeFormat, "ignore")
	}
}

func TestResolveAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	t.Run("config takes precedence", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "env-key")
		cfg := DefaultConfig()
		cfg.API.APIKey = "config-key"
		if got := ResolveAPIKey(cfg); got != "config-key" {
			t.Errorf("ResolveAPIKey = %q, want %q", got, "config-key")
		}
	})

	t.Run("falls back to env", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "env-key")
		cfg := DefaultConfig()
		if got := ResolveAPIKey(cfg); got != "env-key" {
			t.Errorf("ResolveAPIKey = %q, want %q", got, "env-key")
		}
	})

	t.Run("empty when neither set", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		cfg := DefaultConfig()
		if got := ResolveAPIKey(cfg); got != "" {
			t.Errorf("ResolveAPIKey = %q, want empty", got)
		}
	})
}

func TestValidate(t *testing.T) {
	good := DefaultConfig()
	if err := Validate(good); err != nil {
		t.Fatalf("Validate(default) = %v, want nil", err)
	}

	bad := DefaultConfig()
	bad.Rules.OxfordComma = "yes"
	if err := Validate(bad); err == nil {
		t.Errorf("Validate(bad oxford_comma) = nil, want error")
	}

	bad = DefaultConfig()
	bad.Style.Ellipsis = "stars"
	if err := Validate(bad); err == nil {
		t.Errorf("Validate(bad ellipsis) = nil, want error")
	}

	bad = DefaultConfig()
	bad.Style.TimeFormat = "weird"
	if err := Validate(bad); err == nil {
		t.Errorf("Validate(bad time_format) = nil, want error")
	}
}
