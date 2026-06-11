# API Contract

Base path: `/api/v1`

The formal OpenAPI contract lives in the sibling `pawit-vetcare-contracts` repo:

- `openapi/pawit.v1.yaml`
- Generated TypeScript types: `src/pawit-api.ts`

All application endpoints require:

- A valid `pawit_access` cookie or `Authorization: Bearer <jwt>`
- Tenant scope through JWT claim `tenant_id`
- JWTs must be signed with `HS256` using the configured PawIt signing key
- In local development only, `X-PawIt-Tenant-ID` is accepted when `PAWIT_ALLOW_DEV_AUTH=true`

Public auth endpoints do not require an existing session. `POST /auth/login`
verifies email, password, hospital/tenant, and role, then sets the `pawit_access`
HttpOnly cookie and returns the session context. `POST /auth/logout` expires the
same cookie.

All endpoints can return the shared error envelope for authentication, authorization,
validation, conflict, not found, dependency, and rate-limit failures. Rate-limit
responses use HTTP `429` with error code `rate_limited` and include a
`Retry-After` header in seconds.

All responses include `X-Request-ID`. When CORS is allowed for the request origin,
browser clients can read `X-Request-ID` and `Retry-After` through
`Access-Control-Expose-Headers`.

Pet-parent shared medical reads are owner-scoped. When a caller only has
`clinical_note.view_shared`, `prescription.view_shared`, or
`lab_result.view_shared`, list responses include only records that are both
shared with the pet parent and attached to a pet where the caller is an active
guardian with record-view access.

Pet-parent pet record reads are also owner-scoped. When a caller only has
`pet_record.manage_own`, pet record and pet document lists include only pets
where the caller is an active guardian with record-view access.

Pet-parent billing reads are owner-scoped. When a caller only has
`invoice.pay_own`, billing metrics and invoice lists include only invoices
attached to pets where the caller is an active guardian.

## Current API Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/auth/login` | Create a role-scoped authenticated session |
| `POST` | `/auth/logout` | Clear the authenticated session cookie |
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
| `POST` | `/clinical-notes` | Create a draft SOAP/clinical note with optional vitals |
| `POST` | `/clinical-notes/{id}/finalize` | Finalize a draft clinical note and optionally share it with the pet parent |
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
| `GET` | `/audit-logs` | Tenant audit log review for authorized admins |

## Contract Maintenance

- Keep the OpenAPI source synchronized with implemented backend routes before wiring new frontend screens.
- New endpoints should include operation IDs, tag descriptions, documented 4XX responses, and generated TypeScript types.

## Contract Test Coverage

The API repo includes hospital-portal response contract tests for stable read endpoint shapes in `internal/httpapi/contract_test.go`.
Mutation handler tests cover `Idempotency-Key` forwarding for every POST mutation endpoint.
