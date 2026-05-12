package mailauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultDevicePollInterval(t *testing.T) {
	if defaultDevicePollInterval != 5*time.Second {
		t.Fatalf("defaultDevicePollInterval = %v, want 5s", defaultDevicePollInterval)
	}
}

func TestAuthorizeDeviceCode_HappyPath(t *testing.T) {
	store := newMemStore()
	var deviceHits, tokenHits atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		deviceHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "dc-xyz",
			"user_code":        "ABCD-1234",
			"verification_uri": "https://example.test/verify",
			"expires_in":       60,
			"interval":         1,
		})
	})
	// First token poll: pending. Second: success.
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		n := tokenHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at-abc",
			"refresh_token": "rt-xyz",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(Config{
		ClientID:      "cid",
		DeviceAuthURL: srv.URL + "/device",
		TokenURL:      srv.URL + "/token",
	}, store, "geoff", BackendKeyring)

	var gotUserCode, gotURI string
	display := func(userCode, uri, _ string) { gotUserCode, gotURI = userCode, uri }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.AuthorizeDeviceCode(ctx, display); err != nil {
		t.Fatalf("AuthorizeDeviceCode: %v", err)
	}
	if gotUserCode != "ABCD-1234" {
		t.Errorf("user_code = %q, want ABCD-1234", gotUserCode)
	}
	if gotURI != "https://example.test/verify" {
		t.Errorf("verification_uri = %q", gotURI)
	}
	if got, _ := store.Get("geoff"); got != "rt-xyz" {
		t.Errorf("stored refresh = %q, want rt-xyz", got)
	}
	if deviceHits.Load() != 1 {
		t.Errorf("device endpoint hits = %d, want 1", deviceHits.Load())
	}
}

func TestAuthorizeDeviceCode_SlowDownBumpsInterval(t *testing.T) {
	store := newMemStore()
	var pollTimes []time.Time
	var hits atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "dc-xyz",
			"user_code":        "WXYZ-9999",
			"verification_uri": "https://example.test/verify",
			"expires_in":       30,
			"interval":         1,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		pollTimes = append(pollTimes, time.Now())
		n := hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "slow_down"})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "at",
				"refresh_token": "rt",
				"expires_in":    3600,
			})
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(Config{
		ClientID:      "cid",
		DeviceAuthURL: srv.URL + "/device",
		TokenURL:      srv.URL + "/token",
	}, store, "u", BackendKeyring)

	// Test the protocol semantics, not the wall clock. pollDeviceToken
	// returns slowDown on the first hit and success on the second; the
	// loop bumps `interval` by 5s. Verifying both hits land suffices.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.AuthorizeDeviceCode(ctx, func(string, string, string) {}) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("AuthorizeDeviceCode: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("AuthorizeDeviceCode did not return")
	}
	if hits.Load() != 2 {
		t.Errorf("poll hits = %d, want 2 (slow_down then success)", hits.Load())
	}
	if len(pollTimes) >= 2 {
		gap := pollTimes[1].Sub(pollTimes[0])
		if gap < 5*time.Second {
			t.Errorf("second poll gap = %v, want >= 5s (slow_down bumps interval)", gap)
		}
	}
}

func TestAuthorizeDeviceCode_UnsupportedFastFails(t *testing.T) {
	c := NewClient(Config{ClientID: "cid"}, newMemStore(), "u", BackendKeyring)
	err := c.AuthorizeDeviceCode(context.Background(), func(string, string, string) {})
	if !errors.Is(err, ErrDeviceCodeUnsupported) {
		t.Fatalf("want ErrDeviceCodeUnsupported, got %v", err)
	}
}

func TestAuthorizeDeviceCode_AccessDenied(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "dc",
			"user_code":        "U",
			"verification_uri": "https://example.test",
			"expires_in":       30,
			"interval":         1,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(Config{
		ClientID:      "cid",
		DeviceAuthURL: srv.URL + "/device",
		TokenURL:      srv.URL + "/token",
	}, newMemStore(), "u", BackendKeyring)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := c.AuthorizeDeviceCode(ctx, func(string, string, string) {})
	if !errors.Is(err, ErrDeviceConsentDenied) {
		t.Errorf("want ErrDeviceConsentDenied, got %v", err)
	}
}

func TestRequestDeviceAuth_FormShape(t *testing.T) {
	cases := []struct {
		name         string
		scopes       []string
		clientSecret string
		wantScope    bool
		wantSecret   bool
	}{
		{"empty-scopes-and-secret", nil, "", false, false},
		{"scopes-only", []string{"a", "b"}, "", true, false},
		{"secret-only", nil, "sek", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotDevice, gotToken string
			mux := http.NewServeMux()
			mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				gotDevice = string(raw)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"device_code":      "dc",
					"user_code":        "U",
					"verification_uri": "https://example.test",
					"expires_in":       30,
					"interval":         1,
				})
			})
			mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				gotToken = string(raw)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token": "at",
					"expires_in":   3600,
				})
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			c := NewClient(Config{
				ClientID:      "cid",
				ClientSecret:  tc.clientSecret,
				DeviceAuthURL: srv.URL + "/device",
				TokenURL:      srv.URL + "/token",
				Scopes:        tc.scopes,
			}, newMemStore(), "u", BackendKeyring)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := c.AuthorizeDeviceCode(ctx, func(string, string, string) {}); err != nil {
				t.Fatalf("AuthorizeDeviceCode: %v", err)
			}
			hasScope := strings.Contains(gotDevice, "scope=")
			if hasScope != tc.wantScope {
				t.Errorf("device form scope= present=%v, want %v: %q", hasScope, tc.wantScope, gotDevice)
			}
			hasSecret := strings.Contains(gotToken, "client_secret=")
			if hasSecret != tc.wantSecret {
				t.Errorf("token form client_secret= present=%v, want %v: %q", hasSecret, tc.wantSecret, gotToken)
			}
		})
	}
}

func TestRequestDeviceAuth_DefaultsExpiresIn(t *testing.T) {
	// expires_in omitted (zero) and negative both floor to 300 (RFC 8628 §3.2).
	for _, want := range []int{0, -1} {
		t.Run(fmt.Sprintf("expires_in_%d", want), func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"device_code":      "dc",
					"user_code":        "U",
					"verification_uri": "https://example.test",
					"expires_in":       want,
					"interval":         1,
				})
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			c := NewClient(Config{
				ClientID:      "cid",
				DeviceAuthURL: srv.URL + "/device",
				TokenURL:      srv.URL + "/token",
			}, newMemStore(), "u", BackendKeyring)

			da, err := c.RequestDeviceAuth(t.Context())
			if err != nil {
				t.Fatalf("RequestDeviceAuth: %v", err)
			}
			if da.ExpiresIn != 300 {
				t.Errorf("ExpiresIn = %d, want 300", da.ExpiresIn)
			}
		})
	}
}

func TestPollDeviceCode_DefaultsInterval(t *testing.T) {
	// Interval=0 must default to defaultDevicePollInterval. Override the
	// package-level default to a small duration so the test is fast; the
	// boundary mutant (interval <= 0 → interval < 0) is killed by the
	// default-branch executing.
	orig := defaultDevicePollInterval
	defaultDevicePollInterval = 20 * time.Millisecond
	defer func() { defaultDevicePollInterval = orig }()

	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at",
			"refresh_token": "rt",
			"expires_in":    3600,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(Config{
		ClientID: "cid",
		TokenURL: srv.URL + "/token",
	}, newMemStore(), "u", BackendKeyring)

	start := time.Now()
	err := c.PollDeviceCode(t.Context(), &DeviceAuth{
		DeviceCode: "dc",
		ExpiresIn:  5,
		Interval:   0,
	})
	if err != nil {
		t.Fatalf("PollDeviceCode: %v", err)
	}
	if hits.Load() != 1 {
		t.Errorf("token hits = %d, want 1", hits.Load())
	}
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Errorf("PollDeviceCode returned in %v, want >= 15ms (the default tick)", elapsed)
	}
}

func TestPollDeviceToken_ExpiryArithmetic(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at",
			"expires_in":   3600,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	before := time.Now()
	tok, slow, err := pollDeviceToken(t.Context(), Config{ClientID: "cid", TokenURL: srv.URL + "/token"}, "dc")
	after := time.Now()
	if err != nil || slow || tok == nil {
		t.Fatalf("pollDeviceToken: tok=%v slow=%v err=%v", tok, slow, err)
	}
	// Expiry must land in [before+3600s, after+3600s]. Pins both the
	// multiplication and the boundary check (ExpiresIn > 0).
	if tok.Expiry.Before(before.Add(3600*time.Second)) || tok.Expiry.After(after.Add(3600*time.Second)) {
		t.Errorf("Expiry = %v, want in [%v, %v]", tok.Expiry,
			before.Add(3600*time.Second), after.Add(3600*time.Second))
	}
}

func TestPollDeviceToken_NoExpiryWhenZero(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at",
			// expires_in omitted → zero, must leave Expiry zero.
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tok, _, err := pollDeviceToken(t.Context(), Config{ClientID: "cid", TokenURL: srv.URL + "/token"}, "dc")
	if err != nil || tok == nil {
		t.Fatalf("pollDeviceToken: tok=%v err=%v", tok, err)
	}
	if !tok.Expiry.IsZero() {
		t.Errorf("Expiry = %v, want zero", tok.Expiry)
	}
}

func TestAuthorizeDeviceCode_PostsExpectedFields(t *testing.T) {
	var gotForm string
	mux := http.NewServeMux()
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotForm = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "dc",
			"user_code":        "U",
			"verification_uri": "https://example.test",
			"expires_in":       30,
			"interval":         1,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at",
			"refresh_token": "rt",
			"expires_in":    3600,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(Config{
		ClientID:      "test-cid",
		DeviceAuthURL: srv.URL + "/device",
		TokenURL:      srv.URL + "/token",
		Scopes:        []string{"mail.read", "offline_access"},
	}, newMemStore(), "u", BackendKeyring)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.AuthorizeDeviceCode(ctx, func(string, string, string) {}); err != nil {
		t.Fatalf("AuthorizeDeviceCode: %v", err)
	}
	if !strings.Contains(gotForm, "client_id=test-cid") {
		t.Errorf("device POST missing client_id: %q", gotForm)
	}
	if !strings.Contains(gotForm, "mail.read") || !strings.Contains(gotForm, "offline_access") {
		t.Errorf("device POST missing scopes: %q", gotForm)
	}
}
