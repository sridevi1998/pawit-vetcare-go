# Database Decision: PostgreSQL

PawIt VetCare should continue with PostgreSQL as the primary operational database.

## Decision

Use PostgreSQL for production and local persistence. The target production service is Cloud SQL for PostgreSQL 17, connected privately from Cloud Run.

MongoDB is not recommended for the core PawIt VetCare transactional database.

## Why PostgreSQL Fits PawIt

PawIt is a multi-tenant clinic operations system. The main data is highly relational:

- tenants, clinic locations, users, roles, and permissions
- pets, guardians, documents, weights, and clinical records
- appointments, veterinarians, queue entries, and cancellations
- lab orders, lab results, prescriptions, invoices, payments, and audit logs

These workflows need foreign keys, transactions, uniqueness rules, strict status values, tenant-scoped queries, and reliable reporting. PostgreSQL gives those capabilities directly, while still supporting flexible fields through `jsonb` where the shape can vary.

The current repository already implements this direction:

- `PAWIT_DATABASE_URL` enables the PostgreSQL store.
- Migrations live in `internal/database/migrations`.
- `internal/database/store.go` contains the PostgreSQL-backed tenant store.
- Production requires `PAWIT_DATABASE_URL`.
- Cloud Run manifests are prepared for a database-backed deployment.

## Cost Position

PostgreSQL is the more cost-effective choice for this project because one managed database can cover operational data, billing data, reporting queries, audit records, and flexible JSON metadata.

MongoDB could reduce early schema planning, but it would likely increase application complexity for appointments, billing, RBAC, idempotency, and analytics because those relationships would need more application-side enforcement.

## When To Use JSONB

Use regular relational columns for important business fields that need filtering, joins, permissions, reporting, or constraints.

Use PostgreSQL `jsonb` for flexible metadata such as:

- tenant settings
- clinical vitals payloads
- lab integration payloads
- external service metadata
- feature-specific configuration

Avoid storing core records as large JSON documents when they need joins or strong consistency.

## MongoDB Reconsideration Criteria

Reconsider MongoDB only for a separate secondary workload, not the core transactional database, if PawIt later needs:

- high-volume append-only event storage
- unstructured ingestion of third-party clinical documents
- search-oriented document processing
- analytics pipelines where denormalized records are intentionally duplicated

Even then, keep PostgreSQL as the source of truth for appointments, patients, billing, permissions, and audit records.

## Implementation Next Steps

1. Keep PostgreSQL as the only production database target.
2. Continue extending SQL migrations for new product modules.
3. Add typed query generation with `sqlc` when query volume grows.
4. Keep mutation endpoints transactional and idempotent.
5. Add indexes around common tenant-scoped reads before load testing.
6. Store uploaded files in object storage and keep only metadata in PostgreSQL.
7. Use read replicas or materialized views later if analytics traffic grows.

