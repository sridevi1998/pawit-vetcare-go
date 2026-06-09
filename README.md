# PawIt VetCare API

Serverless-ready Go backend for PawIt VetCare, a multi-tenant veterinary hospital management platform inspired by the Docran HMS UI and workflows.

## What Is Included

- Go API entrypoint for Cloud Run
- Secure-by-default middleware: auth boundary, tenant scope, CORS allow-listing, security headers, request size limits, rate limiting
- Veterinary HMS response contracts for appointments, calendar, queue, pet records, prescriptions, clinical notes, labs, billing, analytics, feedback, doctors, and staff
- PostgreSQL-backed tenant-scoped read store when `PAWIT_DATABASE_URL` is configured, with demo data fallback for local UI work
- Appointment request/create and cancellation APIs with role checks, cancellation cutoff enforcement, audit logs, and idempotency-key support
- Queue management APIs for walk-ins, call/start/complete/cancel transitions, audit logs, and idempotency-key support
- Pet record mutation APIs for dog/cat intake, audit-safe archival, and pet document metadata uploads with idempotency-key support
- Lab diagnostics APIs for creating orders, processing status transitions, uploading result metadata, and sharing results with idempotency-key support
- Billing mutation APIs for creating invoices and voiding invoices with audited ClinicAdmin approval and idempotency-key support
- Staff management mutation APIs with ClinicAdmin role checks and idempotency-key support
- Tenant audit log review API for authorized admins
- Dockerfile using a non-root distroless runtime
- GitHub Actions CI with formatting, tests, vulnerability scan, container build, and Trivy scan
- Cloud Run service manifest
- Architecture and GitHub access docs

## Run Locally

```sh
PAWIT_ALLOW_DEV_AUTH=true go run .
```

Then call:

```sh
curl http://localhost:8080/healthz
curl -H "X-PawIt-Tenant-ID: tenant_demo_clinic" http://localhost:8080/api/v1/pets
```

To run against local PostgreSQL instead of the in-memory demo store:

```sh
docker compose up -d postgres
LIQUIBASE_COMMAND_URL=jdbc:postgresql://host.docker.internal:5432/pawit scripts/liquibase.sh update
docker compose exec -T postgres psql -U pawit -d pawit < docs/database/local-dev-seed.sql
PAWIT_ALLOW_DEV_AUTH=true PAWIT_DATABASE_URL=postgres://pawit:local-password@localhost:5432/pawit?sslmode=disable go run .
```

Then use the seeded tenant, user, and role headers:

```sh
curl \
  -H "X-PawIt-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -H "X-PawIt-User-ID: 33333333-3333-3333-3333-333333333333" \
  -H "X-PawIt-Role: ClinicAdmin" \
  http://localhost:8080/api/v1/billing
```

## Production Environment

Required:

- `PAWIT_ENV=production`
- `PAWIT_ALLOW_DEV_AUTH=false`
- `PAWIT_JWT_SIGNING_KEY` from Google Secret Manager
- `PAWIT_DATABASE_URL` from Google Secret Manager
- `PAWIT_ALLOWED_ORIGINS` with the deployed frontend origins
- `PAWIT_TRUSTED_PROXY_CIDRS` with the trusted load balancer or ingress proxy IP/CIDR ranges when forwarded client IP headers should be honored for rate limiting

## Target Platform

- Runtime: Google Cloud Run serverless containers
- Region: `asia-south1` unless you choose otherwise
- Database: PostgreSQL 17 on Cloud SQL, private IP only
- Cache/realtime: Memorystore Redis
- Storage: Google Cloud Storage signed URLs
- AI: Vertex AI Gemini advisory features
- Email/push: AWS SES and Firebase FCM
- Payments: Stripe with Apple Pay readiness

See [docs/architecture.md](docs/architecture.md) and [docs/github-access.md](docs/github-access.md).

The current API surface is summarized in [docs/api-contract.md](docs/api-contract.md). The frontend-facing OpenAPI source of truth lives in the sibling `pawit-vetcare-contracts` repo at `openapi/pawit.v1.yaml`.

The database foundation is summarized in [docs/database/schema.md](docs/database/schema.md). The database decision record is in [docs/database/postgres-decision.md](docs/database/postgres-decision.md).

## Database Migrations

Liquibase is the migration interface for PostgreSQL. Changelogs live under
`db/liquibase` and reference the canonical SQL migrations in
`internal/database/migrations`.

```sh
scripts/liquibase.sh validate
scripts/liquibase.sh status
scripts/liquibase.sh update
```

The Docker-based runner expects a JDBC URL. For local Docker Desktop Postgres use:

```sh
LIQUIBASE_COMMAND_URL=jdbc:postgresql://host.docker.internal:5432/pawit
LIQUIBASE_COMMAND_USERNAME=pawit
LIQUIBASE_COMMAND_PASSWORD=local-password
```

The production migration image is built from `Dockerfile.liquibase` and runs the
Cloud Run job with the `pawit-liquibase-jdbc-url`, `pawit-database-username`, and
`pawit-database-password` secrets.

The legacy `go run ./cmd/migrate <up|down|status>` runner remains available for
local compatibility while deployments transition to Liquibase.

When `PAWIT_DATABASE_URL` is set, the API uses PostgreSQL for tenant-scoped reads. Without it, local development uses the in-memory demo store so frontend screens can be built before a database is running. The optional local seed file lives at [docs/database/local-dev-seed.sql](docs/database/local-dev-seed.sql).

## Local Verification

```sh
test -z "$(gofmt -l .)"
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
docker build -t pawit-vetcare-api:local .
```

CI also runs a Trivy HIGH/CRITICAL filesystem scan.

## Database Direction

PawIt VetCare is PostgreSQL-first. PostgreSQL is the source of truth for tenants, users, RBAC, pets, appointments, clinical records, labs, billing, payments, audit logs, and idempotent mutations.

Use relational tables for core business records and `jsonb` only for flexible metadata such as tenant settings, vitals payloads, and integration-specific data. MongoDB is not planned for the core transactional database because the product depends on strong relationships, constraints, transactions, and tenant-scoped reporting.
