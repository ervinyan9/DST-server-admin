#!/usr/bin/env bash
set -Eeuo pipefail

# 设计原则：entrypoint 只负责启动期最小化初始化，所有运行时配置由管理端写入。
#
# 该脚本只做四件事：
#   1. 准备 /data 目录骨架（cluster/admin/mods/ugc_mods）
#   2. 如设置 DST_CLUSTER_TOKEN 且 token 文件不存在，则写入（mode 0600）
#   3. 如 DST_SKIP_GAME_UPDATE=false，运行 SteamCMD 更新 DST server
#   4. 如 DST_UPDATE_MODS_ON_START=true 且 token 已就绪，运行 mod 更新
# 然后 exec 到传入的命令（通常是 supervisord）。

DATA_DIR="${DST_DATA_DIR:-/data}"
CLUSTER_DIR="${DATA_DIR}/cluster/Cluster_1"
TOKEN_FILE="${CLUSTER_DIR}/cluster_token.txt"
GAME_DIR="${DST_GAME_DIR:-/opt/dst/game}"

log() {
  printf '[dst-waystone] %s\n' "$*"
}

init_layout() {
  mkdir -p \
    "${CLUSTER_DIR}/Master" \
    "${CLUSTER_DIR}/Caves" \
    "${CLUSTER_DIR}/mods" \
    "${DATA_DIR}/mods" \
    "${DATA_DIR}/ugc_mods" \
    "${DATA_DIR}/admin"
}

init_token() {
  if [[ -n "${DST_CLUSTER_TOKEN:-}" && ! -s "$TOKEN_FILE" ]]; then
    log "Writing Klei cluster token from DST_CLUSTER_TOKEN."
    (umask 077 && printf '%s\n' "${DST_CLUSTER_TOKEN}" > "$TOKEN_FILE")
  fi
}

update_game_if_requested() {
  if [[ "${DST_SKIP_GAME_UPDATE:-true}" == "true" ]]; then
    return
  fi
  log "Updating DST dedicated server via SteamCMD."
  "${STEAMCMDDIR:-/home/steam/steamcmd}/steamcmd.sh" \
    +force_install_dir "$GAME_DIR" \
    +login anonymous \
    +app_update 343050 \
    +quit
}

update_mods_if_requested() {
  if [[ "${DST_UPDATE_MODS_ON_START:-false}" != "true" ]]; then
    return
  fi
  if [[ ! -s "$TOKEN_FILE" ]]; then
    log "Skipping mod update: Klei cluster token missing."
    return
  fi
  if [[ ! -x "${GAME_DIR}/bin64/dontstarve_dedicated_server_nullrenderer_x64" ]]; then
    log "Skipping mod update: DST server binary not present."
    return
  fi
  log "Updating DST server mods."
  "${GAME_DIR}/bin64/dontstarve_dedicated_server_nullrenderer_x64" \
    -only_update_server_mods \
    -ugc_directory "${DATA_DIR}/ugc_mods" \
    -persistent_storage_root "$DATA_DIR" \
    -conf_dir cluster \
    -cluster Cluster_1
}

main() {
  init_layout
  init_token
  update_game_if_requested
  update_mods_if_requested
  exec "$@"
}

main "$@"
