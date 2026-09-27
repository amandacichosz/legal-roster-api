-- +goose Up
CREATE TABLE job_postings (
	id BIGSERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	department TEXT NOT NULL,
	location TEXT NOT NULL,
	employment_type TEXT NOT NULL,
	summary TEXT NOT NULL,
	description TEXT NOT NULL,
	requirements TEXT NOT NULL,
	is_active BOOLEAN NOT NULL DEFAULT TRUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE job_applications (
	id BIGSERIAL PRIMARY KEY,
	job_posting_id BIGINT NOT NULL REFERENCES job_postings(id),
	first_name TEXT NOT NULL,
	last_name TEXT NOT NULL,
	email TEXT NOT NULL,
	phone TEXT NOT NULL,
	experience TEXT NOT NULL,
	cover_letter TEXT NOT NULL DEFAULT '',
	resume_url TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX job_postings_active_idx ON job_postings (is_active, slug);
CREATE INDEX job_applications_job_posting_created_at_idx ON job_applications (job_posting_id, created_at DESC);

INSERT INTO job_postings (
	title,
	slug,
	department,
	location,
	employment_type,
	summary,
	description,
	requirements,
	is_active
) VALUES
(
	'Paralegal',
	'paralegal',
	'Legal Operations',
	'Remote',
	'Full-time',
	'Support case preparation, document review, filings, and client communication for a growing legal operations team.',
	'The Paralegal role helps attorneys and legal teams stay organized through matter intake, drafting support, deadline tracking, document collection, and careful case file maintenance.',
	'Strong written communication; experience with legal documents or case management; high attention to detail; comfort managing confidential information; ability to prioritize multiple deadlines.',
	TRUE
),
(
	'Intake Specialist',
	'intake-specialist',
	'Client Intake',
	'Remote',
	'Full-time',
	'Guide prospective clients through initial intake, collect key facts, and route matters to the right legal team.',
	'The Intake Specialist is the first point of contact for many applicants and clients. This role gathers matter details, documents outcomes, schedules next steps, and maintains a professional client experience.',
	'Clear phone and written communication; customer service or intake experience; accurate note-taking; good judgment with sensitive information; reliable follow-through.',
	TRUE
);

-- +goose Down
DROP TABLE job_applications;
DROP TABLE job_postings;
