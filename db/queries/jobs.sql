-- name: ListJobPostings :many
SELECT
	job_postings.id,
	job_postings.title,
	job_postings.slug,
	job_postings.department,
	job_postings.location,
	job_postings.employment_type,
	job_postings.summary,
	job_postings.description,
	job_postings.requirements,
	job_postings.is_active,
	job_postings.created_at,
	job_postings.updated_at,
	COUNT(job_applications.id)::bigint AS application_count
FROM job_postings
LEFT JOIN job_applications ON job_applications.job_posting_id = job_postings.id
GROUP BY job_postings.id
ORDER BY job_postings.created_at DESC, job_postings.id DESC;

-- name: GetJobPostingBySlug :one
SELECT id, title, slug, department, location, employment_type, summary, description, requirements, is_active, created_at, updated_at
FROM job_postings
WHERE slug = $1;

-- name: UpdateJobPostingStatus :one
UPDATE job_postings
SET is_active = $2, updated_at = NOW()
WHERE job_postings.id = $1
RETURNING
	job_postings.id,
	job_postings.title,
	job_postings.slug,
	job_postings.department,
	job_postings.location,
	job_postings.employment_type,
	job_postings.summary,
	job_postings.description,
	job_postings.requirements,
	job_postings.is_active,
	job_postings.created_at,
	job_postings.updated_at,
	(
		SELECT COUNT(job_applications.id)::bigint
		FROM job_applications
		WHERE job_applications.job_posting_id = job_postings.id
	) AS application_count;

-- name: CreateJobApplication :one
INSERT INTO job_applications (
	job_posting_id,
	first_name,
	last_name,
	email,
	phone,
	experience,
	cover_letter,
	resume_url
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, job_posting_id, first_name, last_name, email, phone, experience, cover_letter, resume_url, status, created_at, updated_at;

-- name: ListJobApplications :many
SELECT
	job_applications.id,
	job_applications.job_posting_id,
	job_postings.title AS job_title,
	job_postings.slug AS job_slug,
	job_applications.first_name,
	job_applications.last_name,
	job_applications.email,
	job_applications.phone,
	job_applications.experience,
	job_applications.cover_letter,
	job_applications.resume_url,
	job_applications.status,
	job_applications.created_at,
	job_applications.updated_at
FROM job_applications
JOIN job_postings ON job_postings.id = job_applications.job_posting_id
WHERE ($1 = '' OR job_applications.status = $1)
ORDER BY job_applications.created_at DESC, job_applications.id DESC
LIMIT $2 OFFSET $3;

-- name: CountJobApplications :one
SELECT COUNT(*)::bigint
FROM job_applications
WHERE ($1 = '' OR status = $1);

-- name: GetJobApplicationForHire :one
SELECT
	job_applications.id,
	job_applications.job_posting_id,
	job_postings.title AS job_title,
	job_applications.first_name,
	job_applications.last_name,
	job_applications.email,
	job_applications.phone,
	job_applications.experience,
	job_applications.cover_letter,
	job_applications.resume_url,
	job_applications.status
FROM job_applications
JOIN job_postings ON job_postings.id = job_applications.job_posting_id
WHERE job_applications.id = $1;

-- name: UpdateJobApplicationStatus :one
UPDATE job_applications
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, job_posting_id, first_name, last_name, email, phone, experience, cover_letter, resume_url, status, created_at, updated_at;


-- name: DeleteJobApplication :exec
DELETE FROM job_applications
WHERE id = $1;
