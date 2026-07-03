#!/usr/bin/env bash
set -euo pipefail

cd /data/simplehpc/compose
set -a
. ./.env
set +a

export DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"
export REDIS_URL="redis://:${REDIS_PASSWORD}@127.0.0.1:${REDIS_PORT}/0"
export LDAP_URL="ldap://127.0.0.1:${LDAP_PORT}"
export SIMPLEHPC_ADDR="${SIMPLEHPC_ADDR:-127.0.0.1:18080}"
export SIMPLEHPC_FRONTEND_DIR="${SIMPLEHPC_FRONTEND_DIR:-/data/simplehpc/frontend}"
export SLURM_BIN_DIR="${SLURM_BIN_DIR:-/opt/slurm/current/bin}"
export SLURM_DEFAULT_ACCOUNT="${SLURM_DEFAULT_ACCOUNT:-simplehpc}"
export SLURM_DEFAULT_PARTITION="${SLURM_DEFAULT_PARTITION:-debug}"
export GIN_MODE="${GIN_MODE:-release}"

exec /data/simplehpc/backend-dev/simplehpc-backend
