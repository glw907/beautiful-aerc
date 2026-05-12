package mail

import (
	"errors"
	"testing"
)

func TestProbeResult_OKWhenAllStepsPass(t *testing.T) {
	r := ProbeResult{
		Steps: []ProbeStep{
			{Label: "connect", Status: ProbeOK},
			{Label: "auth", Status: ProbeOK},
		},
	}
	if !r.OK() {
		t.Fatalf("OK() = false, want true")
	}
}

func TestProbeResult_NotOKOnFailedStep(t *testing.T) {
	r := ProbeResult{
		Steps: []ProbeStep{
			{Label: "connect", Status: ProbeOK},
			{Label: "auth", Status: ProbeFail, Detail: "bad creds"},
		},
		Err: errors.New("auth failed"),
	}
	if r.OK() {
		t.Fatalf("OK() = true, want false")
	}
	if r.Err == nil {
		t.Fatalf("Err = nil, want non-nil")
	}
}

func TestProbeResult_NotOKOnErrAlone(t *testing.T) {
	r := ProbeResult{Err: errors.New("dial: i/o timeout")}
	if r.OK() {
		t.Fatalf("OK() = true, want false")
	}
}

func TestIsSelfHosted(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"mail.local", true},
		{"box.home.local", true},
		{"192.168.1.10", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"fd00::1", true},
		{"8.8.8.8", false},
		{"imap.fastmail.com", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			if got := IsSelfHosted(c.host); got != c.want {
				t.Errorf("IsSelfHosted(%q) = %v, want %v", c.host, got, c.want)
			}
		})
	}
}
