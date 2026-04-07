package tidy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultAPIURL = "https://api.anthropic.com/v1/messages"

var httpClient = &http.Client{Timeout: 30 * time.Second}

type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system"`
	Messages  []apiMessage `json:"messages"`
}

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiResponse struct {
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type apiError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// CallAPI sends text to the Claude messages API and returns the response text.
func CallAPI(url, apiKey, model, systemPrompt, text string) (string, error) {
	reqBody := apiRequest{
		Model:     model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []apiMessage{
			{Role: "user", Content: text},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr apiError
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("API returned empty content")
	}

	return apiResp.Content[0].Text, nil
}
