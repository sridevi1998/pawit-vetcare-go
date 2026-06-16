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
	t.Setenv("PAWIT_DOCUMENT_BUCKET", "pawit-prod-documents")
	t.Setenv("PAWIT_GCS_SIGNING_EMAIL", "pawit-signer@example.iam.gserviceaccount.com")
	t.Setenv("PAWIT_GCS_PRIVATE_KEY_PEM", "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected production config to load: %v", err)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("expected database URL to be set")
	}
	if cfg.DocumentBucket != "pawit-prod-documents" {
		t.Fatalf("expected production document bucket, got %q", cfg.DocumentBucket)
	}
}

func TestProductionRequiresDocumentBucket(t *testing.T) {
	t.Setenv("PAWIT_ENV", "production")
	t.Setenv("PAWIT_ALLOW_DEV_AUTH", "false")
	t.Setenv("PAWIT_JWT_SIGNING_KEY", "test-signing-key")
	t.Setenv("PAWIT_DATABASE_URL", "postgres://pawit:secret@example.invalid:5432/pawit")
	t.Setenv("PAWIT_DOCUMENT_BUCKET", "pawit-vetcare-documents-dev")
	t.Setenv("PAWIT_GCS_SIGNING_EMAIL", "pawit-signer@example.iam.gserviceaccount.com")
	t.Setenv("PAWIT_GCS_PRIVATE_KEY_PEM", "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----")

	if _, err := Load(); err == nil {
		t.Fatal("expected production config with dev document bucket to fail")
	}
}

func TestProductionRequiresGCSSigningConfig(t *testing.T) {
	t.Setenv("PAWIT_ENV", "production")
	t.Setenv("PAWIT_ALLOW_DEV_AUTH", "false")
	t.Setenv("PAWIT_JWT_SIGNING_KEY", "test-signing-key")
	t.Setenv("PAWIT_DATABASE_URL", "postgres://pawit:secret@example.invalid:5432/pawit")
	t.Setenv("PAWIT_DOCUMENT_BUCKET", "pawit-prod-documents")
	t.Setenv("PAWIT_GCS_SIGNING_EMAIL", "")
	t.Setenv("PAWIT_GCS_PRIVATE_KEY_PEM", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected production config without GCS signing config to fail")
	}
}

func TestLoadParsesTrustedProxyCIDRs(t *testing.T) {
	t.Setenv("PAWIT_TRUSTED_PROXY_CIDRS", "10.0.0.0/8, 192.0.2.10")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected config to load: %v", err)
	}
	if len(cfg.TrustedProxyCIDRs) != 2 {
		t.Fatalf("expected two trusted proxy entries, got %#v", cfg.TrustedProxyCIDRs)
	}
	if cfg.TrustedProxyCIDRs[0] != "10.0.0.0/8" || cfg.TrustedProxyCIDRs[1] != "192.0.2.10" {
		t.Fatalf("unexpected trusted proxy entries %#v", cfg.TrustedProxyCIDRs)
	}
}

func TestLoadRejectsInvalidTrustedProxyCIDRs(t *testing.T) {
	t.Setenv("PAWIT_TRUSTED_PROXY_CIDRS", "10.0.0.0/8,not-a-cidr")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid trusted proxy config to fail")
	}
}
