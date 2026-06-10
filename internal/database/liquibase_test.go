package database

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLiquibaseChangelogReferencesCanonicalSQLMigrations(t *testing.T) {
	changelog, err := os.ReadFile("../../db/liquibase/changelog/db.changelog-root.yaml")
	if err != nil {
		t.Fatalf("read Liquibase changelog: %v", err)
	}

	required := []string{
		"0001_pawit_core_schema.up.sql",
		"0001_pawit_core_schema.down.sql",
		"0002_mutation_idempotency.up.sql",
		"0002_mutation_idempotency.down.sql",
	}
	for _, item := range required {
		if !strings.Contains(string(changelog), item) {
			t.Fatalf("Liquibase changelog is missing %q", item)
		}
		if _, err := os.Stat("../../internal/database/migrations/" + item); err != nil {
			t.Fatalf("Liquibase migration target %q is unavailable: %v", item, err)
		}
	}
}

func TestLiquibaseScriptHelpDocumentsCommonCommands(t *testing.T) {
	output, err := exec.Command("../../scripts/liquibase.sh", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("liquibase help failed: %v\n%s", err, output)
	}

	text := string(output)
	for _, item := range []string{"validate", "status", "update", "LIQUIBASE_COMMAND_URL"} {
		if !strings.Contains(text, item) {
			t.Fatalf("Liquibase help output is missing %q:\n%s", item, text)
		}
	}
}

func TestLiquibaseScriptChecksRepositoryFilesBeforeDocker(t *testing.T) {
	script, err := os.ReadFile("../../scripts/liquibase.sh")
	if err != nil {
		t.Fatalf("read Liquibase script: %v", err)
	}

	required := []string{
		"db/liquibase/liquibase.properties",
		"db/liquibase/changelog/db.changelog-root.yaml",
		"Liquibase properties file not found",
		"Liquibase changelog not found",
	}
	for _, item := range required {
		if !strings.Contains(string(script), item) {
			t.Fatalf("Liquibase script is missing %q", item)
		}
	}
}
