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
