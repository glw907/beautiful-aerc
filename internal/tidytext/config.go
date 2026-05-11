package tidytext

import (
	"errors"
	"fmt"
	"os"
	"strings"
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
	TimeFormat         string   `toml:"time_format"`
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
			TimeFormat:   "ignore",
		},
	}
}

// ResolveAPIKey returns the effective API key. The config api_key takes
// precedence over the ANTHROPIC_API_KEY environment variable.
func ResolveAPIKey(cfg Config) string {
	if cfg.API.APIKey != "" {
		return cfg.API.APIKey
	}
	return os.Getenv("ANTHROPIC_API_KEY")
}

func validateEnum(field, v string, valid ...string) error {
	for _, s := range valid {
		if v == s {
			return nil
		}
	}
	return fmt.Errorf("config: %s must be one of %s; got %q", field, strings.Join(valid, ", "), v)
}

func validateOxfordComma(v string) error {
	return validateEnum("oxford_comma", v, "insert", "remove", "ignore")
}

func validateEllipsis(v string) error {
	return validateEnum("ellipsis", v, "character", "dots")
}

func validateTimeFormat(v string) error {
	return validateEnum("time_format", v, "uppercase", "lowercase", "periods", "ignore")
}

// Validate reports whether c's string-enum fields hold legal values.
func Validate(c Config) error {
	return errors.Join(
		validateOxfordComma(c.Rules.OxfordComma),
		validateEllipsis(c.Style.Ellipsis),
		validateTimeFormat(c.Style.TimeFormat),
	)
}
