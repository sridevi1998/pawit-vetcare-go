# PawIt VetCare Database Schema

The first persistence slice is PostgreSQL-first and designed for Cloud SQL PostgreSQL 17. The database choice is documented in [postgres-decision.md](postgres-decision.md).

## Principles

- Every operational record is tenant-scoped.
- Location-specific records include `location_id`.
- Clinical, financial, identity, and audit records are archived/cancelled/voided instead of hard-deleted.
- Roles and permissions are database-backed.
- Pet parents can own multiple pets, and pets can have multiple guardians.
- Appointments support one primary veterinarian and additional veterinarians.
- External labs are modeled in v1, but integration is manual/status-based.
- Stripe payment references are stored without card details.
- Sensitive personal fields have ciphertext columns where direct lookup is not required.

## Liquibase Migrations

Liquibase is the migration interface for PostgreSQL. The root changelog lives at:

- `db/liquibase/changelog/db.changelog-root.yaml`

The changelog references the canonical SQL migration files:

- `internal/database/migrations/0001_pawit_core_schema.up.sql`
- `internal/database/migrations/0001_pawit_core_schema.down.sql`
- `internal/database/migrations/0002_mutation_idempotency.up.sql`
- `internal/database/migrations/0002_mutation_idempotency.down.sql`

The API uses the same schema for tenant-scoped read endpoints whenever `PAWIT_DATABASE_URL` is configured.

Run migrations with:

```sh
LIQUIBASE_COMMAND_URL=jdbc:postgresql://host.docker.internal:5432/pawit scripts/liquibase.sh update
```

Validate migration configuration with:

```sh
scripts/liquibase.sh validate
```

The production Liquibase image is built from `Dockerfile.liquibase`. The Cloud
Run migration job manifest is in `deployments/cloud-run/migration-job.yaml` and
consumes the `pawit-liquibase-jdbc-url`, `pawit-database-username`, and
`pawit-database-password` secrets.

The legacy Go migration runner remains available for local compatibility during
the transition, but new deployment wiring should use Liquibase.

## Core Tables

- `tenants`
- `clinic_locations`
- `users`
- `roles`
- `permissions`
- `role_permissions`
- `tenant_memberships`
- `membership_roles`
- `pets`
- `pet_guardians`
- `pet_weight_entries`
- `pet_documents`
- `appointments`
- `appointment_veterinarians`
- `queue_entries`
- `clinical_notes`
- `prescriptions`
- `prescription_medications`
- `lab_centers`
- `lab_orders`
- `lab_results`
- `invoices`
- `invoice_line_items`
- `payments`
- `support_access_sessions`
- `audit_logs`
- `idempotency_keys`

## Next Integration Step

Extend Liquibase-backed SQL migrations for new modules and keep generated
frontend clients aligned with the OpenAPI source of truth.
