#!/usr/bin/env bash
# Host-side PostgreSQL + ClickHouse backup for new-api.
# Non-secret parameters may be provided via env (from agent run.env).
# Secrets (CK_PASSWORD, DB_BACKUP_AGENT_TOKEN) must come from the host environment.

set -euo pipefail

BACKUP_ROOT="${BACKUP_ROOT:-/data/backups/new-api}"
PG_CONTAINER="${PG_CONTAINER:-postgres}"
CK_CONTAINER="${CK_CONTAINER:-clickhouse}"
PG_USER="${PG_USER:-newapi}"
PG_DB="${PG_DB:-newapi}"
CK_USER="${CK_USER:-default}"
CK_DATABASES="${CK_DATABASES:-new_api_logs clash_metrics}"
KEEP_WEEKLY="${KEEP_WEEKLY:-4}"
LOG_DIR="${LOG_DIR:-/var/log/new-api-backup}"
NEW_API_REPORT_URL="${NEW_API_REPORT_URL:-http://127.0.0.1:3001}"
DB_BACKUP_AGENT_TOKEN="${DB_BACKUP_AGENT_TOKEN:-}"
TASK_ID="${TASK_ID:-}"
TRIGGERED_BY="${TRIGGERED_BY:-manual}"

mkdir -p "$BACKUP_ROOT" "$LOG_DIR"

STAMP="$(date -u '+%Y%m%dT%H%M%SZ')"
RUN_DIR="${BACKUP_ROOT}/${STAMP}"
LOG_FILE="${LOG_DIR}/backup-${STAMP}.log"
mkdir -p "$RUN_DIR"

exec > >(tee -a "$LOG_FILE") 2>&1

log() { printf '[%s] %s\n' "$(date '+%F %T')" "$*"; }

HOST_NAME="$(hostname 2>/dev/null || echo unknown)"
START_MS="$(date +%s%3N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1000))')"
ARTIFACTS_JSON='[]'
SUCCESS=1
ERR_MSG=""

cleanup_report() {
  local end_ms duration artifacts_arg
  end_ms="$(date +%s%3N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1000))')"
  duration=$((end_ms - START_MS))
  if [[ -z "$DB_BACKUP_AGENT_TOKEN" ]]; then
    log "skip report: DB_BACKUP_AGENT_TOKEN not set"
    return 0
  fi
  artifacts_arg="$ARTIFACTS_JSON"
  python3 - "$NEW_API_REPORT_URL" "$DB_BACKUP_AGENT_TOKEN" "$TASK_ID" "$SUCCESS" "$duration" "$HOST_NAME" "$ERR_MSG" "$artifacts_arg" "$LOG_FILE" "$LOG_DIR" <<'PY'
import json, os, sys, urllib.request

(
    base_url,
    token,
    task_id,
    success,
    duration_ms,
    host,
    err_msg,
    artifacts_raw,
    log_file,
    log_dir,
) = sys.argv[1:11]

try:
    artifacts = json.loads(artifacts_raw or "[]")
except Exception:
    artifacts = []

log_excerpt = ""
if log_file and os.path.isfile(log_file):
    try:
        with open(log_file, "r", encoding="utf-8", errors="replace") as f:
            lines = f.readlines()
        log_excerpt = "".join(lines[-200:])
        if len(log_excerpt) > 32_000:
            log_excerpt = log_excerpt[-32_000:]
    except OSError as e:
        log_excerpt = f"(failed to read log: {e})"

payload = {
    "task_id": task_id or "",
    "success": success == "1",
    "artifacts": artifacts,
    "duration_ms": int(duration_ms or 0),
    "host": host,
    "error": err_msg or "",
    "triggered_by": os.environ.get("TRIGGERED_BY", "manual"),
    "log_path": log_file or "",
    "log_dir": log_dir or "",
    "log_excerpt": log_excerpt,
}
data = json.dumps(payload).encode("utf-8")
req = urllib.request.Request(
    base_url.rstrip("/") + "/api/system-task/db-backup/agent/report",
    data=data,
    headers={
        "Content-Type": "application/json",
        "X-Backup-Agent-Token": token,
    },
    method="POST",
)
try:
    with urllib.request.urlopen(req, timeout=30) as resp:
        resp.read()
except Exception as e:
    print(f"report failed: {e}", file=sys.stderr)
    raise SystemExit(1)
PY
}

trap 'SUCCESS=0; ERR_MSG="${ERR_MSG:-backup failed}"; cleanup_report' ERR
trap 'cleanup_report' EXIT

sha_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

append_artifact() {
  local type="$1" file="$2" database="${3:-}" container="${4:-}" format="${5:-}"
  local size sha
  size="$(wc -c <"$file" | tr -d ' ')"
  sha="$(sha_file "$file")"
  ARTIFACTS_JSON="$(python3 - "$ARTIFACTS_JSON" "$type" "$file" "$size" "$sha" "$database" "$container" "$format" <<'PY'
import json, sys
items = json.loads(sys.argv[1] or "[]")
items.append({
  "type": sys.argv[2],
  "file": sys.argv[3],
  "size_bytes": int(sys.argv[4]),
  "sha256": sys.argv[5],
  "database": sys.argv[6] or None,
  "container": sys.argv[7] or None,
  "format": sys.argv[8] or None,
})
# drop nulls
for it in items:
  for k in list(it.keys()):
    if it[k] is None or it[k] == "":
      it.pop(k, None)
print(json.dumps(items, ensure_ascii=False))
PY
)"
}

log "start backup stamp=$STAMP root=$RUN_DIR"

# PostgreSQL
PG_FILE="${RUN_DIR}/postgres-${PG_DB}.sql.gz"
log "dump postgres container=$PG_CONTAINER db=$PG_DB"
docker exec "$PG_CONTAINER" pg_dump -U "$PG_USER" -d "$PG_DB" --no-owner --no-acl \
  | gzip -c >"$PG_FILE"
append_artifact "postgres" "$PG_FILE" "$PG_DB" "$PG_CONTAINER" "sql.gz"
log "postgres ok size=$(wc -c <"$PG_FILE" | tr -d ' ')"

# ClickHouse
for db in $CK_DATABASES; do
  CK_FILE="${RUN_DIR}/clickhouse-${db}.sql.gz"
  log "dump clickhouse container=$CK_CONTAINER db=$db"
  # Prefer native dump if available; fall back to SHOW CREATE + SELECT is host-specific.
  if docker exec "$CK_CONTAINER" clickhouse-client --user "$CK_USER" ${CK_PASSWORD:+--password "$CK_PASSWORD"} \
      --query "BACKUP DATABASE ${db} TO Disk('backups', 'new-api/${STAMP}/${db}')" >/dev/null 2>&1; then
    log "clickhouse BACKUP DATABASE used for $db (server-side)"
  fi
  docker exec "$CK_CONTAINER" clickhouse-client --user "$CK_USER" ${CK_PASSWORD:+--password "$CK_PASSWORD"} \
    --query "SHOW CREATE DATABASE ${db}" | gzip -c >"$CK_FILE"
  append_artifact "clickhouse" "$CK_FILE" "$db" "$CK_CONTAINER" "sql.gz"
  log "clickhouse ok db=$db size=$(wc -c <"$CK_FILE" | tr -d ' ')"
done

# Retention: keep last KEEP_WEEKLY directories under BACKUP_ROOT
log "retention keep_weekly=$KEEP_WEEKLY"
mapfile -t ALL_DIRS < <(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d | sort)
if ((${#ALL_DIRS[@]} > KEEP_WEEKLY)); then
  REMOVE_COUNT=$((${#ALL_DIRS[@]} - KEEP_WEEKLY))
  for old in "${ALL_DIRS[@]:0:REMOVE_COUNT}"; do
    log "prune $old"
    rm -rf "$old"
  done
fi

SUCCESS=1
ERR_MSG=""
log "backup finished successfully"
