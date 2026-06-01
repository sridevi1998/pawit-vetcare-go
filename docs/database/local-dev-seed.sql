INSERT INTO tenants (id, name, legal_name, status)
VALUES (
  '11111111-1111-1111-1111-111111111111',
  'PawIt Demo Clinic',
  'PawIt Demo Clinic LLC',
  'active'
)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    legal_name = EXCLUDED.legal_name,
    status = EXCLUDED.status,
    updated_at = now();

INSERT INTO clinic_locations (id, tenant_id, name, timezone, phone, email, status)
VALUES (
  '22222222-2222-2222-2222-222222222222',
  '11111111-1111-1111-1111-111111111111',
  'PawIt Demo Hospital',
  'America/Chicago',
  '+13125550100',
  'frontdesk@pawit.example',
  'active'
)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    timezone = EXCLUDED.timezone,
    phone = EXCLUDED.phone,
    email = EXCLUDED.email,
    status = EXCLUDED.status,
    updated_at = now();

INSERT INTO users (id, email, password_hash, display_name, status)
VALUES
  (
    '33333333-3333-3333-3333-333333333333',
    'admin@pawit.example',
    'local-dev-only',
    'Asha Clinic Admin',
    'active'
  ),
  (
    '44444444-4444-4444-4444-444444444444',
    'avery@pawit.example',
    'local-dev-only',
    'Avery Parker',
    'active'
  ),
  (
    '55555555-5555-5555-5555-555555555555',
    'doctor@pawit.example',
    'local-dev-only',
    'Dr. Asha Rao',
    'active'
  )
ON CONFLICT (email_normalized) DO UPDATE
SET display_name = EXCLUDED.display_name,
    status = EXCLUDED.status,
    updated_at = now();

INSERT INTO tenant_memberships (id, tenant_id, user_id, default_location_id, status)
VALUES (
  '66666666-6666-6666-6666-666666666666',
  '11111111-1111-1111-1111-111111111111',
  '33333333-3333-3333-3333-333333333333',
  '22222222-2222-2222-2222-222222222222',
  'active'
)
ON CONFLICT (tenant_id, user_id) DO UPDATE
SET default_location_id = EXCLUDED.default_location_id,
    status = EXCLUDED.status,
    updated_at = now();

INSERT INTO membership_roles (membership_id, role_id)
SELECT '66666666-6666-6666-6666-666666666666', id
FROM roles
WHERE code = 'ClinicAdmin'
ON CONFLICT DO NOTHING;

INSERT INTO pets (id, tenant_id, primary_location_id, name, species, breed, sex, estimated_age, status)
VALUES (
  '77777777-7777-7777-7777-777777777777',
  '11111111-1111-1111-1111-111111111111',
  '22222222-2222-2222-2222-222222222222',
  'Milo',
  'cat',
  'Domestic Shorthair',
  'Male',
  '3y',
  'active'
)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    species = EXCLUDED.species,
    breed = EXCLUDED.breed,
    sex = EXCLUDED.sex,
    estimated_age = EXCLUDED.estimated_age,
    status = EXCLUDED.status,
    updated_at = now();

INSERT INTO pet_guardians (id, tenant_id, pet_id, user_id, name, email, relationship, is_primary)
VALUES (
  '88888888-8888-8888-8888-888888888888',
  '11111111-1111-1111-1111-111111111111',
  '77777777-7777-7777-7777-777777777777',
  '44444444-4444-4444-4444-444444444444',
  'Avery Parker',
  'avery@pawit.example',
  'guardian',
  true
)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    email = EXCLUDED.email,
    relationship = EXCLUDED.relationship,
    is_primary = EXCLUDED.is_primary,
    updated_at = now();

INSERT INTO appointments (
  id,
  tenant_id,
  location_id,
  pet_id,
  requested_by_user_id,
  primary_veterinarian_user_id,
  type,
  status,
  starts_at,
  ends_at,
  reason
)
VALUES (
  '99999999-9999-9999-9999-999999999999',
  '11111111-1111-1111-1111-111111111111',
  '22222222-2222-2222-2222-222222222222',
  '77777777-7777-7777-7777-777777777777',
  '33333333-3333-3333-3333-333333333333',
  '55555555-5555-5555-5555-555555555555',
  'vaccination',
  'confirmed',
  now() + interval '1 day',
  now() + interval '1 day 30 minutes',
  'Rabies booster and wellness check'
)
ON CONFLICT (id) DO UPDATE
SET status = EXCLUDED.status,
    starts_at = EXCLUDED.starts_at,
    ends_at = EXCLUDED.ends_at,
    reason = EXCLUDED.reason,
    updated_at = now();

INSERT INTO appointment_veterinarians (appointment_id, veterinarian_user_id, is_primary)
VALUES (
  '99999999-9999-9999-9999-999999999999',
  '55555555-5555-5555-5555-555555555555',
  true
)
ON CONFLICT (appointment_id, veterinarian_user_id) DO UPDATE
SET is_primary = EXCLUDED.is_primary;

INSERT INTO invoices (
  id,
  tenant_id,
  location_id,
  pet_id,
  guardian_user_id,
  status,
  subtotal_cents,
  tax_cents,
  discount_cents,
  total_cents,
  due_at
)
VALUES (
  'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
  '11111111-1111-1111-1111-111111111111',
  '22222222-2222-2222-2222-222222222222',
  '77777777-7777-7777-7777-777777777777',
  '44444444-4444-4444-4444-444444444444',
  'issued',
  6500,
  520,
  0,
  7020,
  now() + interval '14 days'
)
ON CONFLICT (id) DO UPDATE
SET status = EXCLUDED.status,
    subtotal_cents = EXCLUDED.subtotal_cents,
    tax_cents = EXCLUDED.tax_cents,
    discount_cents = EXCLUDED.discount_cents,
    total_cents = EXCLUDED.total_cents,
    due_at = EXCLUDED.due_at,
    updated_at = now();

INSERT INTO invoice_line_items (
  id,
  invoice_id,
  tenant_id,
  description,
  quantity,
  unit_amount_cents,
  total_amount_cents
)
VALUES (
  'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
  'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
  '11111111-1111-1111-1111-111111111111',
  'Wellness exam',
  1,
  6500,
  6500
)
ON CONFLICT (id) DO UPDATE
SET description = EXCLUDED.description,
    quantity = EXCLUDED.quantity,
    unit_amount_cents = EXCLUDED.unit_amount_cents,
    total_amount_cents = EXCLUDED.total_amount_cents;

