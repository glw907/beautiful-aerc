package contacts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, "u", "p", false)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPutAddressObject_Success(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %q want PUT", r.Method)
		}
		if got := r.Header.Get("If-Match"); got != `"old"` {
			t.Errorf("If-Match = %q want %q", got, `"old"`)
		}
		w.Header().Set("ETag", `"new"`)
		w.WriteHeader(http.StatusCreated)
	}))
	href, etag, err := c.PutAddressObject(context.Background(),
		"/addressbooks/u/default/u1.vcf", `"old"`,
		[]byte("BEGIN:VCARD\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(href, "/u1.vcf") || etag != `"new"` {
		t.Errorf("got href=%q etag=%q", href, etag)
	}
}

func TestPutAddressObject_PreconditionFailed(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
	}))
	_, _, err := c.PutAddressObject(context.Background(), "/x.vcf", `"e"`, []byte("x"))
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("err = %v want ErrPreconditionFailed", err)
	}
}

func TestPutAddressObject_Auth(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	_, _, err := c.PutAddressObject(context.Background(), "/x.vcf", "", []byte("x"))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err = %v want ErrAuth", err)
	}
}

func TestDeleteAddressObject_NotFound(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	err := c.DeleteAddressObject(context.Background(), "/x.vcf", `"e"`)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v want ErrNotFound", err)
	}
}

func TestDeleteAddressObject_Auth(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	err := c.DeleteAddressObject(context.Background(), "/x.vcf", "")
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err = %v want ErrAuth", err)
	}
}
