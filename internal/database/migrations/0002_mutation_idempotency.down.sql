DROP TABLE IF EXISTS idempotency_keys;

ALTER TABLE appointments DROP CONSTRAINT IF EXISTS telemedicine_meeting_url_required_when_confirmed;

UPDATE appointments
SET telemedicine_url = 'about:blank'
WHERE type = 'telemedicine' AND telemedicine_url IS NULL;

ALTER TABLE appointments
  ADD CONSTRAINT appointments_check
  CHECK ((type = 'telemedicine' AND telemedicine_url IS NOT NULL) OR type <> 'telemedicine');
