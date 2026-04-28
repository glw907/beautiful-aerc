// SPDX-License-Identifier: MIT

package tidy

import (
	"strconv"
	"strings"
)

// Status codes for Tidy results.
const (
	StatusCorrected    = iota
	StatusNoChanges
	StatusNoAuthorText
	StatusError
)

// Result holds the outcome of a Tidy call.
type Result struct {
	Text    string // the (possibly corrected) text
	Status  int
	Message string // human-readable status for stderr
}

// Tidy proofreads input, preserving quoted lines, and returns a Result.
// It never returns a non-nil error — all failures are captured in Result.Status
// and Result.Message. The error return is reserved for future extensibility.
func Tidy(input string, cfg Config, apiKey, apiURL string) (Result, error) {
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	author, _ := SplitQuoted(input)
	if strings.TrimSpace(author) == "" {
		return Result{
			Text:    input,
			Status:  StatusNoAuthorText,
			Message: "tidytext: no author text found",
		}, nil
	}

	if apiKey == "" {
		return Result{
			Text:    input,
			Status:  StatusError,
			Message: "tidytext: ANTHROPIC_API_KEY not set, text unchanged",
		}, nil
	}

	prompt := BuildPrompt(cfg)

	corrected, err := CallAPI(apiURL, apiKey, cfg.API.Model, prompt, author)
	if err != nil {
		return Result{
			Text:    input,
			Status:  StatusError,
			Message: "tidytext: " + err.Error() + ", text unchanged",
		}, nil
	}

	// Ensure corrected text ends with newline.
	if corrected != "" && !strings.HasSuffix(corrected, "\n") {
		corrected += "\n"
	}

	reassembled := Reassemble(corrected, input)

	if reassembled == input {
		return Result{
			Text:    input,
			Status:  StatusNoChanges,
			Message: "tidytext: no changes needed",
		}, nil
	}

	changed := countChangedLines(input, reassembled)
	return Result{
		Text:    reassembled,
		Status:  StatusCorrected,
		Message: "tidytext: " + strconv.Itoa(changed) + " corrections applied",
	}, nil
}

// countChangedLines returns the number of lines that differ between a and b.
func countChangedLines(a, b string) int {
	aLines := strings.Split(strings.TrimSuffix(a, "\n"), "\n")
	bLines := strings.Split(strings.TrimSuffix(b, "\n"), "\n")

	n := len(aLines)
	if len(bLines) > n {
		n = len(bLines)
	}

	changed := 0
	for i := range n {
		var aLine, bLine string
		if i < len(aLines) {
			aLine = aLines[i]
		}
		if i < len(bLines) {
			bLine = bLines[i]
		}
		if aLine != bLine {
			changed++
		}
	}
	return changed
}
