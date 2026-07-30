package pkgrole

import "testing"

func TestOf(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		wantRole       string
		wantModuleRoot string
		wantOK         bool
	}{
		{
			name:           "internal package",
			path:           "github.com/glw907/poplar/internal/ui",
			wantRole:       "ui",
			wantModuleRoot: "github.com/glw907/poplar",
			wantOK:         true,
		},
		{
			name:           "nested internal package",
			path:           "github.com/glw907/poplar/internal/backend/jmapsource",
			wantRole:       "backend",
			wantModuleRoot: "github.com/glw907/poplar",
			wantOK:         true,
		},
		{
			name:           "cmd package",
			path:           "github.com/glw907/poplar/cmd/poplar",
			wantRole:       "poplar",
			wantModuleRoot: "github.com/glw907/poplar",
			wantOK:         true,
		},
		{
			name:           "external test package",
			path:           "github.com/glw907/poplar/internal/store_test",
			wantRole:       "store",
			wantModuleRoot: "github.com/glw907/poplar",
			wantOK:         true,
		},
		{
			name:   "third-party package",
			path:   "charm.land/bubbletea/v2",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, moduleRoot, ok := Of(tt.path)
			if ok != tt.wantOK || role != tt.wantRole || moduleRoot != tt.wantModuleRoot {
				t.Errorf("Of(%q) = %q, %q, %v; want %q, %q, %v",
					tt.path, role, moduleRoot, ok, tt.wantRole, tt.wantModuleRoot, tt.wantOK)
			}
		})
	}
}

func TestInModule(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		moduleRoot string
		want       bool
	}{
		{"exact root", "a", "a", true},
		{"under root", "a/internal/ui", "a", true},
		{"different module", "b/internal/ui", "a", false},
		{"prefix collision", "abc/internal/ui", "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InModule(tt.path, tt.moduleRoot); got != tt.want {
				t.Errorf("InModule(%q, %q) = %v; want %v", tt.path, tt.moduleRoot, got, tt.want)
			}
		})
	}
}
