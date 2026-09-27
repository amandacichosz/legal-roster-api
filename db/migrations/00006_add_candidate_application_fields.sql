-- +goose Up
ALTER TABLE candidates
ADD COLUMN first_name TEXT NOT NULL DEFAULT '',
ADD COLUMN last_name TEXT NOT NULL DEFAULT '',
ADD COLUMN email TEXT NOT NULL DEFAULT '',
ADD COLUMN phone TEXT NOT NULL DEFAULT '',
ADD COLUMN experience TEXT NOT NULL DEFAULT '',
ADD COLUMN cover_letter TEXT NOT NULL DEFAULT '',
ADD COLUMN resume_url TEXT NOT NULL DEFAULT '',
ADD COLUMN source_job_posting_id BIGINT REFERENCES job_postings(id),
ADD COLUMN source_job_application_id BIGINT,
ADD COLUMN availability_status TEXT NOT NULL DEFAULT 'available',
ADD COLUMN contract_status TEXT NOT NULL DEFAULT 'not_started',
ADD COLUMN hired_at TIMESTAMPTZ;

CREATE INDEX candidates_email_idx ON candidates (LOWER(email));
CREATE INDEX candidates_availability_status_idx ON candidates (availability_status);
CREATE INDEX candidates_contract_status_idx ON candidates (contract_status);

-- +goose Down
DROP INDEX candidates_contract_status_idx;
DROP INDEX candidates_availability_status_idx;
DROP INDEX candidates_email_idx;

ALTER TABLE candidates
DROP COLUMN hired_at,
DROP COLUMN contract_status,
DROP COLUMN availability_status,
DROP COLUMN source_job_application_id,
DROP COLUMN source_job_posting_id,
DROP COLUMN resume_url,
DROP COLUMN cover_letter,
DROP COLUMN experience,
DROP COLUMN phone,
DROP COLUMN email,
DROP COLUMN last_name,
DROP COLUMN first_name;
