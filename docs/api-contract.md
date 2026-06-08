# API Contract

Base path: `/api/v1`

The formal OpenAPI contract lives in the sibling `pawit-vetcare-contracts` repo:

- `openapi/pawit.v1.yaml`
- Generated TypeScript types: `src/pawit-api.ts`

All application endpoints require:

- A valid `pawit_access` cookie or `Authorization: Bearer <jwt>`
- Tenant scope through JWT claim `tenant_id`
- In local development only, `X-PawIt-Tenant-ID` is accepted when `PAWIT_ALLOW_DEV_AUTH=true`

## Current API Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/me` | Current user, role, tenant, and clinic context |
| `GET` | `/product-spec` | Approved v1 product policy: species, statuses, payment provider, telemedicine, labs, cancellation |
| `GET` | `/role-policies` | Role and permission policy matrix from the spec |
| `GET` | `/navigation` | Sidebar sections for Docran-style hospital portal layout |
| `GET` | `/dashboard/summary` | Top-level operational metrics |
| `GET` | `/appointments` | Appointment list |
| `POST` | `/appointments` | Create a staff-managed appointment or pet-parent appointment request |
| `POST` | `/appointments/{id}/cancel` | Cancel an appointment with clinic cutoff enforcement |
| `GET` | `/calendar` | Appointment calendar data |
| `GET` | `/queue` | Walk-in and checked-in queue |
| `POST` | `/queue/walk-ins` | Register a walk-in queue entry and linked appointment |
| `POST` | `/queue/{id}/call` | Mark the next queue entry as called |
| `POST` | `/queue/{id}/start` | Move a queue entry into consultation |
| `POST` | `/queue/{id}/complete` | Complete a queue entry and linked appointment |
| `POST` | `/queue/{id}/cancel` | Cancel a queue entry and linked appointment |
| `GET` | `/pets` | Pet and pet-parent records |
| `POST` | `/pets` | Create a dog/cat pet record with first guardian |
| `POST` | `/pets/{id}/archive` | Archive a pet record with audit reason |
| `GET` | `/pets/{id}/documents` | List active document metadata for a pet |
| `POST` | `/pets/{id}/documents` | Register uploaded pet document metadata |
| `POST` | `/pets/{id}/documents/{documentId}/archive` | Archive pet document metadata with an audit reason |
| `GET` | `/patients` | Pet and pet-parent records |
| `GET` | `/prescriptions` | Tenant prescription drafts and finalized prescriptions |
| `POST` | `/prescriptions` | Create a prescription draft with medication lines |
| `POST` | `/prescriptions/{id}/finalize` | Finalize a draft prescription and optionally share it with the pet parent |
| `GET` | `/prescription-templates` | Veterinary prescription templates |
| `GET` | `/clinical-notes` | SOAP notes and consultations |
| `GET` | `/lab-tests` | Diagnostics and reports |
| `POST` | `/lab-tests` | Create an internal or external lab order |
| `POST` | `/lab-tests/{id}/status` | Process a lab order through sample/result statuses |
| `POST` | `/lab-tests/{id}/report` | Upload lab result metadata and optionally share it |
| `GET` | `/billing` | Billing metrics and invoices |
| `POST` | `/billing/invoices` | Create a draft or issued invoice with line items |
| `POST` | `/billing/invoices/{id}/void` | Void an invoice with audited ClinicAdmin approval |
| `GET` | `/analytics` | Demographics, appointments, revenue, diagnoses |
| `GET` | `/feedback` | Reviews and rating distribution |
| `GET` | `/doctors` | Veterinarian management |
| `GET` | `/staff` | Staff management |
| `POST` | `/staff` | Invite or reactivate a tenant staff member and assign a staff role |

## Contract Maintenance

- Keep the OpenAPI source synchronized with implemented backend routes before wiring new frontend screens.
- New endpoints should include operation IDs, tag descriptions, documented 4XX responses, and generated TypeScript types.

## Contract Test Coverage

The API repo includes hospital-portal response contract tests for stable read endpoint shapes in `internal/httpapi/contract_test.go`.
Mutation handler tests cover `Idempotency-Key` forwarding for every POST mutation endpoint.
