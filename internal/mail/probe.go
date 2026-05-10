package mail

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
