package mail

import (
	"net"
	"strings"
)

// IsSelfHosted reports whether host looks like a self-hosted endpoint
// where a self-signed TLS cert is plausible: RFC 1918 / IPv6 ULA,
// loopback, or a `.local` mDNS name. Used by both the connect-error
// hint in mailimap and the wizard's conditional "skip TLS verify"
// prompt.
func IsSelfHosted(host string) bool {
	if strings.HasSuffix(host, ".local") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

// ProbeStatus is the outcome of one probe step.
type ProbeStatus int

const (
	ProbeOK ProbeStatus = iota + 1
	ProbeFail
)

// ProbeStep is one entry in a probe transcript. Label is shown to
// the user; Detail carries an optional measurement on success
// ("1,247 messages") or a server-side reason on failure.
type ProbeStep struct {
	Label  string
	Status ProbeStatus
	Detail string
}

// ProbeResult records the step-by-step outcome of a connect-test
// against a Backend. Err is set when any step's Status is ProbeFail.
type ProbeResult struct {
	Steps []ProbeStep
	Err   error
}

// OK reports whether the probe completed without a failed step.
func (r ProbeResult) OK() bool {
	if r.Err != nil {
		return false
	}
	for _, s := range r.Steps {
		if s.Status == ProbeFail {
			return false
		}
	}
	return true
}
