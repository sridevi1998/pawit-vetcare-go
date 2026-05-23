# PawIt VetCare Architecture

PawIt VetCare mirrors the Docran product shape but adapts workflows for veterinary clinics and hospitals.

## Repositories

| Repository | Runtime | Purpose |
| --- | --- | --- |
| `pawit-vetcare-api` | Go 1.22 on Cloud Run | Multi-tenant API, auth, RBAC, veterinary HMS domain, billing, diagnostics, AI advisory orchestration |
| `pawit-vetcare-hospital` | Next.js 15, React 19, TypeScript, Tailwind CSS | Vet clinic staff portal matching the Docran-style operational UI |
| `pawit-vetcare-pet-parent` | Vite 7, React 19, Capacitor, i18next, Tailwind CSS | Pet parent web/mobile portal |
| `pawit-vetcare-marketing` | Next.js 16, React 19, Tailwind v4, Radix UI, Framer Motion | Public website |
| `pawit-vetcare-booking-bff` | Go 1.22 on Cloud Run | Public booking API boundary for appointment search and booking |
| `pawit-vetcare-contracts` | TypeScript package and OpenAPI | Shared API contracts for frontend type safety |
| `pawit-vetcare-infra` | Terraform | GCP Cloud Run, Cloud SQL, Memorystore, Artifact Registry, Secret Manager, IAM, load balancer |

## Serverless Deployment

All runtime services should deploy to Google Cloud Run in `asia-south1` behind a global HTTPS load balancer. PostgreSQL 17 runs on Cloud SQL with private IP only. Redis runs on Memorystore with private networking. Secrets live in Google Secret Manager and are injected at runtime.

## Backend Choices

The backend replaces Docran's TypeScript/Express service with Go. Recommended production libraries for the next slice:

- HTTP router: Go `net/http` or `chi`
- DB access: `pgx` plus `sqlc` for typed PostgreSQL queries
- Validation: typed request DTOs plus `go-playground/validator`
- Realtime: Cloud Run WebSockets or Pub/Sub-driven updates; Redis adapter if Socket.IO compatibility is required
- AI: Vertex AI Gemini advisory calls, never autonomous diagnosis
- Observability: OpenTelemetry to Google Cloud Trace and structured JSON logs

## Veterinary Modules

- Appointments and check-in
- Calendar and queue
- Pet and pet-parent records
- SOAP/clinical notes
- Vaccination and deworming plans
- Veterinary prescriptions and templates
- Lab and diagnostics
- Billing, invoices, Razorpay payments
- Staff and veterinarian management
- Feedback and analytics
- Teleconsultation and pet-parent self service

## Security Baseline

- JWT in `HttpOnly`, `Secure`, `SameSite=Strict` cookies with refresh-token rotation
- WebAuthn support for staff
- Database-backed RBAC and per-feature permissions
- Tenant scope required on every application request
- Private Cloud SQL and Redis, no public IP
- TLS end-to-end, HSTS, strict CSP, CORS allow-listing
- Field-level AES-256-GCM encryption for sensitive pet-parent data
- Audit logs for clinical, billing, identity, and admin actions
- CI gates for tests, formatting, vulnerability scanning, secret scanning, and container scanning
