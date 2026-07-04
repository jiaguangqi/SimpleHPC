# SimpleHPC Backend

Go backend for the SimpleHPC prototype. The first implementation targets the test server services documented in `../docs/BACKEND_SERVICES_DEPLOYMENT.md`.

## Data Authenticity Regression

Run the frontend static-data audit from the project root:

```bash
node --test tests/static-data-audit.test.js
```

The audit rejects known simulated cluster records so they cannot silently return
to production pages.

## Run Locally

```bash
cd backend
cp .env.example .env
# fill .env from the local credentials document or server environment
set -a
. ./.env
set +a
go run ./cmd/server
```

When running on the test server, set `SIMPLEHPC_FRONTEND_DIR` to the directory containing `index.html`.

## First API Surface

- `GET /api/health`
- `GET /api/v1/overview`
- `POST /api/v1/ldap/bootstrap`
- `GET /api/v1/ldap/users`
- `GET /api/v1/slurm/nodes`
- `GET /api/v1/slurm/jobs`
- `GET /api/v1/slurm/jobs/history?since=today`
- `POST /api/v1/inspection/run`
- `GET /api/v1/storage/roots`
- `PUT /api/v1/storage/roots`
- `GET /api/v1/storage/list?path=/data/home&showHidden=false`
- `POST /api/v1/storage/directory`
- `POST /api/v1/storage/upload`
- `POST /api/v1/storage/copy`
- `POST /api/v1/storage/move`
- `POST /api/v1/storage/delete`
- `GET /api/v1/storage/download?path=/data/home/user001/file.txt`
- `POST /api/v1/storage/archive`

## Migration

The first PostgreSQL schema is in `migrations/001_init.sql`. Apply it on the server with `psql` before enabling write features.

## Test Server Notes

The current dev binary was verified on `10.10.38.152` from `/data/simplehpc/backend-dev/simplehpc-backend`.

On the test server, a repeatable dev launch can use:

```bash
/data/simplehpc/backend-dev/run-dev-server.sh
```

The script reads `/data/simplehpc/compose/.env` and builds the runtime connection URLs without storing secrets in the repository.
