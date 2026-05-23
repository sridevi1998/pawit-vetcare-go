# PawIt VetCare API

Serverless-ready Go backend for PawIt VetCare, a multi-tenant veterinary hospital management platform inspired by the Docran HMS UI and workflows.

## What Is Included

- Go API entrypoint for Cloud Run
- Secure-by-default middleware: auth boundary, tenant scope, CORS allow-listing, security headers, request size limits, rate limiting
- Veterinary HMS response contracts for appointments, calendar, queue, pet records, prescriptions, clinical notes, labs, billing, analytics, feedback, doctors, and staff
- PostgreSQL-backed tenant-scoped read store when `PAWIT_DATABASE_URL` is configured, with demo data fallback for local UI work
- Appointment request/create and cancellation APIs with role checks, cancellation cutoff enforcement, audit logs, and idempotency-key support
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

## Production Environment

Required:

- `PAWIT_ENV=production`
- `PAWIT_ALLOW_DEV_AUTH=false`
- `PAWIT_JWT_SIGNING_KEY` from Google Secret Manager
- `PAWIT_DATABASE_URL` from Google Secret Manager
- `PAWIT_ALLOWED_ORIGINS` with the deployed frontend origins

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

The current API surface is summarized in [docs/api-contract.md](docs/api-contract.md).

The database foundation is summarized in [docs/database/schema.md](docs/database/schema.md).

## Database Migrations

```sh
PAWIT_DATABASE_URL=postgres://pawit:local-password@localhost:5432/pawit?sslmode=disable go run ./cmd/migrate status
PAWIT_DATABASE_URL=postgres://pawit:local-password@localhost:5432/pawit?sslmode=disable go run ./cmd/migrate up
```

The production container includes `/app/pawit-migrate` for Cloud Run migration jobs.

When `PAWIT_DATABASE_URL` is set, the API uses PostgreSQL for tenant-scoped reads. Without it, local development uses the in-memory demo store so frontend screens can be built before a database is running.
