# legal-roster-api

Go backend for the Legal Bench Vue app in `../legal-bench`.

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
go run github.com/pressly/goose/v3/cmd/goose@latest -dir db/migrations postgres "$DATABASE_URL" up
```

## First Admin

The login page does not allow public account creation. Bootstrap the first `platform_admin` with these optional `.env` values:

```sh
BOOTSTRAP_PLATFORM_ADMIN_EMAIL=admin@example.com
BOOTSTRAP_PLATFORM_ADMIN_PASSWORD=change-this-strong-password
BOOTSTRAP_PLATFORM_ADMIN_NAME=Platform Admin
```

After the first platform admin is created, remove those bootstrap values from `.env`.

## User Management

A `platform_admin` can manage users from the Users panel. Inviting a user creates an activation link where the user sets their password. Users are disabled instead of deleted so account history stays intact.

When SMTP is configured, the API sends the activation email. In local development without SMTP, the API returns and logs the activation link.

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
OBJECT_STORAGE_BUCKET=legal-bench-resumes
OBJECT_STORAGE_ACCESS_KEY_ID=example-access-key
OBJECT_STORAGE_SECRET_ACCESS_KEY=example-secret-key
OBJECT_STORAGE_FORCE_PATH_STYLE=true
```

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

Run the Vue app from `../legal-bench` in a second terminal:

```sh
pnpm dev
```

The Vue dev server proxies `/api` requests to `http://localhost:8080`.

## Production

Build the Vue app first:

```sh
cd ../legal-bench
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
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `POST /api/auth/activate`
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
- `POST /api/admin/users/invitations`
- `PATCH /api/admin/users/{id}/role`
- `PATCH /api/admin/users/{id}/status`

## Tests

```sh
go test ./...
```
