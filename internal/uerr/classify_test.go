package uerr_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
)

// TestClassifyErrPrecedence pins ClassifyErr's contract: the
// Classified error closest to err itself wins, whatever a Classified
// error nested deeper beneath it carries. No current producer builds
// a tree with two Classified errors, but both nestings below are
// constructible, and the walk order errors.AsType documents,
// depth-first from err itself rather than from the leaves up, means
// the outer one is always the one ClassifyErr reports.
func TestClassifyErrPrecedence(t *testing.T) {
	uerrtest.Capture(t)

	failure := backend.Failure{Class: uerr.ClassServer, Cause: errors.New("db unavailable")}
	wrapped := uerr.New("uerr.test", nil, uerr.ClassAuth, errors.New("token rejected"))

	tests := []struct {
		name      string
		outer     error
		wantClass uerr.Class
		wantCause error
	}{
		{
			name:      "Error wrapping Failure: the outer uerr.Error wins",
			outer:     uerr.New("uerr.test", nil, uerr.ClassAuth, failure),
			wantClass: uerr.ClassAuth,
			wantCause: failure,
		},
		{
			name:      "Failure wrapping Error: the outer backend.Failure wins",
			outer:     backend.Failure{Class: uerr.ClassServer, Cause: wrapped},
			wantClass: uerr.ClassServer,
			wantCause: wrapped,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class, cause := uerr.ClassifyErr(tt.outer, uerr.ClassConnection)
			if class != tt.wantClass {
				t.Errorf("Class = %v, want %v", class, tt.wantClass)
			}
			if !reflect.DeepEqual(cause, tt.wantCause) {
				t.Errorf("Cause = %v, want %v", cause, tt.wantCause)
			}
		})
	}
}
