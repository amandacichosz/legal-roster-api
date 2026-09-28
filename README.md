# legal-roster-api

Go backend for the Legal Bench Vue app in `../legal-roster`.

The service uses PostgreSQL with `pgx`, SQLC-generated query code, Goose migrations,
and Docker Compose for local database development. In production, point `DATABASE_URL`
at a managed Postgres database.

## Stack

- Database: PostgreSQL
- Go driver: `github.com/jackc/pgx/v5`
- Query layer: SQLC
- Migrations: Goose
- Local database: Docker Compose
- Production database: managed Postgres

## Setup

Install Go, Docker, SQLC, and Goose. Then download Go dependencies:

```sh
go mod download
```

Copy the environment file if needed:

```sh
cp .env.example .env
```

## Local Database

Start Postgres:

```sh
docker compose up -d postgres
```

Run migrations:

```sh
export $(grep -v '^#' .env | xargs)
go run github.com/pressly/goose/v3/cmd/goose -dir db/migrations postgres "$DATABASE_URL" up
```

## Supabase Database

The API can run against Supabase Postgres by setting `DATABASE_URL` to the shared
pooler connection string with TLS enabled. Use session mode on port `5432` for
migrations and other administrative commands:

```sh
export DATABASE_URL='postgresql://postgres.imbsitvagnpgsubgcyql:YOUR-PERCENT-ENCODED-PASSWORD@aws-0-us-west-2.pooler.supabase.com:5432/postgres?sslmode=require'
```

If the database password contains special characters, percent-encode them before
placing it in the URL.

Run the existing Goose migrations against Supabase:

```sh
go run github.com/pressly/goose/v3/cmd/goose -dir db/migrations postgres "$DATABASE_URL" up
```

For the production API, either keep session mode or use the shared transaction
pooler by changing the port to `6543`. Keep the local Docker database URL in
`.env` for local-only development if you do not want your development server
writing to Supabase.

## Supabase Auth

The frontend signs users in with Supabase Auth and sends the access token to the
Go API. Configure the API with:

```sh
SUPABASE_URL=https://YOUR_PROJECT_REF.supabase.co
SUPABASE_PUBLISHABLE_KEY=sb_publishable_...
```

Configure the Vue frontend with the corresponding `VITE_SUPABASE_URL` and
`VITE_SUPABASE_PUBLISHABLE_KEY` values. Keep public sign-up disabled. New identities
created through Supabase Auth are synchronized into `public.users` as disabled
admins and must be enabled by an existing administrator.

## User Management

Both `platform_admin` and `admin` users can access the admin section. They can
assign either role and enable or disable application access from the Users panel.

## Resume Object Storage

Resumes are stored as private objects and exposed through authenticated API download URLs. Do not make the bucket public.

Local development uses a private directory inside the API project:

```sh
OBJECT_STORAGE_PROVIDER=local
OBJECT_STORAGE_LOCAL_DIR=data/private-objects
```

Production should use an S3-compatible private bucket, for example AWS S3, Cloudflare R2, or MinIO:

```sh
OBJECT_STORAGE_PROVIDER=s3
OBJECT_STORAGE_ENDPOINT=https://s3.amazonaws.com
OBJECT_STORAGE_REGION=us-east-1
OBJECT_STORAGE_BUCKET=legal-roster-resumes
OBJECT_STORAGE_ACCESS_KEY_ID=example-access-key
OBJECT_STORAGE_SECRET_ACCESS_KEY=example-secret-key
OBJECT_STORAGE_FORCE_PATH_STYLE=true
```

Cloudflare R2 can also be configured directly with its standard values:

```sh
R2_ACCOUNT_ID=example-account-id
R2_BUCKET=legal-roster-resumes
R2_ACCESS_KEY_ID=example-access-key
R2_SECRET_ACCESS_KEY=example-secret-key
```

When these values are present, the API automatically uses the R2 S3 endpoint.
To copy existing local resume objects to R2 while preserving their storage keys:

```sh
go run ./cmd/migrate-resumes
```

The command verifies every uploaded object and keeps the local files intact.

Security expectations:

- Keep the bucket private.
- Use a dedicated IAM/service account with least privilege to only the resume bucket/prefix.
- Enable provider-side encryption at rest.
- Rotate access keys and store them in a secrets manager in production.
- Serve downloads only through `GET /api/admin/resumes/{publicID}/download`.

## Email

Configure SMTP to send invitation emails:

```sh
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USERNAME=apikey-or-user
SMTP_PASSWORD=example-password
EMAIL_FROM=no-reply@example.com
```

## SQLC

Regenerate database code after editing files in `db/queries` or `db/migrations`:

```sh
go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
```

The generated Go package lives in `internal/db`.

## Development

Run the API:

```sh
go run ./cmd/server
```

Run the Vue app from `../legal-roster` in a second terminal:

```sh
pnpm dev
```

The Vue dev server proxies `/api` requests to `http://localhost:8080`.

## Production

Build the Vue app first:

```sh
cd ../legal-roster
pnpm build
```

Then build and run the API:

```sh
cd ../legal-roster-api
go build -o bin/legal-roster-api ./cmd/server
./bin/legal-roster-api
```

Set `DATABASE_URL` to your managed Postgres connection string in production.

## Endpoints

- `GET /api/health`
- `GET /api/auth/me`
- `GET /api/candidates`
- `POST /api/candidates`
- `GET /api/jobs/{slug}`
- `POST /api/jobs/{slug}/applications`
- `GET /api/admin/jobs`
- `PATCH /api/admin/jobs/{id}/status`
- `GET /api/admin/applicants`
- `GET /api/admin/resumes/{publicID}/download`
- `PATCH /api/admin/applicants/{id}/status`
- `POST /api/admin/applicants/{id}/hire`
- `GET /api/admin/users`
- `PATCH /api/admin/users/{id}/role`
- `PATCH /api/admin/users/{id}/status`

## Tests

```sh
go test ./...
```

## GitHub Actions and Render

The API workflow provisions PostgreSQL 17, applies every Goose migration, runs
the Go tests, and builds the server. On successful pushes to `master`, it then
applies pending migrations to production Supabase and triggers a Render deploy.
Pull requests run verification without modifying production or deploying.

In this GitHub repository, create a production environment and add the secret
`PRODUCTION_DATABASE_URL` using the Supabase session pooler connection string on
port `5432`. Add `RENDER_DEPLOY_HOOK_URL` using the deploy hook from the API's
Render service. Consider requiring reviewers on the production environment.
Disable Render's automatic Git deploys so an unverified push cannot bypass the
GitHub Actions gate.
