package config

import "testing"

func TestProductionRequiresDatabaseURL(t *testing.T) {
	t.Setenv("PAWIT_ENV", "production")
	t.Setenv("PAWIT_ALLOW_DEV_AUTH", "false")
	t.Setenv("PAWIT_JWT_SIGNING_KEY", "test-signing-key")
	t.Setenv("PAWIT_DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected production config without PAWIT_DATABASE_URL to fail")
	}
}

func TestProductionConfigAcceptsRequiredSecrets(t *testing.T) {
	t.Setenv("PAWIT_ENV", "production")
	t.Setenv("PAWIT_ALLOW_DEV_AUTH", "false")
	t.Setenv("PAWIT_JWT_SIGNING_KEY", "test-signing-key")
	t.Setenv("PAWIT_DATABASE_URL", "postgres://pawit:secret@example.invalid:5432/pawit")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected production config to load: %v", err)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("expected database URL to be set")
	}
}
