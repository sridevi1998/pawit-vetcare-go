CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE tenant_status AS ENUM ('active', 'suspended', 'archived');
CREATE TYPE user_status AS ENUM ('active', 'invited', 'disabled', 'archived');
CREATE TYPE species AS ENUM ('dog', 'cat');
CREATE TYPE appointment_type AS ENUM (
  'in_clinic',
  'telemedicine',
  'walk_in',
  'follow_up',
  'vaccination',
  'lab_diagnostic',
  'procedure_consult'
);
CREATE TYPE appointment_status AS ENUM (
  'requested',
  'scheduled',
  'confirmed',
  'checked_in',
  'waiting',
  'in_progress',
  'completed',
  'cancelled',
  'no_show',
  'needs_reschedule',
  'rejected'
);
CREATE TYPE queue_status AS ENUM ('waiting', 'called', 'in_progress', 'completed', 'cancelled');
CREATE TYPE clinical_note_status AS ENUM ('draft', 'finalized', 'archived');
CREATE TYPE prescription_status AS ENUM ('draft', 'finalized', 'archived');
CREATE TYPE lab_order_status AS ENUM ('ordered', 'sample_collected', 'sent_out', 'in_progress', 'completed', 'cancelled');
CREATE TYPE lab_type AS ENUM ('internal', 'external');
CREATE TYPE invoice_status AS ENUM ('draft', 'issued', 'pending', 'paid', 'void', 'refunded');
CREATE TYPE payment_status AS ENUM ('pending', 'succeeded', 'failed', 'refunded', 'cancelled');
CREATE TYPE document_status AS ENUM ('active', 'archived');
CREATE TYPE support_session_status AS ENUM ('active', 'expired', 'revoked');

CREATE TABLE tenants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  legal_name text,
  status tenant_status NOT NULL DEFAULT 'active',
  default_cancellation_cutoff_hours integer NOT NULL DEFAULT 24 CHECK (default_cancellation_cutoff_hours >= 0),
  stripe_customer_id text,
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz
);

CREATE TABLE clinic_locations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  name text NOT NULL,
  timezone text NOT NULL DEFAULT 'America/Chicago',
  phone text,
  email text,
  address_ciphertext bytea,
  cancellation_cutoff_hours integer CHECK (cancellation_cutoff_hours IS NULL OR cancellation_cutoff_hours >= 0),
  status tenant_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz
);

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL,
  email_normalized text GENERATED ALWAYS AS (lower(email)) STORED,
  password_hash text NOT NULL,
  display_name text NOT NULL,
  status user_status NOT NULL DEFAULT 'active',
  phone_ciphertext bytea,
  last_login_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz,
  UNIQUE (email_normalized)
);

CREATE TABLE roles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  description text NOT NULL,
  system_role boolean NOT NULL DEFAULT true
);

CREATE TABLE permissions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  description text NOT NULL
);

CREATE TABLE role_permissions (
  role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE tenant_memberships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  user_id uuid NOT NULL REFERENCES users(id),
  default_location_id uuid REFERENCES clinic_locations(id),
  status user_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz,
  UNIQUE (tenant_id, user_id)
);

CREATE TABLE membership_roles (
  membership_id uuid NOT NULL REFERENCES tenant_memberships(id) ON DELETE CASCADE,
  role_id uuid NOT NULL REFERENCES roles(id),
  PRIMARY KEY (membership_id, role_id)
);

CREATE TABLE pets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  primary_location_id uuid REFERENCES clinic_locations(id),
  name text NOT NULL,
  species species NOT NULL,
  breed text,
  sex text,
  date_of_birth date,
  estimated_age text,
  microchip_id_ciphertext bytea,
  allergies text,
  active_conditions text,
  status document_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz
);

CREATE TABLE pet_guardians (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  pet_id uuid NOT NULL REFERENCES pets(id),
  user_id uuid REFERENCES users(id),
  name text NOT NULL,
  email text,
  phone_ciphertext bytea,
  relationship text NOT NULL DEFAULT 'guardian',
  is_primary boolean NOT NULL DEFAULT false,
  can_view_records boolean NOT NULL DEFAULT true,
  can_request_appointments boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz
);

CREATE TABLE pet_weight_entries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  pet_id uuid NOT NULL REFERENCES pets(id),
  weight_oz integer NOT NULL CHECK (weight_oz > 0),
  measured_at timestamptz NOT NULL DEFAULT now(),
  recorded_by_user_id uuid REFERENCES users(id)
);

CREATE TABLE pet_documents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  pet_id uuid NOT NULL REFERENCES pets(id),
  uploaded_by_user_id uuid REFERENCES users(id),
  title text NOT NULL,
  document_type text NOT NULL,
  object_path text NOT NULL,
  content_type text NOT NULL,
  size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
  status document_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz
);

CREATE TABLE appointments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  location_id uuid NOT NULL REFERENCES clinic_locations(id),
  pet_id uuid NOT NULL REFERENCES pets(id),
  requested_by_user_id uuid REFERENCES users(id),
  primary_veterinarian_user_id uuid REFERENCES users(id),
  type appointment_type NOT NULL,
  status appointment_status NOT NULL DEFAULT 'requested',
  starts_at timestamptz,
  ends_at timestamptz,
  reason text NOT NULL,
  notes text,
  telemedicine_url text,
  cancellation_reason text,
  cancelled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz,
  CHECK ((type = 'telemedicine' AND telemedicine_url IS NOT NULL) OR type <> 'telemedicine')
);

CREATE TABLE appointment_veterinarians (
  appointment_id uuid NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
  veterinarian_user_id uuid NOT NULL REFERENCES users(id),
  is_primary boolean NOT NULL DEFAULT false,
  PRIMARY KEY (appointment_id, veterinarian_user_id)
);

CREATE UNIQUE INDEX one_primary_vet_per_appointment
  ON appointment_veterinarians (appointment_id)
  WHERE is_primary;

CREATE TABLE queue_entries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  location_id uuid NOT NULL REFERENCES clinic_locations(id),
  appointment_id uuid REFERENCES appointments(id),
  pet_id uuid NOT NULL REFERENCES pets(id),
  status queue_status NOT NULL DEFAULT 'waiting',
  priority text NOT NULL DEFAULT 'normal',
  reason text,
  checked_in_at timestamptz NOT NULL DEFAULT now(),
  called_at timestamptz,
  completed_at timestamptz,
  cancelled_at timestamptz
);

CREATE TABLE clinical_notes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  location_id uuid NOT NULL REFERENCES clinic_locations(id),
  pet_id uuid NOT NULL REFERENCES pets(id),
  appointment_id uuid REFERENCES appointments(id),
  author_user_id uuid NOT NULL REFERENCES users(id),
  finalized_by_user_id uuid REFERENCES users(id),
  status clinical_note_status NOT NULL DEFAULT 'draft',
  reason_for_visit text,
  subjective text,
  objective text,
  assessment text,
  plan text,
  vitals jsonb NOT NULL DEFAULT '{}'::jsonb,
  shared_with_pet_parent boolean NOT NULL DEFAULT false,
  finalized_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz
);

CREATE TABLE prescriptions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  location_id uuid NOT NULL REFERENCES clinic_locations(id),
  pet_id uuid NOT NULL REFERENCES pets(id),
  appointment_id uuid REFERENCES appointments(id),
  created_by_user_id uuid NOT NULL REFERENCES users(id),
  prescribing_veterinarian_user_id uuid REFERENCES users(id),
  status prescription_status NOT NULL DEFAULT 'draft',
  instructions text,
  shared_with_pet_parent boolean NOT NULL DEFAULT false,
  finalized_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz
);

CREATE TABLE prescription_medications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  prescription_id uuid NOT NULL REFERENCES prescriptions(id) ON DELETE CASCADE,
  medication_name text NOT NULL,
  strength text,
  dosage text,
  frequency text,
  duration text,
  route text,
  instructions text
);

CREATE TABLE lab_centers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  name text NOT NULL,
  lab_type lab_type NOT NULL,
  contact_email text,
  contact_phone text,
  created_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz
);

CREATE TABLE lab_orders (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  location_id uuid NOT NULL REFERENCES clinic_locations(id),
  pet_id uuid NOT NULL REFERENCES pets(id),
  appointment_id uuid REFERENCES appointments(id),
  lab_center_id uuid REFERENCES lab_centers(id),
  ordered_by_user_id uuid NOT NULL REFERENCES users(id),
  test_type text NOT NULL,
  sample_type text,
  priority text NOT NULL DEFAULT 'normal',
  status lab_order_status NOT NULL DEFAULT 'ordered',
  shared_with_pet_parent boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  cancelled_at timestamptz
);

CREATE TABLE lab_results (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  lab_order_id uuid NOT NULL REFERENCES lab_orders(id),
  uploaded_by_user_id uuid REFERENCES users(id),
  result_notes text,
  report_object_path text,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE invoices (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  location_id uuid NOT NULL REFERENCES clinic_locations(id),
  pet_id uuid REFERENCES pets(id),
  guardian_user_id uuid REFERENCES users(id),
  status invoice_status NOT NULL DEFAULT 'draft',
  currency char(3) NOT NULL DEFAULT 'USD',
  subtotal_cents integer NOT NULL DEFAULT 0 CHECK (subtotal_cents >= 0),
  tax_cents integer NOT NULL DEFAULT 0 CHECK (tax_cents >= 0),
  discount_cents integer NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
  total_cents integer NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
  stripe_payment_intent_id text,
  stripe_checkout_session_id text,
  due_at timestamptz,
  paid_at timestamptz,
  voided_at timestamptz,
  refunded_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE invoice_line_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  description text NOT NULL,
  quantity integer NOT NULL DEFAULT 1 CHECK (quantity > 0),
  unit_amount_cents integer NOT NULL CHECK (unit_amount_cents >= 0),
  total_amount_cents integer NOT NULL CHECK (total_amount_cents >= 0),
  related_resource_type text,
  related_resource_id uuid
);

CREATE TABLE payments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  invoice_id uuid NOT NULL REFERENCES invoices(id),
  status payment_status NOT NULL DEFAULT 'pending',
  amount_cents integer NOT NULL CHECK (amount_cents >= 0),
  currency char(3) NOT NULL DEFAULT 'USD',
  provider text NOT NULL DEFAULT 'stripe',
  provider_payment_id text,
  provider_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE support_access_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  super_admin_user_id uuid NOT NULL REFERENCES users(id),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  reason text NOT NULL,
  status support_session_status NOT NULL DEFAULT 'active',
  started_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz
);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid REFERENCES tenants(id),
  location_id uuid REFERENCES clinic_locations(id),
  actor_user_id uuid REFERENCES users(id),
  actor_role text,
  support_access_session_id uuid REFERENCES support_access_sessions(id),
  action text NOT NULL,
  resource_type text NOT NULL,
  resource_id uuid,
  reason text,
  ip_address inet,
  user_agent text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO roles (code, description) VALUES
  ('SuperAdmin', 'Internal PawIt platform administrator with audited support access.'),
  ('ClinicAdmin', 'Tenant administrator for clinic settings, staff, records, exports, and billing settings.'),
  ('Veterinarian', 'Clinical care provider responsible for notes, prescriptions, lab orders, and shared records.'),
  ('Receptionist', 'Front-desk operator for appointment, queue, document, and billing workflows.'),
  ('VetTechnician', 'Clinical support user for vitals, draft notes, prescription drafts, and delegated lab workflows.'),
  ('LabTechnician', 'Lab workflow user for sample status, result upload, and lab completion.'),
  ('PetParent', 'External pet guardian user for self-service records, requests, documents, and payments.');

INSERT INTO permissions (code, description) VALUES
  ('tenant.manage', 'Manage platform tenants.'),
  ('location.manage', 'Manage clinic locations.'),
  ('staff.manage', 'Manage staff and role assignments.'),
  ('pet_record.manage', 'Manage tenant pet records.'),
  ('pet_record.manage_own', 'Manage own linked pet details.'),
  ('pet_document.upload', 'Upload pet documents.'),
  ('appointment.manage', 'Manage appointments and requests.'),
  ('appointment_request.own', 'Create and cancel own appointment requests.'),
  ('queue.manage', 'Manage walk-in and check-in queue.'),
  ('clinical_note.view', 'View tenant clinical notes.'),
  ('clinical_note.draft', 'Create draft clinical notes.'),
  ('clinical_note.finalize', 'Finalize clinical notes.'),
  ('clinical_note.view_shared', 'View clinical notes shared with pet parent.'),
  ('prescription.view', 'View prescriptions.'),
  ('prescription.draft', 'Create prescription drafts.'),
  ('prescription.finalize', 'Finalize prescriptions.'),
  ('prescription.view_shared', 'View prescriptions shared with pet parent.'),
  ('lab_order.create', 'Create lab orders.'),
  ('lab_order.process', 'Process lab orders and results.'),
  ('lab_result.share', 'Share lab results with pet parent.'),
  ('lab_result.view_shared', 'View lab results shared with pet parent.'),
  ('invoice.create', 'Create invoices.'),
  ('invoice.manage', 'Manage invoices.'),
  ('invoice.pay_own', 'Pay own invoices.'),
  ('payment.refund_void', 'Refund payments and void invoices.'),
  ('analytics.full', 'View full tenant analytics.'),
  ('analytics.operational', 'View operational analytics.'),
  ('analytics.clinical', 'View clinical analytics.'),
  ('analytics.lab', 'View lab workflow analytics.'),
  ('data.export', 'Export tenant data.'),
  ('support.impersonation', 'Initiate audited support access.'),
  ('audit_log.view', 'View audit logs.'),
  ('shared_medical_record.view_own', 'View own shared medical records.'),
  ('pet_parent_profile.manage_own', 'Manage own pet-parent profile.');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN ('tenant.manage', 'support.impersonation', 'audit_log.view')
WHERE r.code = 'SuperAdmin';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
  'location.manage',
  'staff.manage',
  'pet_record.manage',
  'pet_document.upload',
  'appointment.manage',
  'queue.manage',
  'clinical_note.view',
  'prescription.view',
  'lab_order.create',
  'lab_order.process',
  'invoice.create',
  'invoice.manage',
  'payment.refund_void',
  'analytics.full',
  'data.export',
  'audit_log.view'
)
WHERE r.code = 'ClinicAdmin';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
  'pet_record.manage',
  'appointment.manage',
  'queue.manage',
  'clinical_note.view',
  'clinical_note.draft',
  'clinical_note.finalize',
  'prescription.view',
  'prescription.draft',
  'prescription.finalize',
  'lab_order.create',
  'lab_result.share',
  'analytics.clinical'
)
WHERE r.code = 'Veterinarian';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
  'pet_record.manage',
  'pet_document.upload',
  'appointment.manage',
  'queue.manage',
  'clinical_note.view',
  'prescription.view',
  'lab_order.create',
  'invoice.create',
  'analytics.operational'
)
WHERE r.code = 'Receptionist';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
  'pet_record.manage',
  'appointment.manage',
  'clinical_note.view',
  'clinical_note.draft',
  'prescription.view',
  'prescription.draft',
  'lab_order.create',
  'lab_order.process',
  'analytics.clinical'
)
WHERE r.code = 'VetTechnician';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN ('lab_order.process', 'analytics.lab')
WHERE r.code = 'LabTechnician';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
  'pet_parent_profile.manage_own',
  'pet_record.manage_own',
  'pet_document.upload',
  'appointment_request.own',
  'clinical_note.view_shared',
  'prescription.view_shared',
  'lab_result.view_shared',
  'invoice.pay_own',
  'shared_medical_record.view_own'
)
WHERE r.code = 'PetParent';

CREATE INDEX idx_locations_tenant ON clinic_locations(tenant_id);
CREATE INDEX idx_memberships_tenant_user ON tenant_memberships(tenant_id, user_id);
CREATE INDEX idx_pets_tenant ON pets(tenant_id);
CREATE INDEX idx_pet_guardians_tenant_pet ON pet_guardians(tenant_id, pet_id);
CREATE INDEX idx_pet_documents_tenant_pet ON pet_documents(tenant_id, pet_id);
CREATE INDEX idx_appointments_tenant_location_time ON appointments(tenant_id, location_id, starts_at);
CREATE INDEX idx_appointments_tenant_pet ON appointments(tenant_id, pet_id);
CREATE INDEX idx_queue_tenant_location_status ON queue_entries(tenant_id, location_id, status);
CREATE INDEX idx_clinical_notes_tenant_pet ON clinical_notes(tenant_id, pet_id);
CREATE INDEX idx_prescriptions_tenant_pet ON prescriptions(tenant_id, pet_id);
CREATE INDEX idx_lab_orders_tenant_status ON lab_orders(tenant_id, status);
CREATE INDEX idx_invoices_tenant_status ON invoices(tenant_id, status);
CREATE INDEX idx_payments_tenant_invoice ON payments(tenant_id, invoice_id);
CREATE INDEX idx_audit_logs_tenant_created ON audit_logs(tenant_id, created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
