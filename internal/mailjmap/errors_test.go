package mailjmap

import (
	"errors"
	"io"
	"net"
	"net/url"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"

	"github.com/glw907/poplar/internal/mail"
)

func TestClassifyErr(t *testing.T) {
	tests := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"auth401", &jmap.RequestError{Status: 401}, mail.ErrAuth},
		{"auth403", &jmap.RequestError{Status: 403}, mail.ErrAuth},
		{"notfound", &jmap.RequestError{Status: 404}, mail.ErrNotFound},
		{"eof", io.EOF, mail.ErrConnection},
		{"closed-pipe", io.ErrClosedPipe, mail.ErrConnection},
		{"net-closed", net.ErrClosed, mail.ErrConnection},
		{"url-wrapping-eof", &url.Error{Op: "Post", URL: "https://x", Err: io.EOF}, mail.ErrConnection},
		{"op-error", &net.OpError{Op: "read"}, mail.ErrConnection},
		{"unrelated", errors.New("boom"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyErr(tt.in)
			if tt.want == nil {
				if got != tt.in {
					t.Fatalf("classifyErr(%v) = %v, want pass-through %v", tt.in, got, tt.in)
				}
				return
			}
			if !errors.Is(got, tt.want) {
				t.Fatalf("classifyErr(%v) = %v, want errors.Is %v", tt.in, got, tt.want)
			}
		})
	}
}
