#!/usr/bin/env bash
set -euo pipefail

job_id="${1:-}"
if [[ -z "$job_id" || ! "$job_id" =~ ^[0-9]+$ ]]; then
  echo "usage: $0 <numeric-slurm-job-id>" >&2
  exit 2
fi

conf="${SLURMDBD_CONF:-/etc/slurm/slurmdbd.conf}"
cluster="${SLURM_CLUSTER_NAME:-simplehpc-dev}"
if [[ ! "$cluster" =~ ^[A-Za-z0-9_.-]+$ ]]; then
  echo "invalid cluster name: $cluster" >&2
  exit 2
fi

value() {
  grep -E "^$1=" "$conf" | tail -n 1 | cut -d= -f2-
}

db="${SLURM_ACCT_DB:-$(value StorageLoc)}"
user="${SLURM_ACCT_USER:-$(value StorageUser)}"
pass="${SLURM_ACCT_PASS:-$(value StoragePass)}"
host="${SLURM_ACCT_HOST:-$(value StorageHost)}"
port="${SLURM_ACCT_PORT:-$(value StoragePort)}"

host="${host:-127.0.0.1}"
port="${port:-3306}"

sql=$(cat <<SQL
SELECT
  j.id_job,
  j.job_name,
  j.\`account\`,
  j.id_user,
  j.\`partition\`,
  j.\`state\`,
  j.cpus_req,
  j.mem_req,
  j.nodes_alloc,
  j.nodelist,
  FROM_UNIXTIME(NULLIF(j.time_submit, 0)) AS submit_time,
  FROM_UNIXTIME(NULLIF(j.time_start, 0)) AS start_time,
  FROM_UNIXTIME(NULLIF(j.time_end, 0)) AS end_time,
  TIMESTAMPDIFF(SECOND, FROM_UNIXTIME(NULLIF(j.time_submit, 0)), FROM_UNIXTIME(NULLIF(j.time_start, 0))) AS queue_seconds,
  CASE
    WHEN j.time_end > 0 THEN TIMESTAMPDIFF(SECOND, FROM_UNIXTIME(NULLIF(j.time_start, 0)), FROM_UNIXTIME(j.time_end))
    WHEN j.time_start > 0 THEN TIMESTAMPDIFF(SECOND, FROM_UNIXTIME(j.time_start), NOW())
    ELSE 0
  END AS run_seconds,
  j.work_dir,
  j.std_out,
  j.std_err,
  j.tres_req,
  j.tres_alloc,
  j.submit_line,
  s.batch_script
FROM \`${cluster}_job_table\` j
LEFT JOIN \`${cluster}_job_script_table\` s
  ON s.hash_inx = j.script_hash_inx
WHERE j.id_job = ${job_id}
  AND j.deleted = 0
ORDER BY j.job_db_inx DESC
LIMIT 1;
SQL
)

if command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}}' | grep -qx simplehpc-mariadb; then
  docker exec -i simplehpc-mariadb mariadb -u"$user" -p"$pass" -D "$db" --table <<<"$sql"
else
  mariadb -h "$host" -P "$port" -u"$user" -p"$pass" -D "$db" --table <<<"$sql"
fi
