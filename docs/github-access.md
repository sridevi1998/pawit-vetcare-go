# GitHub And Cloud Access Needed

To create separate repositories and deploy PawIt VetCare, I need you to provide or authorize:

## GitHub

- GitHub organization or username where repos should live
- Permission to create repositories
- Preferred visibility: private or public
- Repository names approval for:
  - `pawit-vetcare-api`
  - `pawit-vetcare-hospital`
  - `pawit-vetcare-pet-parent`
  - `pawit-vetcare-marketing`
  - `pawit-vetcare-booking-bff`
  - `pawit-vetcare-contracts`
  - `pawit-vetcare-infra`
- GitHub Actions permission to create environments: `dev`, `staging`, `production`
- GitHub Actions secrets permission for deployment credentials
- Branch protection preference for `main` and `integration`

## Google Cloud

- GCP project ID
- Billing-enabled project access
- Region confirmation, defaulting to `asia-south1`
- Permission to create or manage:
  - Cloud Run
  - Artifact Registry
  - Cloud SQL PostgreSQL
  - Memorystore Redis
  - Secret Manager
  - Cloud Load Balancing and managed certificates
  - VPC connector and private service networking
  - Cloud Trace, Cloud Logging, and IAM service accounts

## Third Parties

- AWS SES sender/domain credentials
- MSG91 credentials
- Firebase project credentials for FCM
- Razorpay key ID and secret
- Google Vertex AI project access
- Production domains and DNS access

## Local Authorization

If you want me to push from this machine, authenticate either `gh` or `git` for the target GitHub account, then tell me the organization/user name and repo visibility.
