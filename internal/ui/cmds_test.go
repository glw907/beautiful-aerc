package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeAttachFilename(t *testing.T) {
	cases := []struct {
		name, partID, want string
	}{
		{"report.pdf", "2", "report.pdf"},
		{"", "2.1", "attachment-2.1"},
		{"a/b/c.txt", "1", "a_b_c.txt"},
		{"  spaced.bin  ", "3", "spaced.bin"},
	}
	for _, c := range cases {
		if got := sanitizeAttachFilename(c.name, c.partID); got != c.want {
			t.Errorf("sanitize(%q, %q) = %q, want %q", c.name, c.partID, got, c.want)
		}
	}
}

func TestResolveSaveTarget_Collision(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.pdf"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a-1.pdf"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := resolveSaveTarget(dir, "a.pdf")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := filepath.Join(dir, "a-2.pdf")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveSaveTarget_Fresh(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveSaveTarget(dir, "fresh.bin")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != filepath.Join(dir, "fresh.bin") {
		t.Errorf("got %q", got)
	}
}

func TestUnsubscribePostCmd(t *testing.T) {
	t.Run("2xx success", func(t *testing.T) {
		var got string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			got = string(body)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		msg := unsubscribePostCmd(srv.URL)()
		done, ok := msg.(UnsubscribeDoneMsg)
		if !ok {
			t.Fatalf("got %T, want UnsubscribeDoneMsg", msg)
		}
		if got != "List-Unsubscribe=One-Click" {
			t.Errorf("body = %q, want %q", got, "List-Unsubscribe=One-Click")
		}
		if done.Host == "" {
			t.Error("Host empty")
		}
	})

	t.Run("non-2xx surfaces ErrorMsg", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		msg := unsubscribePostCmd(srv.URL)()
		if _, ok := msg.(ErrorMsg); !ok {
			t.Fatalf("got %T, want ErrorMsg", msg)
		}
	})

	t.Run("network failure surfaces ErrorMsg", func(t *testing.T) {
		msg := unsubscribePostCmd("http://127.0.0.1:1")()
		if _, ok := msg.(ErrorMsg); !ok {
			t.Fatalf("got %T, want ErrorMsg", msg)
		}
	})
}
