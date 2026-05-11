package wizard

import "testing"

func TestApplyFastmailJMAP(t *testing.T) {
	m := Model{
		Preset:       "fastmail",
		Email:        "user@fastmail.com",
		Token:        "fmip-secret-token",
		IdentityName: "User",
		AccountLabel: "Fastmail",
	}
	cfg, err := Apply(m)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if cfg.Preset != "fastmail" {
		t.Errorf("Preset = %q", cfg.Preset)
	}
	if cfg.Backend != "jmap" {
		t.Errorf("Backend = %q, want jmap", cfg.Backend)
	}
	if cfg.Email != "user@fastmail.com" {
		t.Errorf("Email = %q", cfg.Email)
	}
	if cfg.Name != "Fastmail" {
		t.Errorf("Name = %q", cfg.Name)
	}
	if cfg.Password != "fmip-secret-token" {
		t.Errorf("Password = %q", cfg.Password)
	}
	if len(cfg.Identities) != 1 || cfg.Identities[0].Name != "User" {
		t.Errorf("Identities = %+v", cfg.Identities)
	}
}

func TestApplyPlainIMAP(t *testing.T) {
	m := Model{
		Preset:   "imap",
		Email:    "user@example.com",
		Host:     "mail.example.com",
		Port:     "993",
		Password: "p",
	}
	cfg, err := Apply(m)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if cfg.Backend != "imap" || cfg.Host != "mail.example.com" || cfg.Port != 993 {
		t.Errorf("backend/host/port = %s/%s:%d", cfg.Backend, cfg.Host, cfg.Port)
	}
	if cfg.SMTP.Host != "mail.example.com" {
		t.Errorf("SMTP.Host = %q", cfg.SMTP.Host)
	}
}

func TestApplyAccountLabelDefaultsToEmail(t *testing.T) {
	m := Model{Preset: "fastmail", Email: "u@fm.com", Token: "t"}
	cfg, err := Apply(m)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if cfg.Name != "u@fm.com" {
		t.Errorf("Name = %q, want fallback to email", cfg.Name)
	}
}

func TestApplyPortInvalid(t *testing.T) {
	m := Model{Preset: "imap", Email: "u@x", Host: "h", Port: "nope"}
	if _, err := Apply(m); err == nil {
		t.Fatal("expected port parse error")
	}
}

func TestApplyOAuthDone(t *testing.T) {
	m := Model{
		Preset:      "gmail",
		Email:       "user@gmail.com",
		OAuthDone:   true,
		OAuthStore:  "keyring",
		OAuthCID:    "my-client-id",
		OAuthSecret: "my-client-secret",
	}
	cfg, err := Apply(m)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if cfg.Auth != "xoauth2" {
		t.Errorf("Auth = %q, want xoauth2", cfg.Auth)
	}
	if cfg.PasswordCmd != "" {
		t.Errorf("PasswordCmd = %q, want empty", cfg.PasswordCmd)
	}
	if cfg.OAuth == nil || cfg.OAuth.ClientID != "my-client-id" {
		t.Errorf("OAuth = %+v", cfg.OAuth)
	}
	if cfg.OAuthStore != "keyring" {
		t.Errorf("OAuthStore = %q", cfg.OAuthStore)
	}
}

func TestApplyOAuthNotDoneErrors(t *testing.T) {
	m := Model{Preset: "gmail", Email: "user@gmail.com"}
	if _, err := Apply(m); err == nil {
		t.Fatal("expected error when OAuthDone is false")
	}
}

func TestApplySignatureOnlyNoIdentityName(t *testing.T) {
	m := Model{
		Preset:    "fastmail",
		Email:     "geoff@907.life",
		Token:     "tok",
		Signature: "Geoff\nlinks at [907.life](https://907.life)",
	}
	cfg, err := Apply(m)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(cfg.Identities) != 1 {
		t.Fatalf("Identities len = %d, want 1", len(cfg.Identities))
	}
	id := cfg.Identities[0]
	if id.Name != "" || id.Email != "geoff@907.life" {
		t.Errorf("Identity = %+v", id)
	}
	if len(id.Signatures) != 1 {
		t.Fatalf("Signatures len = %d, want 1", len(id.Signatures))
	}
	sig := id.Signatures[0]
	if sig.Name != "default" || sig.Text != "Geoff\nlinks at [907.life](https://907.life)" {
		t.Errorf("Signature = %+v", sig)
	}
}

func TestApplyIdentityNamePlusSignature(t *testing.T) {
	m := Model{
		Preset:       "fastmail",
		Email:        "geoff@907.life",
		Token:        "tok",
		IdentityName: "Geoff Wright",
		Signature:    "Geoff Wright\ngeoff@907.life",
	}
	cfg, err := Apply(m)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(cfg.Identities) != 1 {
		t.Fatalf("Identities len = %d, want 1", len(cfg.Identities))
	}
	id := cfg.Identities[0]
	if id.Name != "Geoff Wright" || id.Email != "geoff@907.life" {
		t.Errorf("Identity = %+v", id)
	}
	if len(id.Signatures) != 1 {
		t.Fatalf("Signatures len = %d, want 1", len(id.Signatures))
	}
	sig := id.Signatures[0]
	if sig.Name != "default" || sig.Text != "Geoff Wright\ngeoff@907.life" {
		t.Errorf("Signature = %+v", sig)
	}
}

func TestApplyEmptySignatureWithIdentityNameProducesNoSignatures(t *testing.T) {
	m := Model{
		Preset:       "fastmail",
		Email:        "geoff@907.life",
		Token:        "tok",
		IdentityName: "Geoff Wright",
	}
	cfg, err := Apply(m)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(cfg.Identities) != 1 {
		t.Fatalf("Identities len = %d, want 1", len(cfg.Identities))
	}
	id := cfg.Identities[0]
	if len(id.Signatures) != 0 {
		t.Errorf("Signatures = %+v, want none", id.Signatures)
	}
}
