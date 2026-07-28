package keyring

import "testing"

func TestToken(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		env        string
		want       string
		wantErr    bool
	}{
		{name: "configured wins", configured: "cfg-token", env: "env-token", want: "cfg-token"},
		{name: "falls back to env", configured: "", env: "env-token", want: "env-token"},
		{name: "errors with neither", configured: "", env: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvFastmailToken, tt.env)
			got, err := Token(tt.configured)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Token: want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Token: %v", err)
			}
			if got != tt.want {
				t.Errorf("Token = %q, want %q", got, tt.want)
			}
		})
	}
}
