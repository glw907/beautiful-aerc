package jmap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing auth header")
		}
		w.Write([]byte(`{
			"primaryAccounts": {
				"urn:ietf:params:jmap:mail": "acct-123"
			},
			"apiUrl": "https://api.example.com/jmap/api/"
		}`))
	}))
	defer srv.Close()

	s, err := NewSession("test-token", srv.URL)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if s.AccountID != "acct-123" {
		t.Errorf("AccountID = %q, want %q", s.AccountID, "acct-123")
	}
	if s.APIURL != "https://api.example.com/jmap/api/" {
		t.Errorf("APIURL = %q", s.APIURL)
	}
}

func TestNewSessionBadToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	_, err := NewSession("bad-token", srv.URL)
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestCallWith(t *testing.T) {
	var gotUsing []string
	s := testSession(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Using []string `json:"using"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		gotUsing = body.Using
		w.Write([]byte(`{"methodResponses":[]}`))
	})

	caps := []string{
		"urn:ietf:params:jmap:core",
		"https://www.fastmail.com/dev/maskedemail",
	}
	_, err := s.CallWith(caps, nil)
	if err != nil {
		t.Fatalf("CallWith: %v", err)
	}
	if len(gotUsing) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(gotUsing))
	}
	if gotUsing[1] != "https://www.fastmail.com/dev/maskedemail" {
		t.Errorf("using[1] = %q, want maskedemail capability", gotUsing[1])
	}
}
