-- +goose Up
UPDATE job_applications
SET status = 'rejected'
WHERE status = 'declined';

ALTER TABLE job_applications DROP CONSTRAINT job_applications_status_check;
ALTER TABLE job_applications ADD CONSTRAINT job_applications_status_check CHECK (status IN ('applied', 'in_review', 'rejected'));

CREATE INDEX IF NOT EXISTS job_applications_status_id_idx ON job_applications (status, id DESC);

-- +goose Down
UPDATE job_applications
SET status = 'declined'
WHERE status = 'rejected';

UPDATE job_applications
SET status = 'in_progress'
WHERE status = 'in_review';

ALTER TABLE job_applications DROP CONSTRAINT job_applications_status_check;
ALTER TABLE job_applications ADD CONSTRAINT job_applications_status_check CHECK (status IN ('applied', 'in_progress', 'declined'));

DROP INDEX IF EXISTS job_applications_status_id_idx;
