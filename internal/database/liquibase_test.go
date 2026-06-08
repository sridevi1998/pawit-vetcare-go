package database

import (
	"os"
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
