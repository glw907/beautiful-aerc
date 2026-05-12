package mail

import (
	"errors"
	"io"
	"net"
	"net/url"
	"testing"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

func TestIsConnectionDead(t *testing.T) {
	opErr := &net.OpError{Op: "read", Net: "tcp", Err: errors.New("reset")}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain", errors.New("nope"), false},
		{"eof", io.EOF, true},
		{"unexpected eof", io.ErrUnexpectedEOF, true},
		{"closed pipe", io.ErrClosedPipe, true},
		{"net closed", net.ErrClosed, true},
		{"timeout", timeoutErr{}, true},
		{"op error", opErr, true},
		{"wrapped url.Error around EOF", &url.Error{Op: "Get", URL: "https://x", Err: io.EOF}, true},
		{"wrapped url.Error around benign", &url.Error{Op: "Get", URL: "https://x", Err: errors.New("nope")}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsConnectionDead(c.err); got != c.want {
				t.Fatalf("IsConnectionDead(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
