package database

import (
	"strings"
	"testing"
)

func TestMigrationsAreEmbedded(t *testing.T) {
	up, err := Migrations("up")
	if err != nil {
		t.Fatalf("load up migrations: %v", err)
	}
	down, err := Migrations("down")
	if err != nil {
		t.Fatalf("load down migrations: %v", err)
	}
	if len(up) != 1 {
		t.Fatalf("expected 1 up migration, got %d", len(up))
	}
	if len(down) != 1 {
		t.Fatalf("expected 1 down migration, got %d", len(down))
	}
}

func TestCoreSchemaContainsTenantScopedTables(t *testing.T) {
	up, err := Migrations("up")
	if err != nil {
		t.Fatalf("load up migrations: %v", err)
	}
	sql := up[0].SQL
	required := []string{
		"CREATE TABLE tenants",
		"CREATE TABLE clinic_locations",
		"CREATE TABLE users",
		"CREATE TABLE roles",
		"CREATE TABLE permissions",
		"CREATE TABLE pets",
		"CREATE TABLE pet_guardians",
		"CREATE TABLE appointments",
		"CREATE TABLE appointment_veterinarians",
		"CREATE TABLE clinical_notes",
		"CREATE TABLE prescriptions",
		"CREATE TABLE lab_orders",
		"CREATE TABLE invoices",
		"CREATE TABLE audit_logs",
		"tenant_id uuid NOT NULL REFERENCES tenants(id)",
	}
	for _, item := range required {
		if !strings.Contains(sql, item) {
			t.Fatalf("schema is missing %q", item)
		}
	}
}

func TestInvalidMigrationDirectionFails(t *testing.T) {
	if _, err := Migrations("sideways"); err == nil {
		t.Fatal("expected invalid migration direction to fail")
	}
}

func TestMigrationVersionAndChecksum(t *testing.T) {
	migration := Migration{Name: "0001_pawit_core_schema.up.sql", SQL: "select 1;"}

	if migration.Version() != "0001_pawit_core_schema" {
		t.Fatalf("unexpected version %q", migration.Version())
	}
	if migration.Checksum() == "" {
		t.Fatal("expected checksum")
	}
}

func TestSplitSQLStatements(t *testing.T) {
	statements := splitSQLStatements("select 'a;b'; select 'it''s ok';")

	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d: %#v", len(statements), statements)
	}
	if statements[0] != "select 'a;b'" {
		t.Fatalf("unexpected first statement %q", statements[0])
	}
	if statements[1] != "select 'it''s ok'" {
		t.Fatalf("unexpected second statement %q", statements[1])
	}
}
