// SPDX-License-Identifier: MIT

package tidy

import "strings"

// BuildPrompt constructs a Claude system prompt from the enabled rules and
// style preferences in cfg. Only enabled rules appear in the fix list.
// Guardrail items are always present regardless of which rules are enabled.
func BuildPrompt(cfg Config) string {
	var b strings.Builder

	b.WriteString("You are a proofreader. Fix errors in the text below and return ONLY the corrected text with no commentary, explanations, or markdown formatting.\n\n")

	// Rules section — only emit lines for enabled rules.
	var rules []string

	if cfg.Rules.Spelling {
		rules = append(rules, "Fix misspelled words")
	}
	if cfg.Rules.Grammar {
		rules = append(rules, "Fix grammar errors (to/too, its/it's, their/there/they're, affect/effect, then/than, lose/loose)")
	}
	if cfg.Rules.Punctuation {
		if cfg.Style.EmDashSpaces {
			rules = append(rules, "Convert hyphens used as em dashes to em dashes (—) with spaces ( — )")
		} else {
			rules = append(rules, "Convert hyphens used as em dashes to em dashes (—) with no spaces (—)")
		}
		rules = append(rules, "Convert hyphens used as en dashes to en dashes (–)")
		if cfg.Style.Ellipsis == "dots" {
			rules = append(rules, `Convert ellipsis character (…) to three dots: "…" to "..."`)
		} else {
			rules = append(rules, `Convert three dots to ellipsis character: "..." to "…"`)
		}
	}
	switch cfg.Style.TimeFormat {
	case "uppercase":
		rules = append(rules, `Standardize time formatting: use a space before uppercase AM/PM with no periods (e.g., "3pm" → "3 PM", "10:00 a.m." → "10:00 AM")`)
	case "lowercase":
		rules = append(rules, `Standardize time formatting: use no space before lowercase am/pm with no periods (e.g., "3 PM" → "3pm", "10:00 a.m." → "10:00am")`)
	case "periods":
		rules = append(rules, `Standardize time formatting: use a space before lowercase a.m./p.m. with periods (e.g., "3pm" → "3 p.m.", "10:00 AM" → "10:00 a.m.")`)
	}
	if cfg.Rules.Whitespace {
		rules = append(rules, "Fix whitespace errors (double spaces, trailing spaces)")
	}
	if cfg.Rules.Capitalization {
		rules = append(rules, `Fix capitalization errors (start of sentences, standalone "I")`)
	}
	if cfg.Rules.RepeatedWords {
		rules = append(rules, `Fix repeated words ("the the", "is is")`)
	}
	if cfg.Rules.MissingPunctuation {
		rules = append(rules, "Fix missing punctuation (missing period at end of final sentence, double commas)")
	}
	switch cfg.Rules.OxfordComma {
	case "insert":
		rules = append(rules, "Insert Oxford comma before the conjunction in lists of three or more items")
	case "remove":
		rules = append(rules, "Remove Oxford comma before the conjunction in lists of three or more items")
	}

	if len(rules) > 0 {
		b.WriteString("Fix the following:\n")
		for _, r := range rules {
			b.WriteString("- ")
			b.WriteString(r)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Guardrails — always present.
	b.WriteString("Do NOT:\n")
	guardrails := []string{
		"Rephrase or restructure sentences",
		"Change tone or formality",
		"Expand or contract contractions",
		"Add or remove content",
		"Change the author's voice or style",
		"Modify text inside backtick code spans or code blocks",
	}
	for _, g := range guardrails {
		b.WriteString("- ")
		b.WriteString(g)
		b.WriteString("\n")
	}
	for _, ci := range cfg.Style.CustomInstructions {
		b.WriteString("- ")
		b.WriteString(ci)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString("If the text has no errors, return it exactly as-is.")

	return b.String()
}
