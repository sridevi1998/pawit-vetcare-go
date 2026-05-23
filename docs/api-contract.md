# API Contract

Base path: `/api/v1`

All application endpoints require:

- A valid `pawit_access` cookie or `Authorization: Bearer <jwt>`
- Tenant scope through JWT claim `tenant_id`
- In local development only, `X-PawIt-Tenant-ID` is accepted when `PAWIT_ALLOW_DEV_AUTH=true`

## Current Read Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/me` | Current user, role, tenant, and clinic context |
| `GET` | `/product-spec` | Approved v1 product policy: species, statuses, payment provider, telemedicine, labs, cancellation |
| `GET` | `/role-policies` | Role and permission policy matrix from the spec |
| `GET` | `/navigation` | Sidebar sections for Docran-style hospital portal layout |
| `GET` | `/dashboard/summary` | Top-level operational metrics |
| `GET` | `/appointments` | Appointment list |
| `GET` | `/calendar` | Appointment calendar data |
| `GET` | `/queue` | Walk-in and checked-in queue |
| `GET` | `/pets` | Pet and pet-parent records |
| `GET` | `/patients` | Pet and pet-parent records |
| `GET` | `/prescription-templates` | Veterinary prescription templates |
| `GET` | `/clinical-notes` | SOAP notes and consultations |
| `GET` | `/lab-tests` | Diagnostics and reports |
| `GET` | `/billing` | Billing metrics and invoices |
| `GET` | `/analytics` | Demographics, appointments, revenue, diagnoses |
| `GET` | `/feedback` | Reviews and rating distribution |
| `GET` | `/doctors` | Veterinarian management |
| `GET` | `/staff` | Staff management |

## Next Contract Slice

- Add OpenAPI 3.1 spec in `pawit-vetcare-contracts`
- Generate frontend TypeScript clients from OpenAPI
- Expand PostgreSQL repositories with create/update workflows
- Add mutation endpoints with idempotency keys for appointments, queue, invoices, lab reports, staff, and prescriptions
- Add shared contract tests for API responses used by the hospital portal
