package cache

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

func TestAccount_Connected_ReportsBackendPresence(t *testing.T) {
	a := &Account{}
	if a.Connected() {
		t.Fatalf("nil backend: Connected() = true, want false")
	}
	a.Backend = mail.NewMockBackend()
	if !a.Connected() {
		t.Fatalf("non-nil backend: Connected() = false, want true")
	}
}

func TestErrNotConnected_IsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("fetch headers: %w", ErrNotConnected)
	if !errors.Is(wrapped, ErrNotConnected) {
		t.Fatalf("errors.Is wrapped ErrNotConnected = false")
	}
}
