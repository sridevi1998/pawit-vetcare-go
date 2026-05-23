ALTER TABLE appointments DROP CONSTRAINT IF EXISTS appointments_check;

ALTER TABLE appointments
  ADD CONSTRAINT telemedicine_meeting_url_required_when_confirmed
  CHECK (
    type <> 'telemedicine'
    OR status IN ('requested', 'needs_reschedule', 'cancelled', 'rejected')
    OR telemedicine_url IS NOT NULL
  );

CREATE TABLE idempotency_keys (
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  key text NOT NULL,
  request_hash text NOT NULL,
  response_status integer NOT NULL,
  response_body jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL DEFAULT now() + interval '24 hours',
  PRIMARY KEY (tenant_id, key)
);

CREATE INDEX idx_idempotency_keys_expires ON idempotency_keys(expires_at);
