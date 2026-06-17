#!/usr/bin/env bash
set -Eeuo pipefail

DATA_DIR="${DST_DATA_DIR:-/data}"
CLUSTER_NAME="${DST_CLUSTER_NAME:-Cluster_1}"
CONF_DIR="${DST_CONF_DIR:-cluster}"
CLUSTER_DIR="${DATA_DIR}/${CONF_DIR}/${CLUSTER_NAME}"
MASTER_DIR="${CLUSTER_DIR}/Master"
CAVES_DIR="${CLUSTER_DIR}/Caves"
MODS_DIR="${CLUSTER_DIR}/mods"
TOKEN_FILE="${CLUSTER_DIR}/cluster_token.txt"
ADMIN_DIR="${DST_ADMIN_DIR:-/opt/dst/admin}"
GAME_DIR="${DST_GAME_DIR:-/opt/dst/game}"
GAME_BIN="${GAME_DIR}/bin64/dontstarve_dedicated_server_nullrenderer_x64"

log() {
  printf '[dst-waystone] %s\n' "$*"
}

write_file_if_missing() {
  local path="$1"
  local content="$2"
  if [[ ! -f "$path" ]]; then
    mkdir -p "$(dirname "$path")"
    printf '%s\n' "$content" > "$path"
  fi
}

bool_value() {
  case "${1:-}" in
    true|TRUE|1|yes|YES|on|ON) printf 'true' ;;
    *) printf 'false' ;;
  esac
}

init_layout() {
  mkdir -p "$MASTER_DIR" "$CAVES_DIR" "$MODS_DIR" \
    "${DATA_DIR}/mods" "${DATA_DIR}/ugc_mods" "${DATA_DIR}/admin" \
    "${ADMIN_DIR}/config" "${ADMIN_DIR}/mods/generated"
}

init_cluster_config() {
  local enable_caves
  enable_caves="$(bool_value "${DST_ENABLE_CAVES:-true}")"

  write_file_if_missing "${CLUSTER_DIR}/cluster.ini" "[GAMEPLAY]
game_mode = ${DST_GAME_MODE:-survival}
max_players = ${DST_MAX_PLAYERS:-6}
pvp = false
pause_when_empty = true

[NETWORK]
cluster_description = Dedicated Don't Starve Together server managed by dst-admin.
cluster_name = ${DST_SERVER_NAME:-DST Admin Server}
cluster_password = ${DST_SERVER_PASSWORD:-}
cluster_intention = cooperative
offline_cluster = false
lan_only_cluster = false

[MISC]
console_enabled = true
max_snapshots = 6

[SHARD]
shard_enabled = ${enable_caves}
bind_ip = 127.0.0.1
master_ip = 127.0.0.1
master_port = 10998
cluster_key = dst_cluster_key"

  write_file_if_missing "${MASTER_DIR}/server.ini" "[NETWORK]
server_port = 10999

[SHARD]
is_master = true

[STEAM]
authentication_port = 8766
master_server_port = 12346"

  write_file_if_missing "${CAVES_DIR}/server.ini" "[NETWORK]
server_port = 11000

[SHARD]
is_master = false
name = Caves
id = 2

[STEAM]
authentication_port = 8767
master_server_port = 12347"

  write_file_if_missing "${MASTER_DIR}/leveldataoverride.lua" "return {
  id = \"SURVIVAL_TOGETHER\",
  name = \"Default Plus\",
  desc = \"A standard Don't Starve Together world.\",
  location = \"forest\",
  version = 4,
  overrides = {},
}"

  write_file_if_missing "${CAVES_DIR}/leveldataoverride.lua" "return {
  id = \"DST_CAVE\",
  name = \"The Caves\",
  desc = \"The caves preset.\",
  location = \"cave\",
  version = 4,
  overrides = {},
}"
}

init_mod_config() {
  if [[ ! -f "${MODS_DIR}/dedicated_server_mods_setup.lua" ]]; then
    if [[ -f "${ADMIN_DIR}/mods/generated/dedicated_server_mods_setup.lua" ]]; then
      cp "${ADMIN_DIR}/mods/generated/dedicated_server_mods_setup.lua" "${MODS_DIR}/dedicated_server_mods_setup.lua"
    else
      printf '%s\n' '-- Add ServerModSetup("workshop_id") entries here.' > "${MODS_DIR}/dedicated_server_mods_setup.lua"
    fi
  fi

  for shard_dir in "$MASTER_DIR" "$CAVES_DIR"; do
    if [[ ! -f "${shard_dir}/modoverrides.lua" ]]; then
      if [[ -f "${ADMIN_DIR}/mods/generated/modoverrides.lua" ]]; then
        cp "${ADMIN_DIR}/mods/generated/modoverrides.lua" "${shard_dir}/modoverrides.lua"
      else
        printf '%s\n' 'return {' '}' > "${shard_dir}/modoverrides.lua"
      fi
    fi
  done
}

init_admin_state() {
  if [[ ! -f "${ADMIN_DIR}/config/server-settings.json" ]]; then
    cat > "${ADMIN_DIR}/config/server-settings.json" <<EOF
{
  "server_name": "${DST_SERVER_NAME:-DST Admin Server}",
  "server_password": "${DST_SERVER_PASSWORD:-}",
  "game_mode": "${DST_GAME_MODE:-survival}",
  "max_players": ${DST_MAX_PLAYERS:-6},
  "pvp": false,
  "pause_when_empty": true,
  "enable_caves": $(bool_value "${DST_ENABLE_CAVES:-true}")
}
EOF
  fi
}

init_token() {
  if [[ -n "${DST_CLUSTER_TOKEN:-}" ]]; then
    log "Writing Klei cluster token from DST_CLUSTER_TOKEN."
    umask 077
    printf '%s' "${DST_CLUSTER_TOKEN}" > "$TOKEN_FILE"
  fi

  if [[ -f "$TOKEN_FILE" ]]; then
    local token
    token="$(tr -d '\r\n' < "$TOKEN_FILE")"
    printf '%s' "$token" > "$TOKEN_FILE"
  fi
}

token_ready() {
  [[ -s "$TOKEN_FILE" ]]
}

update_game_if_requested() {
  if [[ "${DST_SKIP_GAME_UPDATE:-true}" == "true" ]]; then
    return
  fi
  log "Updating DST dedicated server via SteamCMD."
  "${STEAMCMDDIR}/steamcmd.sh" \
    +force_install_dir "$GAME_DIR" \
    +login anonymous \
    +app_update 343050 \
    +quit
}

update_mods_if_requested() {
  if [[ "${DST_UPDATE_MODS_ON_START:-false}" != "true" ]]; then
    return
  fi
  if ! token_ready; then
    log "Skipping mod update because Klei cluster token is missing."
    return
  fi
  log "Updating DST server mods."
  "${GAME_DIR}/bin64/dontstarve_dedicated_server_nullrenderer_x64" \
    -only_update_server_mods \
    -ugc_directory "${DATA_DIR}/ugc_mods" \
    -persistent_storage_root "$DATA_DIR" \
    -conf_dir "$CONF_DIR" \
    -cluster "$CLUSTER_NAME"
}

configure_supervisor() {
  if [[ ! -x "$GAME_BIN" ]]; then
    export DST_DISABLE_GAME_PROCESSES=true
    log "DST server binary is missing at ${GAME_BIN}. DST shards will stay disabled."
    log "Set DST_SKIP_GAME_UPDATE=false to download DST at container startup, or build with DST_DOWNLOAD_AT_BUILD=true."
  elif token_ready; then
    export DST_DISABLE_GAME_PROCESSES=false
  else
    export DST_DISABLE_GAME_PROCESSES=true
    log "Klei token missing. DST shards will stay disabled; dst-admin will still start."
    log "Set DST_CLUSTER_TOKEN or create ${TOKEN_FILE}, then restart the container."
  fi
}

main() {
  init_layout
  init_cluster_config
  init_mod_config
  init_admin_state
  init_token
  update_game_if_requested
  update_mods_if_requested
  configure_supervisor
  exec "$@"
}

main "$@"
