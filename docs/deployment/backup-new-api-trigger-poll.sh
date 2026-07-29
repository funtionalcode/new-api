#!/usr/bin/env bash
# Poll new-api for db_backup config/script bundle + pending tasks.
# Materializes non-secret env and optional script, then runs host backup.

set -euo pipefail

DB_BACKUP_AGENT_TOKEN="${DB_BACKUP_AGENT_TOKEN:-}"
NEW_API_REPORT_URL="${NEW_API_REPORT_URL:-http://127.0.0.1:3001}"
BACKUP_SCRIPT="${BACKUP_SCRIPT:-/usr/local/bin/backup-new-api-db.sh}"
LOG_DIR="${LOG_DIR:-/var/log/new-api-backup}"
POLL_LOCK_FILE="${POLL_LOCK_FILE:-/var/lock/backup-new-api-trigger-poll.lock}"
STATE_DIR="${STATE_DIR:-/var/lib/new-api-backup}"
RUN_ENV_FILE="${RUN_ENV_FILE:-${STATE_DIR}/run.env}"
SCRIPT_SHA_FILE="${SCRIPT_SHA_FILE:-${STATE_DIR}/script.sha256}"

mkdir -p "$LOG_DIR" "$STATE_DIR"

log() { printf '[%s] %s\n' "$(date '+%F %T')" "$*"; }

if [[ -z "$DB_BACKUP_AGENT_TOKEN" ]]; then
  log "skip poll: DB_BACKUP_AGENT_TOKEN not set"
  exit 0
fi

if [[ ! -x "$BACKUP_SCRIPT" ]]; then
  log "ERROR: backup script missing or not executable: $BACKUP_SCRIPT"
  exit 1
fi

exec 8>"$POLL_LOCK_FILE"
if ! flock -n 8; then
  log "another poll is already running"
  exit 0
fi

if ! command -v curl >/dev/null 2>&1; then
  log "ERROR: curl is required"
  exit 1
fi
if ! command -v python3 >/dev/null 2>&1; then
  log "ERROR: python3 is required"
  exit 1
fi

local_sha=""
if [[ -f "$SCRIPT_SHA_FILE" ]]; then
  local_sha="$(tr -d '[:space:]' < "$SCRIPT_SHA_FILE" || true)"
fi

bundle_payload="$(python3 -c 'import json,sys; print(json.dumps({"local_script_sha256": sys.argv[1]}))' "$local_sha")"
bundle_response="$(curl -fsS -X POST \
  -H "X-Backup-Agent-Token: ${DB_BACKUP_AGENT_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "$bundle_payload" \
  "${NEW_API_REPORT_URL%/}/api/system-task/db-backup/agent/bundle" || true)"

if [[ -n "$bundle_response" ]]; then
  python3 - "$bundle_response" "$RUN_ENV_FILE" "$BACKUP_SCRIPT" "$SCRIPT_SHA_FILE" <<'PY'
import json, os, sys, tempfile

raw, run_env, script_path, sha_file = sys.argv[1:5]
try:
    data = json.loads(raw)
except Exception as e:
    print(f"bundle parse failed: {e}", file=sys.stderr)
    raise SystemExit(0)

payload = data.get("data") if isinstance(data, dict) else None
if not isinstance(payload, dict):
    raise SystemExit(0)

config = payload.get("config") or {}
if isinstance(config, dict):
    lines = []
    for k, v in config.items():
        if not isinstance(k, str) or not isinstance(v, str):
            continue
        if k in {"CK_PASSWORD", "DB_BACKUP_AGENT_TOKEN", "NEW_API_REPORT_URL"}:
            continue
        # shell-safe single-quoted value
        safe = v.replace("'", "'\"'\"'")
        lines.append(f"{k}='{safe}'")
    tmp = run_env + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        f.write("\n".join(lines) + ("\n" if lines else ""))
    os.replace(tmp, run_env)

if not payload.get("script_apply"):
    raise SystemExit(0)

sha = payload.get("script_sha256") or ""
if payload.get("script_unchanged"):
    raise SystemExit(0)

content = payload.get("script_content") or ""
if not content or not sha:
    raise SystemExit(0)

# Fixed path only; refuse unexpected target.
if os.path.abspath(script_path) != "/usr/local/bin/backup-new-api-db.sh":
    print(f"refusing unexpected script path: {script_path}", file=sys.stderr)
    raise SystemExit(1)

dir_name = os.path.dirname(script_path)
fd, tmp_path = tempfile.mkstemp(prefix="backup-script.", dir=dir_name)
try:
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        f.write(content)
        if not content.endswith("\n"):
            f.write("\n")
    os.chmod(tmp_path, 0o755)
    os.replace(tmp_path, script_path)
finally:
    if os.path.exists(tmp_path):
        try:
            os.unlink(tmp_path)
        except OSError:
            pass

with open(sha_file, "w", encoding="utf-8") as f:
    f.write(sha + "\n")
print(f"materialized script sha256={sha}")
PY
else
  log "bundle request failed or empty; continue with local config if any"
fi

response="$(curl -fsS -X POST \
  -H "X-Backup-Agent-Token: ${DB_BACKUP_AGENT_TOKEN}" \
  -H 'Content-Type: application/json' \
  "${NEW_API_REPORT_URL%/}/api/system-task/db-backup/agent/pending" || true)"

if [[ -z "$response" ]]; then
  log "poll request failed or empty response"
  exit 0
fi

task_id="$(printf '%s' "$response" | python3 -c 'import json,sys
try:
    data=json.load(sys.stdin)
except Exception:
    print("")
    raise SystemExit(0)
payload=data.get("data") if isinstance(data, dict) else None
if not isinstance(payload, dict):
    print("")
    raise SystemExit(0)
print(payload.get("task_id") or "")
')"

if [[ -z "$task_id" ]]; then
  exit 0
fi

log "claimed pending db_backup task_id=$task_id"
export TASK_ID="$task_id"
export DB_BACKUP_AGENT_TOKEN
export NEW_API_REPORT_URL

if [[ -f "$RUN_ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  set -a
  # Do not let empty keys wipe secrets from the cron environment.
  # shellcheck disable=SC1090
  source "$RUN_ENV_FILE"
  set +a
fi

"$BACKUP_SCRIPT"
