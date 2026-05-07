package tidy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mockAPI(t *testing.T, corrected string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Content: []contentBlock{{Type: "text", Text: corrected}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestTidy_BasicCorrection(t *testing.T) {
	input := "i went to the the store.\n"
	corrected := "I went to the store.\n"

	srv := mockAPI(t, corrected)
	defer srv.Close()

	cfg := DefaultConfig()
	result, err := Tidy(input, cfg, "test-key", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusCorrected {
		t.Errorf("status = %d, want StatusCorrected (%d)", result.Status, StatusCorrected)
	}
	if result.Text != corrected {
		t.Errorf("text = %q, want %q", result.Text, corrected)
	}
	if !strings.Contains(result.Message, "corrections applied") {
		t.Errorf("message = %q, want to contain 'corrections applied'", result.Message)
	}
}

func TestTidy_NoChanges(t *testing.T) {
	input := "I went to the store.\n"

	srv := mockAPI(t, input)
	defer srv.Close()

	cfg := DefaultConfig()
	result, err := Tidy(input, cfg, "test-key", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusNoChanges {
		t.Errorf("status = %d, want StatusNoChanges (%d)", result.Status, StatusNoChanges)
	}
	if result.Text != input {
		t.Errorf("text = %q, want %q", result.Text, input)
	}
	if result.Message != "tidytext: no changes needed" {
		t.Errorf("message = %q, want 'tidytext: no changes needed'", result.Message)
	}
}

func TestTidy_PreservesQuotedText(t *testing.T) {
	input := "> quoted line\ni made a eror here.\n"
	authorCorrected := "\nI made an error here.\n"

	srv := mockAPI(t, authorCorrected)
	defer srv.Close()

	cfg := DefaultConfig()
	result, err := Tidy(input, cfg, "test-key", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusCorrected {
		t.Errorf("status = %d, want StatusCorrected (%d)", result.Status, StatusCorrected)
	}
	if !strings.Contains(result.Text, "> quoted line") {
		t.Errorf("quoted text missing from result: %q", result.Text)
	}
}

func TestTidy_AllQuoted(t *testing.T) {
	input := "> line one\n> line two\n"

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	result, err := Tidy(input, cfg, "test-key", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusNoAuthorText {
		t.Errorf("status = %d, want StatusNoAuthorText (%d)", result.Status, StatusNoAuthorText)
	}
	if result.Text != input {
		t.Errorf("text = %q, want original %q", result.Text, input)
	}
	if result.Message != "tidytext: no author text found" {
		t.Errorf("message = %q, want 'tidytext: no author text found'", result.Message)
	}
	if called {
		t.Error("API should not be called when there is no author text")
	}
}

func TestTidy_MissingAPIKey(t *testing.T) {
	input := "some text here.\n"

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	result, err := Tidy(input, cfg, "", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusError {
		t.Errorf("status = %d, want StatusError (%d)", result.Status, StatusError)
	}
	if result.Text != input {
		t.Errorf("text = %q, want original %q", result.Text, input)
	}
	if result.Message != "tidytext: ANTHROPIC_API_KEY not set, text unchanged" {
		t.Errorf("message = %q, want API key error message", result.Message)
	}
	if called {
		t.Error("API should not be called when apiKey is empty")
	}
}

func TestTidy_APIError(t *testing.T) {
	input := "some text here.\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"internal server error"}}`))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	result, err := Tidy(input, cfg, "test-key", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusError {
		t.Errorf("status = %d, want StatusError (%d)", result.Status, StatusError)
	}
	if result.Text != input {
		t.Errorf("text = %q, want original %q", result.Text, input)
	}
	if !strings.HasPrefix(result.Message, "tidytext: ") {
		t.Errorf("message = %q, want prefix 'tidytext: '", result.Message)
	}
	if !strings.HasSuffix(result.Message, ", text unchanged") {
		t.Errorf("message = %q, want suffix ', text unchanged'", result.Message)
	}
}
