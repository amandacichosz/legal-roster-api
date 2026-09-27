-- name: CreateResumeFile :one
INSERT INTO resume_files (public_id, storage_key, original_filename, content_type, size_bytes)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, public_id, storage_key, original_filename, content_type, size_bytes, created_at;

-- name: GetResumeFileByPublicID :one
SELECT id, public_id, storage_key, original_filename, content_type, size_bytes, created_at
FROM resume_files
WHERE public_id = $1;

-- name: StartCandidateResumeParse :one
INSERT INTO candidate_resume_parses (
	candidate_id,
	resume_file_id,
	status,
	parser_version,
	error_message
)
VALUES ($1, $2, 'processing', $3, '')
ON CONFLICT (candidate_id) DO UPDATE
SET
	resume_file_id = EXCLUDED.resume_file_id,
	status = 'processing',
	parser_version = EXCLUDED.parser_version,
	error_message = '',
	updated_at = NOW()
RETURNING id, candidate_id, resume_file_id, status, parser_version, parsed_text, summary, email, phone, years_experience, skills, practice_areas, jurisdictions, education, certifications, languages, error_message, parsed_at, created_at, updated_at;

-- name: CompleteCandidateResumeParse :one
UPDATE candidate_resume_parses
SET
	status = 'completed',
	parser_version = $2,
	parsed_text = $3,
	summary = $4,
	email = $5,
	phone = $6,
	years_experience = $7,
	skills = $8,
	practice_areas = $9,
	jurisdictions = $10,
	education = $11,
	certifications = $12,
	languages = $13,
	error_message = '',
	parsed_at = NOW(),
	updated_at = NOW()
WHERE candidate_id = $1
RETURNING id, candidate_id, resume_file_id, status, parser_version, parsed_text, summary, email, phone, years_experience, skills, practice_areas, jurisdictions, education, certifications, languages, error_message, parsed_at, created_at, updated_at;

-- name: FailCandidateResumeParse :one
UPDATE candidate_resume_parses
SET
	status = 'failed',
	error_message = $2,
	updated_at = NOW()
WHERE candidate_id = $1
RETURNING id, candidate_id, resume_file_id, status, parser_version, parsed_text, summary, email, phone, years_experience, skills, practice_areas, jurisdictions, education, certifications, languages, error_message, parsed_at, created_at, updated_at;
