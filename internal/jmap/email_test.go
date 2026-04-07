package jmap

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestQueryInbox(t *testing.T) {
	s := testSession(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MethodCalls []json.RawMessage `json:"methodCalls"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		resp := map[string]any{
			"methodResponses": []any{
				[]any{"Mailbox/query", map[string]any{
					"ids": []string{"mb-inbox"},
				}, "0"},
				[]any{"Email/query", map[string]any{
					"ids":   []string{"em1", "em2", "em3"},
					"total": 3,
				}, "1"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	ids, err := QueryInbox(s, "from:test@example.com")
	if err != nil {
		t.Fatalf("QueryInbox: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 ids, got %d", len(ids))
	}
}

func TestMoveEmails(t *testing.T) {
	called := false
	s := testSession(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		resp := map[string]any{
			"methodResponses": []any{
				[]any{"Email/set", map[string]any{
					"updated": map[string]any{
						"em1": nil,
						"em2": nil,
					},
				}, "0"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	err := MoveEmails(s, []string{"em1", "em2"}, "mb-inbox", "mb-dest")
	if err != nil {
		t.Fatalf("MoveEmails: %v", err)
	}
	if !called {
		t.Error("API not called")
	}
}
