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
