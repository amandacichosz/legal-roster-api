-- +goose Up
ALTER TABLE job_applications
ADD COLUMN status TEXT NOT NULL DEFAULT 'applied',
ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
ADD CONSTRAINT job_applications_status_check CHECK (status IN ('applied', 'in_progress', 'declined'));

CREATE INDEX job_applications_status_created_at_idx ON job_applications (status, created_at DESC);

-- +goose Down
DROP INDEX job_applications_status_created_at_idx;
ALTER TABLE job_applications
DROP CONSTRAINT job_applications_status_check,
DROP COLUMN updated_at,
DROP COLUMN status;
