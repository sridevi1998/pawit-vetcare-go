# Security Policy

PawIt VetCare handles sensitive clinical, billing, and pet-parent data. Treat all production data as confidential.

## Current Controls In This Repo

- Production startup fails without `PAWIT_JWT_SIGNING_KEY`
- Development auth is blocked in production
- API requests require authentication and tenant scope
- Security headers are applied to every response
- CORS is allow-listed
- Request bodies are size-limited
- Basic per-IP rate limiting is enabled
- Docker runtime uses a distroless non-root image
- CI includes tests, formatting, vulnerability scanning, and Trivy scanning

## Required Before Production

- Replace demo data with PostgreSQL 17 through private Cloud SQL
- Enforce database tenant scoping for every query
- Add refresh-token rotation and token revocation
- Add field-level AES-256-GCM encryption for sensitive personal data
- Add immutable audit logs for clinical, billing, and admin actions
- Add WebAuthn for staff accounts
- Add SAST, dependency review, and secret scanning branch protections
- Complete a threat model for telemedicine, payments, file uploads, and AI advisory flows

## Reporting

Do not open public issues for vulnerabilities. Report privately to the repository owners.
