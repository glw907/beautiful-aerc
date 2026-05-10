package config

import (
	"errors"
	"strings"
	"testing"
)

func TestConfigError_FormatsAllFields(t *testing.T) {
	e := &ConfigError{
		Path:    "/home/user/.config/poplar/config.toml",
		Line:    42,
		Account: "fastmail",
		Field:   "email",
		Message: "field is required",
		Suggest: `add "email = ..." under the [[account]] block`,
	}
	got := e.Error()
	for _, want := range []string{
		"config.toml:42",
		`account "fastmail"`,
		`field "email"`,
		"field is required",
		`fix: add "email`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q, missing %q", got, want)
		}
	}
}

func TestConfigError_OmitsBlankSegments(t *testing.T) {
	e := &ConfigError{Field: "email", Message: "required"}
	got := e.Error()
	if strings.Contains(got, "account") {
		t.Errorf("Error() = %q, should not mention account when blank", got)
	}
	if strings.HasPrefix(got, ":") {
		t.Errorf("Error() = %q, leading colon when path/line absent", got)
	}
}

func TestConfigError_IsErrConfigInvalid(t *testing.T) {
	e := &ConfigError{Field: "email", Message: "required"}
	if !errors.Is(e, ErrConfigInvalid) {
		t.Errorf("errors.Is(e, ErrConfigInvalid) = false, want true")
	}
}
