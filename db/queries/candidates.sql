-- name: ListCandidates :many
SELECT
	candidates.id,
	candidates.name,
	candidates.role,
	candidates.focus,
	candidates.score,
	candidates.status,
	candidates.created_at,
	candidates.updated_at,
	candidates.first_name,
	candidates.last_name,
	candidates.email,
	candidates.phone,
	candidates.experience,
	candidates.cover_letter,
	candidates.resume_url,
	candidates.source_job_posting_id,
	candidates.source_job_application_id,
	candidates.availability_status,
	candidates.contract_status,
	candidates.hired_at,
	COALESCE(job_postings.title, '') AS source_job_title,
	COALESCE(candidate_resume_parses.status, '') AS resume_parse_status,
	COALESCE(candidate_resume_parses.summary, '') AS resume_summary,
	candidate_resume_parses.years_experience,
	COALESCE(candidate_resume_parses.skills, '{}') AS skills,
	COALESCE(candidate_resume_parses.practice_areas, '{}') AS practice_areas,
	COALESCE(candidate_resume_parses.jurisdictions, '{}') AS jurisdictions,
	COALESCE(candidate_resume_parses.education, '{}') AS education,
	COALESCE(candidate_resume_parses.certifications, '{}') AS certifications,
	COALESCE(candidate_resume_parses.languages, '{}') AS languages,
	COALESCE(candidate_resume_parses.error_message, '') AS resume_parse_error,
	candidate_resume_parses.parsed_at
FROM candidates
LEFT JOIN job_postings ON job_postings.id = candidates.source_job_posting_id
LEFT JOIN candidate_resume_parses ON candidate_resume_parses.candidate_id = candidates.id
ORDER BY candidates.created_at DESC, candidates.id DESC;

-- name: CreateCandidate :one
INSERT INTO candidates (name, role, focus, score, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING
	id,
	name,
	role,
	focus,
	score,
	status,
	created_at,
	updated_at,
	first_name,
	last_name,
	email,
	phone,
	experience,
	cover_letter,
	resume_url,
	source_job_posting_id,
	source_job_application_id,
	availability_status,
	contract_status,
	hired_at;

-- name: CreateCandidateFromApplication :one
INSERT INTO candidates (
	name,
	role,
	focus,
	score,
	status,
	first_name,
	last_name,
	email,
	phone,
	experience,
	cover_letter,
	resume_url,
	source_job_posting_id,
	source_job_application_id,
	availability_status,
	contract_status,
	hired_at
)
VALUES ($1, $2, $3, 0, 'Hired', $4, $5, $6, $7, $8, $9, $10, $11, $12, 'available', 'not_started', NOW())
RETURNING
	id,
	name,
	role,
	focus,
	score,
	status,
	created_at,
	updated_at,
	first_name,
	last_name,
	email,
	phone,
	experience,
	cover_letter,
	resume_url,
	source_job_posting_id,
	source_job_application_id,
	availability_status,
	contract_status,
	hired_at;
