package tidy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCallAPI(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotReq apiRequest
		var gotAPIKey, gotVersion, gotContentType string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			gotAPIKey = r.Header.Get("x-api-key")
			gotVersion = r.Header.Get("anthropic-version")
			gotContentType = r.Header.Get("content-type")

			if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
				t.Fatalf("decode request body: %v", err)
			}

			resp := apiResponse{
				Content: []contentBlock{
					{Type: "text", Text: "corrected text"},
				},
			}
			w.Header().Set("content-type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Errorf("encode response: %v", err)
			}
		}))
		defer srv.Close()

		got, err := CallAPI(srv.URL, "test-key", "claude-test", "system prompt", "input text")
		if err != nil {
			t.Fatalf("CallAPI returned error: %v", err)
		}
		if got != "corrected text" {
			t.Errorf("got %q, want %q", got, "corrected text")
		}
		if gotAPIKey != "test-key" {
			t.Errorf("x-api-key = %q, want %q", gotAPIKey, "test-key")
		}
		if gotVersion != "2023-06-01" {
			t.Errorf("anthropic-version = %q, want %q", gotVersion, "2023-06-01")
		}
		if !strings.HasPrefix(gotContentType, "application/json") {
			t.Errorf("content-type = %q, want application/json", gotContentType)
		}
		if gotReq.Model != "claude-test" {
			t.Errorf("model = %q, want %q", gotReq.Model, "claude-test")
		}
		if gotReq.System != "system prompt" {
			t.Errorf("system = %q, want %q", gotReq.System, "system prompt")
		}
		if len(gotReq.Messages) != 1 {
			t.Fatalf("messages len = %d, want 1", len(gotReq.Messages))
		}
		if gotReq.Messages[0].Role != "user" {
			t.Errorf("message role = %q, want %q", gotReq.Messages[0].Role, "user")
		}
		if gotReq.Messages[0].Content != "input text" {
			t.Errorf("message content = %q, want %q", gotReq.Messages[0].Content, "input text")
		}
		if gotReq.MaxTokens != 4096 {
			t.Errorf("max_tokens = %d, want 4096", gotReq.MaxTokens)
		}
	})

	t.Run("http error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"message": "internal server error"},
			}); err != nil {
				t.Errorf("encode response: %v", err)
			}
		}))
		defer srv.Close()

		_, err := CallAPI(srv.URL, "key", "model", "system", "text")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("error %q missing status 500", err.Error())
		}
	})

	t.Run("empty content", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "application/json")
			if err := json.NewEncoder(w).Encode(apiResponse{Content: []contentBlock{}}); err != nil {
				t.Errorf("encode response: %v", err)
			}
		}))
		defer srv.Close()

		_, err := CallAPI(srv.URL, "key", "model", "system", "text")
		if err == nil {
			t.Fatal("expected error for empty content, got nil")
		}
	})

	t.Run("network error", func(t *testing.T) {
		_, err := CallAPI("http://127.0.0.1:1", "key", "model", "system", "text")
		if err == nil {
			t.Fatal("expected error for unreachable server, got nil")
		}
	})
}
