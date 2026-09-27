-- +goose Up
CREATE TABLE candidate_resume_parses (
	id BIGSERIAL PRIMARY KEY,
	candidate_id BIGINT NOT NULL UNIQUE REFERENCES candidates(id) ON DELETE CASCADE,
	resume_file_id BIGINT NOT NULL REFERENCES resume_files(id),
	status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
	parser_version TEXT NOT NULL DEFAULT '',
	parsed_text TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	years_experience INTEGER,
	skills TEXT[] NOT NULL DEFAULT '{}',
	practice_areas TEXT[] NOT NULL DEFAULT '{}',
	jurisdictions TEXT[] NOT NULL DEFAULT '{}',
	education TEXT[] NOT NULL DEFAULT '{}',
	certifications TEXT[] NOT NULL DEFAULT '{}',
	languages TEXT[] NOT NULL DEFAULT '{}',
	error_message TEXT NOT NULL DEFAULT '',
	parsed_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX candidate_resume_parses_resume_file_id_idx ON candidate_resume_parses (resume_file_id);
CREATE INDEX candidate_resume_parses_status_idx ON candidate_resume_parses (status);
CREATE INDEX candidate_resume_parses_skills_idx ON candidate_resume_parses USING GIN (skills);
CREATE INDEX candidate_resume_parses_practice_areas_idx ON candidate_resume_parses USING GIN (practice_areas);
CREATE INDEX candidate_resume_parses_jurisdictions_idx ON candidate_resume_parses USING GIN (jurisdictions);
CREATE INDEX candidate_resume_parses_text_search_idx
	ON candidate_resume_parses USING GIN (to_tsvector('english', parsed_text));

-- +goose Down
DROP TABLE candidate_resume_parses;
