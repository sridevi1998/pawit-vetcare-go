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

## Migrations

Migration files live in:

- `internal/database/migrations/0001_pawit_core_schema.up.sql`
- `internal/database/migrations/0001_pawit_core_schema.down.sql`
- `internal/database/migrations/0002_mutation_idempotency.up.sql`
- `internal/database/migrations/0002_mutation_idempotency.down.sql`

The Go package `internal/database` embeds migrations so the future deploy/migration job can load them in order.
The API uses the same schema for tenant-scoped read endpoints whenever `PAWIT_DATABASE_URL` is configured.

Run migrations with:

```sh
PAWIT_DATABASE_URL=postgres://pawit:local-password@localhost:5432/pawit?sslmode=disable go run ./cmd/migrate up
```

The container image includes both binaries:

- `/app/pawit-api`
- `/app/pawit-migrate`

The Cloud Run migration job manifest is in `deployments/cloud-run/migration-job.yaml`.

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

Add shared contract tests for API responses used by the hospital portal and generate frontend clients from the OpenAPI source of truth.
