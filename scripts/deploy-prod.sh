#!/usr/bin/env bash
set -Eeuo pipefail

MODE="dry-run"
HOST="paycn"
REMOTE_ROOT="/opt/dst-waystone"
REF="main"

usage() {
  cat <<'USAGE'
Usage: scripts/deploy-prod.sh [--dry-run|--execute] [--host paycn] [--path /opt/dst-waystone] [--ref main]

Default mode is --dry-run. Use --execute to mutate production.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      MODE="dry-run"
      shift
      ;;
    --execute)
      MODE="execute"
      shift
      ;;
    --host)
      HOST="${2:?missing --host value}"
      shift 2
      ;;
    --path)
      REMOTE_ROOT="${2:?missing --path value}"
      shift 2
      ;;
    --ref)
      REF="${2:?missing --ref value}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "${HOST}${REMOTE_ROOT}${REF}" in
  *"'"*)
    echo "host/path/ref must not contain single quotes" >&2
    exit 2
    ;;
esac

REMOTE_DOCKER_DIR="${REMOTE_ROOT}/docker"
BACKUP_DIR="${REMOTE_ROOT}/backups/$(date +%Y%m%d-%H%M%S)"

run() {
  printf '+ %s\n' "$*"
  if [[ "$MODE" == "execute" ]]; then
    "$@"
  fi
}

remote() {
  local command="$1"
  run ssh "$HOST" "$command"
}

echo "[deploy] mode=${MODE} host=${HOST} path=${REMOTE_ROOT} ref=${REF}"
if [[ "$MODE" == "dry-run" ]]; then
  echo "[deploy] dry-run only; pass --execute to deploy."
fi

remote "set -e; test -d '${REMOTE_ROOT}'; test -d '${REMOTE_DOCKER_DIR}'; cd '${REMOTE_ROOT}'; echo current_sha=\$(git rev-parse --short HEAD); git status --short"

remote "set -e; mkdir -p '${BACKUP_DIR}'; chmod 700 '${BACKUP_DIR}'; cd '${REMOTE_DOCKER_DIR}'; cp -p .env '${BACKUP_DIR}/env' 2>/dev/null || true; docker cp dst-waystone:/data/admin/dst-admin.db '${BACKUP_DIR}/dst-admin.db' 2>/dev/null || true; docker cp dst-waystone:/data/cluster/Cluster_1 '${BACKUP_DIR}/Cluster_1' 2>/dev/null || true; echo backup_dir='${BACKUP_DIR}'"

remote "set -e; cd '${REMOTE_ROOT}'; git fetch --all --prune; if git show-ref --verify --quiet 'refs/remotes/origin/${REF}'; then git checkout '${REF}'; git merge --ff-only 'origin/${REF}'; else git checkout '${REF}'; fi; git rev-parse --short HEAD"

remote "set -e; cd '${REMOTE_DOCKER_DIR}'; docker compose build dst-waystone"

remote "set -e; cd '${REMOTE_DOCKER_DIR}'; docker compose up -d --no-deps dst-waystone"

remote "set -e; cd '${REMOTE_DOCKER_DIR}'; docker compose ps; docker images dst-waystone:local --format '{{.ID}} {{.Size}} {{.CreatedSince}}'; docker exec dst-waystone supervisorctl -c /opt/dst/runtime/supervisord.conf status; docker exec dst-waystone sh -lc 'test -f /data/admin/dst-admin.db && echo db=present || echo db=missing; test -x /opt/dst/game/bin64/dontstarve_dedicated_server_nullrenderer_x64 && echo binary=present || echo binary=missing'; ss -lunp | grep -E ':(10999|11000|12346|12347)\b' || true"

echo "[deploy] done"
