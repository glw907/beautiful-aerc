package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// llmStat holds the outcome of one LLM rendering call.
type llmStat struct {
	MS     int64  `json:"ms"`
	Tokens int    `json:"tokens"`
	Error  string `json:"error,omitempty"`
}

// promptTemplate is the structure saved to corpus/llm-prompt.md.
// {{PRINCIPLES}} and {{EMAIL}} mark where dynamic content is substituted.
const promptTemplate = `Apply the following rendering principles when converting the email to markdown:

{{PRINCIPLES}}

---

Render the email below to markdown following all principles above.
Output only the rendered markdown. No preamble, no commentary, no sign-off.

{{EMAIL}}
`

// buildPrompt returns the full prompt for one message by substituting
// the principles document and the email body into promptTemplate.
func buildPrompt(principles, emailContent string) string {
	p := strings.Replace(promptTemplate, "{{PRINCIPLES}}", principles, 1)
	return strings.Replace(p, "{{EMAIL}}", emailContent, 1)
}

// tokenEst estimates the output token count as len(output)/4.
func tokenEst(output string) int {
	return len(output) / 4
}

const llmTimeout = 3 * time.Minute

// runLLM calls claude -p with the given model, passing prompt on stdin.
// It returns the rendered markdown, wall milliseconds, and any error.
func runLLM(ctx context.Context, prompt, model string) (string, int64, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", model)
	cmd.Stdin = bytes.NewBufferString(prompt)
	out, err := cmd.Output()
	ms := time.Since(start).Milliseconds()
	if err != nil {
		return "", ms, fmt.Errorf("claude: %w", err)
	}
	return string(out), ms, nil
}

// mergeEntry adds or overwrites the "llm" key in an existing message JSON object.
func mergeEntry(existing json.RawMessage, stat llmStat) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(existing, &obj); err != nil {
		return nil, fmt.Errorf("unmarshal entry: %w", err)
	}

	statBytes, err := json.Marshal(stat)
	if err != nil {
		return nil, fmt.Errorf("marshal llm stat: %w", err)
	}
	obj["llm"] = statBytes

	out, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal entry: %w", err)
	}
	return out, nil
}
