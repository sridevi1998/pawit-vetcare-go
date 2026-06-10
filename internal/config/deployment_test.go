package config

import (
	"os"
	"strings"
	"testing"
)

func TestCloudRunServiceInjectsRequiredProductionSecrets(t *testing.T) {
	manifest, err := os.ReadFile("../../deployments/cloud-run/service.yaml")
	if err != nil {
		t.Fatalf("read Cloud Run service manifest: %v", err)
	}

	required := []string{
		"name: PAWIT_ENV",
		"value: production",
		"name: PAWIT_ALLOW_DEV_AUTH",
		"value: \"false\"",
		"name: PAWIT_ALLOWED_ORIGINS",
		"value: https://hospital.pawit.care,https://app.pawit.care",
		"name: PAWIT_TRUSTED_PROXY_CIDRS",
		"value: 10.0.0.0/8",
		"name: PAWIT_RATE_LIMIT_RPM",
		"value: \"120\"",
		"name: PAWIT_REQUEST_BODY_LIMIT_BYTES",
		"value: \"1048576\"",
		"name: PAWIT_JWT_SIGNING_KEY",
		"name: pawit-jwt-signing-key",
		"name: PAWIT_DATABASE_URL",
		"name: pawit-database-url",
	}
	for _, item := range required {
		if !strings.Contains(string(manifest), item) {
			t.Fatalf("Cloud Run service manifest is missing %q", item)
		}
	}
}

func TestCloudRunMigrationJobRunsLiquibaseWithRequiredSecrets(t *testing.T) {
	manifest, err := os.ReadFile("../../deployments/cloud-run/migration-job.yaml")
	if err != nil {
		t.Fatalf("read Cloud Run migration job manifest: %v", err)
	}

	required := []string{
		"name: pawit-vetcare-migrate",
		"run.googleapis.com/execution-environment: gen2",
		"run.googleapis.com/vpc-access-egress: private-ranges-only",
		"maxRetries: 0",
		"timeoutSeconds: 600",
		"pawit-vetcare-liquibase:COMMIT_SHA",
		"- update",
		"name: LIQUIBASE_COMMAND_URL",
		"name: pawit-liquibase-jdbc-url",
		"name: LIQUIBASE_COMMAND_USERNAME",
		"name: pawit-database-username",
		"name: LIQUIBASE_COMMAND_PASSWORD",
		"name: pawit-database-password",
	}
	for _, item := range required {
		if !strings.Contains(string(manifest), item) {
			t.Fatalf("Cloud Run migration job manifest is missing %q", item)
		}
	}
}
