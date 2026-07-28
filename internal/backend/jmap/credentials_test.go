package jmap

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestStaticCredentialsNeverRefresh(t *testing.T) {
	creds := NewStaticCredentials("tok")
	for range 3 {
		token, err := creds.Token(context.Background())
		if err != nil {
			t.Fatalf("Token: %v", err)
		}
		if token != "tok" {
			t.Fatalf("Token = %q, want tok", token)
		}
	}
}

func TestCredentialsRefreshesOnExpiry(t *testing.T) {
	var calls atomic.Int32
	creds := &Credentials{
		token:     "old",
		expiresAt: time.Now().Add(-time.Minute),
		RefreshFunc: func(context.Context) (string, time.Time, error) {
			calls.Add(1)
			return "new", time.Now().Add(time.Hour), nil
		},
	}

	token, err := creds.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token != "new" {
		t.Fatalf("Token = %q, want new", token)
	}
	if calls.Load() != 1 {
		t.Fatalf("RefreshFunc calls = %d, want 1", calls.Load())
	}

	// A second call within the new expiry window must not refresh again.
	if _, err := creds.Token(context.Background()); err != nil {
		t.Fatalf("Token: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("RefreshFunc calls = %d, want 1 (cached)", calls.Load())
	}
}

func TestCredentialsSingleFlightRefresh(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	creds := &Credentials{
		expiresAt: time.Now().Add(-time.Minute),
		RefreshFunc: func(context.Context) (string, time.Time, error) {
			calls.Add(1)
			<-release
			return "new", time.Now().Add(time.Hour), nil
		},
	}

	const waiters = 5
	results := make(chan string, waiters)
	for range waiters {
		go func() {
			token, err := creds.Token(context.Background())
			if err != nil {
				t.Errorf("Token: %v", err)
			}
			results <- token
		}()
	}

	time.Sleep(20 * time.Millisecond) // let every goroutine reach the wait
	close(release)

	for range waiters {
		if got := <-results; got != "new" {
			t.Errorf("Token = %q, want new", got)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("RefreshFunc calls = %d, want 1", calls.Load())
	}
}

func TestCredentialsRefreshError(t *testing.T) {
	wantErr := errors.New("refresh failed")
	creds := &Credentials{
		expiresAt:   time.Now().Add(-time.Minute),
		RefreshFunc: func(context.Context) (string, time.Time, error) { return "", time.Time{}, wantErr },
	}

	_, err := creds.Token(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Token error = %v, want %v", err, wantErr)
	}
}
