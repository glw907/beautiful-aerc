// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"testing"
)

func TestTemplateMatchesGolden(t *testing.T) {
	want, err := os.ReadFile("template.golden")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got := Template()
	if got != string(want) {
		t.Errorf("Template() output drifted from template.golden\n"+
			"len got = %d, len want = %d\n"+
			"(run with -update to regenerate the golden)", len(got), len(want))
	}
}
