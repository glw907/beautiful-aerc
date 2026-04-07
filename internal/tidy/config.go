package tidy

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// APIConfig holds Anthropic API settings.
type APIConfig struct {
	Model  string `toml:"model"`
	APIKey string `toml:"api_key"`
}

// RulesConfig holds grammar and style rule toggles.
type RulesConfig struct {
	Spelling           bool   `toml:"spelling"`
	Grammar            bool   `toml:"grammar"`
	Punctuation        bool   `toml:"punctuation"`
	Whitespace         bool   `toml:"whitespace"`
	Capitalization     bool   `toml:"capitalization"`
	RepeatedWords      bool   `toml:"repeated_words"`
	MissingPunctuation bool   `toml:"missing_punctuation"`
	OxfordComma        string `toml:"oxford_comma"`
}

// StyleConfig holds prose style preferences.
type StyleConfig struct {
	EmDashSpaces       bool     `toml:"em_dash_spaces"`
	Ellipsis           string   `toml:"ellipsis"`
	CustomInstructions []string `toml:"custom_instructions"`
}

// Config holds the full tidytext configuration.
type Config struct {
	API   APIConfig   `toml:"api"`
	Rules RulesConfig `toml:"rules"`
	Style StyleConfig `toml:"style"`
}

// DefaultConfig returns a Config with all default values applied.
func DefaultConfig() Config {
	return Config{
		API: APIConfig{
			Model: "claude-haiku-4-5-20251001",
		},
		Rules: RulesConfig{
			Spelling:           true,
			Grammar:            true,
			Punctuation:        true,
			Whitespace:         true,
			Capitalization:     true,
			RepeatedWords:      true,
			MissingPunctuation: true,
			OxfordComma:        "ignore",
		},
		Style: StyleConfig{
			EmDashSpaces: false,
			Ellipsis:     "character",
		},
	}
}

// LoadConfig reads a TOML config file at path and merges it with defaults.
// If the file does not exist, defaults are returned without error.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	if _, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if err := validateOxfordComma(cfg.Rules.OxfordComma); err != nil {
		return Config{}, err
	}
	if err := validateEllipsis(cfg.Style.Ellipsis); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// ApplyRuleOverrides applies "key=value" overrides to the Rules section of cfg.
// Unknown keys or bad formats return an error.
func ApplyRuleOverrides(cfg *Config, overrides []string) error {
	for _, o := range overrides {
		key, val, ok := strings.Cut(o, "=")
		if !ok {
			return fmt.Errorf("rule override %q: missing '='", o)
		}
		var dst *bool
		switch key {
		case "spelling":
			dst = &cfg.Rules.Spelling
		case "grammar":
			dst = &cfg.Rules.Grammar
		case "punctuation":
			dst = &cfg.Rules.Punctuation
		case "whitespace":
			dst = &cfg.Rules.Whitespace
		case "capitalization":
			dst = &cfg.Rules.Capitalization
		case "repeated_words":
			dst = &cfg.Rules.RepeatedWords
		case "missing_punctuation":
			dst = &cfg.Rules.MissingPunctuation
		case "oxford_comma":
			if err := validateOxfordComma(val); err != nil {
				return err
			}
			cfg.Rules.OxfordComma = val
			continue
		default:
			return fmt.Errorf("rule override: unknown key %q", key)
		}
		if err := setBool(dst, key, val); err != nil {
			return err
		}
	}
	return nil
}

// ApplyStyleOverrides applies "key=value" overrides to the Style section of cfg.
// Unknown keys or bad formats return an error.
func ApplyStyleOverrides(cfg *Config, overrides []string) error {
	for _, o := range overrides {
		key, val, ok := strings.Cut(o, "=")
		if !ok {
			return fmt.Errorf("style override %q: missing '='", o)
		}
		switch key {
		case "em_dash_spaces":
			if err := setBool(&cfg.Style.EmDashSpaces, key, val); err != nil {
				return err
			}
		case "ellipsis":
			if err := validateEllipsis(val); err != nil {
				return err
			}
			cfg.Style.Ellipsis = val
		default:
			return fmt.Errorf("style override: unknown key %q", key)
		}
	}
	return nil
}

// ResolveAPIKey returns the effective API key. The config api_key takes
// precedence over the ANTHROPIC_API_KEY environment variable.
func ResolveAPIKey(cfg Config) string {
	if cfg.API.APIKey != "" {
		return cfg.API.APIKey
	}
	return os.Getenv("ANTHROPIC_API_KEY")
}

// ConfigString returns a human-readable representation of cfg.
func ConfigString(cfg Config) string {
	var b strings.Builder
	fmt.Fprintf(&b, "API:\n")
	fmt.Fprintf(&b, "  model:    %s\n", cfg.API.Model)
	if cfg.API.APIKey != "" {
		fmt.Fprintf(&b, "  api_key:  (set)\n")
	} else {
		fmt.Fprintf(&b, "  api_key:  (not set)\n")
	}
	fmt.Fprintf(&b, "Rules:\n")
	fmt.Fprintf(&b, "  spelling:            %v\n", cfg.Rules.Spelling)
	fmt.Fprintf(&b, "  grammar:             %v\n", cfg.Rules.Grammar)
	fmt.Fprintf(&b, "  punctuation:         %v\n", cfg.Rules.Punctuation)
	fmt.Fprintf(&b, "  whitespace:          %v\n", cfg.Rules.Whitespace)
	fmt.Fprintf(&b, "  capitalization:      %v\n", cfg.Rules.Capitalization)
	fmt.Fprintf(&b, "  repeated_words:      %v\n", cfg.Rules.RepeatedWords)
	fmt.Fprintf(&b, "  missing_punctuation: %v\n", cfg.Rules.MissingPunctuation)
	fmt.Fprintf(&b, "  oxford_comma:        %s\n", cfg.Rules.OxfordComma)
	fmt.Fprintf(&b, "Style:\n")
	fmt.Fprintf(&b, "  em_dash_spaces:      %v\n", cfg.Style.EmDashSpaces)
	fmt.Fprintf(&b, "  ellipsis:            %s\n", cfg.Style.Ellipsis)
	if len(cfg.Style.CustomInstructions) > 0 {
		fmt.Fprintf(&b, "  custom_instructions:\n")
		for _, s := range cfg.Style.CustomInstructions {
			fmt.Fprintf(&b, "    - %s\n", s)
		}
	}
	return b.String()
}

func validateOxfordComma(v string) error {
	switch v {
	case "insert", "remove", "ignore":
		return nil
	default:
		return fmt.Errorf("config: oxford_comma must be insert, remove, or ignore; got %q", v)
	}
}

func validateEllipsis(v string) error {
	switch v {
	case "character", "dots":
		return nil
	default:
		return fmt.Errorf("config: ellipsis must be character or dots; got %q", v)
	}
}

func setBool(dst *bool, key, val string) error {
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fmt.Errorf("override %q: invalid bool value %q", key, val)
	}
	*dst = b
	return nil
}
