package mailauth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestConsentServer_PassesError(t *testing.T) {
	srv, redirect, done, err := runConsentServer([2]int{0, 0})
	if err != nil {
		t.Fatalf("runConsentServer: %v", err)
	}
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	resp, err := http.Get(redirect + "?error=access_denied")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	resp.Body.Close()

	select {
	case res := <-done:
		if res.err == nil {
			t.Fatal("res.err = nil, want non-nil")
		}
		if !strings.Contains(res.err.Error(), "oauth callback: access_denied") {
			t.Errorf("err = %q, want it to wrap %q", res.err, "oauth callback: access_denied")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("consent channel never fired")
	}
}

func TestConsentServer_PassesCode(t *testing.T) {
	srv, redirect, done, err := runConsentServer([2]int{0, 0})
	if err != nil {
		t.Fatalf("runConsentServer: %v", err)
	}
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	resp, err := http.Get(redirect + "?code=THECODE&state=THESTATE")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	resp.Body.Close()

	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("err = %v, want nil", res.err)
		}
		if res.code != "THECODE" {
			t.Errorf("code = %q, want THECODE", res.code)
		}
		if res.state != "THESTATE" {
			t.Errorf("state = %q, want THESTATE", res.state)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("consent channel never fired")
	}
}

func TestConsentServer_Timeouts(t *testing.T) {
	srv, _, _, err := runConsentServer([2]int{0, 0})
	if err != nil {
		t.Fatalf("runConsentServer: %v", err)
	}
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	if srv.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 10s", srv.ReadHeaderTimeout)
	}
	if srv.WriteTimeout != 5*time.Second {
		t.Errorf("WriteTimeout = %v, want 5s", srv.WriteTimeout)
	}
}

func TestListen_ScansPortRange(t *testing.T) {
	// Occupy a port, then ask listen() to scan a two-port range that
	// starts on the busy port. Exercises the loop's second iteration.
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("seed listener: %v", err)
	}
	defer busy.Close()
	busyPort := busy.Addr().(*net.TCPAddr).Port

	ln, err := listen([2]int{busyPort, busyPort + 1})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	got := ln.Addr().(*net.TCPAddr).Port
	if got != busyPort+1 {
		t.Errorf("listen port = %d, want %d (skipping busy %d)", got, busyPort+1, busyPort)
	}
}

func TestListen_ExhaustedRange(t *testing.T) {
	// Pins the loop's terminating boundary (the `<=` mutant) by
	// forcing iteration to end without finding a free port.
	a, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen a: %v", err)
	}
	defer a.Close()
	port := a.Addr().(*net.TCPAddr).Port

	_, err = listen([2]int{port, port})
	if err == nil {
		t.Fatal("listen err = nil, want exhaustion error")
	}
	want := fmt.Sprintf("no free port in range %d–%d", port, port)
	if !strings.Contains(err.Error(), want) {
		t.Errorf("err = %q, want it to contain %q", err, want)
	}
}
