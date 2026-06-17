#!/usr/bin/env bash
set -Eeuo pipefail

INSTALL_DIR="/opt/dst-server"
KLEI_ROOT="DoNotStarveTogether"
CLUSTER_NAME="Cluster_1"
SERVER_NAME="DST HK Server"
SERVER_DESCRIPTION="Dedicated Don't Starve Together server deployed by Docker."
SERVER_PASSWORD=""
MAX_PLAYERS="6"
GAME_MODE="survival"
PAUSE_WHEN_EMPTY="true"
PVP="false"
ENABLE_CAVES="true"
IMAGE="jamesits/dst-server:latest"
TOKEN=""
TOKEN_FILE=""
FORCE_TOKEN="false"
SKIP_DOCKER_INSTALL="false"
SKIP_START="false"

usage() {
  cat <<'USAGE'
Install or update a Docker-based Don't Starve Together dedicated server.

Required on first install:
  --token TOKEN                 Klei cluster token
  --token-file PATH             Read Klei cluster token from file

Common options:
  --install-dir PATH            Default: /opt/dst-server
  --server-name NAME            Default: DST HK Server
  --server-password PASSWORD    Empty means no password
  --description TEXT            Server description
  --max-players N               Default: 6
  --no-caves                    Disable caves shard
  --image IMAGE                 Default: jamesits/dst-server:latest
  --force-token                 Overwrite existing cluster_token.txt
  --skip-docker-install         Do not install Docker automatically
  --skip-start                  Generate files only, do not start containers
  -h, --help                    Show this help

Examples:
  sudo bash install-dst-server.sh --token 'pds-g^...=' --server-name 'Our DST'
  sudo bash install-dst-server.sh --token-file /root/dst-token.txt --server-password 'secret'
USAGE
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

log() {
  printf '[dst-install] %s\n' "$*"
}

require_root() {
  if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    die "Please run as root, for example: sudo bash $0 ..."
  fi
}

shell_quote() {
  local value="${1:-}"
  printf "'%s'" "${value//\'/\'\\\'\'}"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --install-dir) INSTALL_DIR="${2:?}"; shift 2 ;;
      --cluster-name) CLUSTER_NAME="${2:?}"; shift 2 ;;
      --server-name) SERVER_NAME="${2:?}"; shift 2 ;;
      --description) SERVER_DESCRIPTION="${2:?}"; shift 2 ;;
      --server-password) SERVER_PASSWORD="${2:-}"; shift 2 ;;
      --max-players) MAX_PLAYERS="${2:?}"; shift 2 ;;
      --game-mode) GAME_MODE="${2:?}"; shift 2 ;;
      --pause-when-empty) PAUSE_WHEN_EMPTY="${2:?}"; shift 2 ;;
      --pvp) PVP="${2:?}"; shift 2 ;;
      --token) TOKEN="${2:?}"; shift 2 ;;
      --token-file) TOKEN_FILE="${2:?}"; shift 2 ;;
      --image) IMAGE="${2:?}"; shift 2 ;;
      --force-token) FORCE_TOKEN="true"; shift ;;
      --no-caves) ENABLE_CAVES="false"; shift ;;
      --skip-docker-install) SKIP_DOCKER_INSTALL="true"; shift ;;
      --skip-start) SKIP_START="true"; shift ;;
      -h|--help) usage; exit 0 ;;
      *) die "Unknown option: $1" ;;
    esac
  done
}

load_token() {
  if [[ -n "$TOKEN_FILE" ]]; then
    [[ -f "$TOKEN_FILE" ]] || die "Token file not found: $TOKEN_FILE"
    TOKEN="$(tr -d '\r\n' < "$TOKEN_FILE")"
  fi
}

ensure_docker() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    log "Docker and Compose plugin are already installed."
    return
  fi

  [[ "$SKIP_DOCKER_INSTALL" == "false" ]] || die "Docker is missing and --skip-docker-install was set."
  [[ -r /etc/os-release ]] || die "Cannot detect Linux distribution."
  # shellcheck disable=SC1091
  . /etc/os-release

  if command -v apt-get >/dev/null 2>&1; then
    local repo_id="$ID"
    if [[ "$ID" != "ubuntu" && "$ID" != "debian" ]]; then
      [[ "${ID_LIKE:-}" == *"ubuntu"* ]] && repo_id="ubuntu"
      [[ "${ID_LIKE:-}" == *"debian"* ]] && repo_id="debian"
    fi
    [[ "$repo_id" == "ubuntu" || "$repo_id" == "debian" ]] \
      || die "Cannot map this apt-based distribution to a Docker repository."

    log "Installing Docker Engine and Compose plugin via apt."
    apt-get update
    apt-get install -y ca-certificates curl gnupg
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL "https://download.docker.com/linux/${repo_id}/gpg" | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    local codename="${VERSION_CODENAME:-}"
    [[ -n "$codename" ]] || die "VERSION_CODENAME is missing; install Docker manually or use --skip-docker-install."
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/${repo_id} ${codename} stable" \
      > /etc/apt/sources.list.d/docker.list
    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  elif command -v dnf >/dev/null 2>&1 || command -v yum >/dev/null 2>&1; then
    local pkg
    pkg="$(command -v dnf || command -v yum)"
    log "Installing Docker Engine and Compose plugin via ${pkg##*/}."
    "$pkg" install -y yum-utils
    "$pkg" config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
    "$pkg" install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  else
    die "No supported package manager found. Install Docker manually, then rerun with --skip-docker-install."
  fi

  systemctl enable --now docker
}

write_compose() {
  cat > "$INSTALL_DIR/docker-compose.yml" <<EOF
services:
  dst-server:
    image: ${IMAGE}
    container_name: dst-server
    restart: unless-stopped
    entrypoint:
      - /bin/bash
      - -lc
      - |
        chmod 711 /root
        exec /usr/local/bin/entrypoint.sh "\$\$@"
      - dst-entrypoint
    command:
      - supervisord
      - -c
      - /etc/supervisor/supervisor.conf
      - -n
    environment:
      DST_SERVER_ARCH: amd64
    ports:
      - "10999-11000:10999-11000/udp"
      - "12346-12347:12346-12347/udp"
    volumes:
      - ./data:/data
      - ./server:/opt/dst_server
      - ./steam:/root/Steam
    stop_grace_period: 6m
EOF
}

write_cluster_ini() {
  cat > "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/cluster.ini" <<EOF
[GAMEPLAY]
game_mode = ${GAME_MODE}
max_players = ${MAX_PLAYERS}
pvp = ${PVP}
pause_when_empty = ${PAUSE_WHEN_EMPTY}

[NETWORK]
cluster_description = ${SERVER_DESCRIPTION}
cluster_name = ${SERVER_NAME}
cluster_password = ${SERVER_PASSWORD}
cluster_intention = cooperative
offline_cluster = false
lan_only_cluster = false

[MISC]
console_enabled = true
max_snapshots = 6

[SHARD]
shard_enabled = ${ENABLE_CAVES}
bind_ip = 127.0.0.1
master_ip = 127.0.0.1
master_port = 10998
cluster_key = dst_cluster_key
EOF
}

write_master_ini() {
  mkdir -p "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/Master"
  cat > "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/Master/server.ini" <<'EOF'
[NETWORK]
server_port = 10999

[SHARD]
is_master = true

[STEAM]
authentication_port = 8766
master_server_port = 27016
EOF
}

write_caves_ini() {
  if [[ "$ENABLE_CAVES" != "true" ]]; then
    return
  fi

  mkdir -p "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/Caves"
  cat > "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/Caves/server.ini" <<'EOF'
[NETWORK]
server_port = 11000

[SHARD]
is_master = false
name = Caves
id = 2

[STEAM]
authentication_port = 8767
master_server_port = 27017
EOF

  cat > "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/Caves/leveldataoverride.lua" <<'EOF'
return {
  id = "DST_CAVE",
  name = "The Caves",
  desc = "The caves preset.",
  location = "cave",
  version = 4,
  overrides = {},
}
EOF
}

write_world_files() {
  cat > "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/Master/leveldataoverride.lua" <<'EOF'
return {
  id = "SURVIVAL_TOGETHER",
  name = "Default Plus",
  desc = "A standard Don't Starve Together world.",
  location = "forest",
  version = 4,
  overrides = {},
}
EOF

  cat > "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/Master/modoverrides.lua" <<'EOF'
return {
}
EOF

  if [[ "$ENABLE_CAVES" == "true" ]]; then
    cat > "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/Caves/modoverrides.lua" <<'EOF'
return {
}
EOF
  fi

  mkdir -p "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/mods"
  cat > "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/mods/dedicated_server_mods_setup.lua" <<'EOF'
-- Add server mods here, for example:
-- ServerModSetup("375859599")
EOF
}

write_token() {
  local token_path="$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME/cluster_token.txt"
  if [[ -f "$token_path" && "$FORCE_TOKEN" != "true" ]]; then
    log "Keeping existing token: $token_path"
    return
  fi

  [[ -n "$TOKEN" ]] || die "Token is required on first install. Use --token or --token-file."
  umask 077
  printf '%s\n' "$TOKEN" > "$token_path"
}

write_env_snapshot() {
  cat > "$INSTALL_DIR/install-options.env" <<EOF
INSTALL_DIR=$(shell_quote "$INSTALL_DIR")
CLUSTER_NAME=$(shell_quote "$CLUSTER_NAME")
KLEI_ROOT=$(shell_quote "$KLEI_ROOT")
SERVER_NAME=$(shell_quote "$SERVER_NAME")
SERVER_DESCRIPTION=$(shell_quote "$SERVER_DESCRIPTION")
MAX_PLAYERS=$(shell_quote "$MAX_PLAYERS")
GAME_MODE=$(shell_quote "$GAME_MODE")
PAUSE_WHEN_EMPTY=$(shell_quote "$PAUSE_WHEN_EMPTY")
PVP=$(shell_quote "$PVP")
ENABLE_CAVES=$(shell_quote "$ENABLE_CAVES")
IMAGE=$(shell_quote "$IMAGE")
EOF
}

configure_firewall() {
  if command -v ufw >/dev/null 2>&1; then
    log "Opening DST UDP ports in ufw if ufw is active."
    ufw allow 10999:11000/udp >/dev/null || true
    ufw allow 12346:12347/udp >/dev/null || true
  fi
}

start_server() {
  if [[ "$SKIP_START" != "false" ]]; then
    return 0
  fi
  log "Pulling image and starting DST server."
  (cd "$INSTALL_DIR" && docker compose pull && docker compose up -d)
}

print_summary() {
  cat <<EOF

DST server files are ready.
Install dir: $INSTALL_DIR
Compose file: $INSTALL_DIR/docker-compose.yml
Data dir: $INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME

Useful commands:
  cd $INSTALL_DIR && docker compose ps
  cd $INSTALL_DIR && docker compose logs -f --tail=120
  cd $INSTALL_DIR && docker compose restart
  cd $INSTALL_DIR && docker compose down

Open these UDP ports in your cloud security group:
  10999, 11000, 12346, 12347
EOF
}

main() {
  parse_args "$@"
  require_root
  load_token
  ensure_docker
  mkdir -p "$INSTALL_DIR/data/$KLEI_ROOT/$CLUSTER_NAME"
  write_compose
  write_cluster_ini
  write_master_ini
  write_caves_ini
  write_world_files
  write_token
  write_env_snapshot
  configure_firewall
  start_server
  print_summary
}

main "$@"
