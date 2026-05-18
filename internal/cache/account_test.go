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

func TestOpen_NoBackend_SucceedsAndDeferredWire(t *testing.T) {
	dir := t.TempDir()
	a, err := Open("Test", dir, Config{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	if a.Connected() {
		t.Fatalf("Connected() after Open without WireBackend = true, want false")
	}
	if got := a.AccountName(); got != "Test" {
		t.Fatalf("AccountName() pre-wire = %q, want %q", got, "Test")
	}
	if got := a.AccountEmail(); got != "" {
		t.Fatalf("AccountEmail() pre-wire = %q, want \"\"", got)
	}
}

func TestWireBackend_AssignsAndStartsBackfiller(t *testing.T) {
	dir := t.TempDir()
	a, err := Open("Test", dir, Config{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	be := &fakeBackend{folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}}}
	ct := &fakeChangeTracker{}
	if err := a.WireBackend(be, ct); err != nil {
		t.Fatalf("WireBackend: %v", err)
	}
	if !a.Connected() {
		t.Fatalf("Connected() after WireBackend = false")
	}
	if err := a.WireBackend(be, ct); err == nil {
		t.Fatalf("second WireBackend = nil, want error")
	}
}

func TestWireBackend_StartsDrainer(t *testing.T) {
	dir := t.TempDir()
	a, err := Open("Test", dir, Config{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	be := &fakeBackend{folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}}}
	ct := &fakeChangeTracker{}
	if err := a.WireBackend(be, ct); err != nil {
		t.Fatalf("WireBackend: %v", err)
	}
	// Shape check: WireBackend must have stored a cancel for the drainer context.
	// End-to-end drain behavior is validated by TestIntegration_TriageRoundTrip.
	if a.drainerStop == nil {
		t.Fatal("drainerStop is nil after WireBackend; drainer was not started")
	}
}

func TestOpen_AccountName_FromName(t *testing.T) {
	dir := t.TempDir()
	a, err := Open("MyAccount", dir, Config{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	if got := a.AccountName(); got != "MyAccount" {
		t.Fatalf("AccountName() = %q, want %q", got, "MyAccount")
	}
	be := &fakeBackend{}
	ct := &fakeChangeTracker{}
	if err := a.WireBackend(be, ct); err != nil {
		t.Fatalf("WireBackend: %v", err)
	}
	if got := a.AccountName(); got != "MyAccount" {
		t.Fatalf("AccountName() post-wire = %q, want %q", got, "MyAccount")
	}
}
