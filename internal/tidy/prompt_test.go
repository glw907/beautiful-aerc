// SPDX-License-Identifier: MIT

package tidy

import (
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		contains []string
		absent   []string
	}{
		{
			name: "defaults: all rules present, oxford comma absent (ignore)",
			cfg:  DefaultConfig(),
			contains: []string{
				"You are a proofreader.",
				"Fix misspelled words",
				"Fix grammar errors",
				"Fix whitespace errors",
				"Fix capitalization errors",
				"Fix repeated words",
				"Fix missing punctuation",
				// Punctuation rules (em dash no spaces, ellipsis character)
				"with no spaces",
				`"..." to "…"`,
				// Guardrails
				"Do NOT:",
				"Rephrase or restructure sentences",
				"If the text has no errors, return it exactly as-is.",
			},
			absent: []string{
				// OxfordComma=ignore means no oxford comma instruction
				"Oxford comma",
				// No custom instructions by default
			},
		},
		{
			name: "spelling and grammar disabled",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Rules.Spelling = false
				cfg.Rules.Grammar = false
				return cfg
			}(),
			contains: []string{
				"You are a proofreader.",
				"Fix whitespace errors",
				"Fix capitalization errors",
				"Do NOT:",
				"If the text has no errors, return it exactly as-is.",
			},
			absent: []string{
				"Fix misspelled words",
				"Fix grammar errors",
			},
		},
		{
			name: "oxford comma insert",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Rules.OxfordComma = "insert"
				return cfg
			}(),
			contains: []string{
				"Insert Oxford comma",
			},
			absent: []string{
				"Remove Oxford comma",
			},
		},
		{
			name: "oxford comma remove",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Rules.OxfordComma = "remove"
				return cfg
			}(),
			contains: []string{
				"Remove Oxford comma",
			},
			absent: []string{
				"Insert Oxford comma",
			},
		},
		{
			name: "em dash with spaces",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Rules.Punctuation = true
				cfg.Style.EmDashSpaces = true
				return cfg
			}(),
			contains: []string{
				"with spaces",
			},
			absent: []string{
				"with no spaces",
			},
		},
		{
			name: "em dash with no spaces",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Rules.Punctuation = true
				cfg.Style.EmDashSpaces = false
				return cfg
			}(),
			contains: []string{
				"with no spaces",
			},
			absent: []string{
				"with spaces",
			},
		},
		{
			name: "ellipsis dots style",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Rules.Punctuation = true
				cfg.Style.Ellipsis = "dots"
				return cfg
			}(),
			contains: []string{
				`"…" to "..."`,
			},
			absent: []string{
				`"..." to "…"`,
			},
		},
		{
			name: "ellipsis character style",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Rules.Punctuation = true
				cfg.Style.Ellipsis = "character"
				return cfg
			}(),
			contains: []string{
				`"..." to "…"`,
			},
			absent: []string{
				`"…" to "..."`,
			},
		},
		{
			name: "custom instructions appear in guardrails",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Style.CustomInstructions = []string{
					"change bullet points to numbered lists",
					"alter the paragraph structure",
				}
				return cfg
			}(),
			contains: []string{
				"Do NOT:",
				"change bullet points to numbered lists",
				"alter the paragraph structure",
			},
		},
		{
			name: "all rules disabled: guardrails still present",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Rules.Spelling = false
				cfg.Rules.Grammar = false
				cfg.Rules.Punctuation = false
				cfg.Rules.Whitespace = false
				cfg.Rules.Capitalization = false
				cfg.Rules.RepeatedWords = false
				cfg.Rules.MissingPunctuation = false
				cfg.Rules.OxfordComma = "ignore"
				return cfg
			}(),
			contains: []string{
				"You are a proofreader.",
				"Do NOT:",
				"Rephrase or restructure sentences",
				"If the text has no errors, return it exactly as-is.",
			},
			absent: []string{
				"Fix misspelled words",
				"Fix grammar errors",
				"Fix whitespace errors",
				"Fix capitalization errors",
				"Fix repeated words",
				"Fix missing punctuation",
				"Oxford comma",
			},
		},
		{
			name: "time_format uppercase",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Style.TimeFormat = "uppercase"
				return cfg
			}(),
			contains: []string{
				`"3pm" → "3 PM"`,
				`"10:00 a.m." → "10:00 AM"`,
			},
			absent: []string{
				"10:00am",
				"a.m./p.m.",
			},
		},
		{
			name: "time_format lowercase",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Style.TimeFormat = "lowercase"
				return cfg
			}(),
			contains: []string{
				`"3 PM" → "3pm"`,
				`"10:00 a.m." → "10:00am"`,
				"no space before lowercase am/pm",
			},
		},
		{
			name: "time_format periods",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Style.TimeFormat = "periods"
				return cfg
			}(),
			contains: []string{
				`"3pm" → "3 p.m."`,
				`"10:00 AM" → "10:00 a.m."`,
			},
			absent: []string{
				"no periods",
			},
		},
		{
			name: "time_format ignore: no time formatting instruction",
			cfg:  DefaultConfig(),
			absent: []string{
				"Standardize time formatting",
			},
		},
		{
			name: "punctuation disabled: no em dash or ellipsis instructions",
			cfg: func() Config {
				cfg := DefaultConfig()
				cfg.Rules.Punctuation = false
				return cfg
			}(),
			absent: []string{
				"em dash",
				"ellipsis",
				"en dash",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildPrompt(tc.cfg)
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildPrompt() missing %q\ngot:\n%s", want, got)
				}
			}
			for _, unwanted := range tc.absent {
				if strings.Contains(got, unwanted) {
					t.Errorf("BuildPrompt() should not contain %q\ngot:\n%s", unwanted, got)
				}
			}
		})
	}
}
