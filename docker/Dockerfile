# syntax=docker/dockerfile:1

ARG DST_IMAGE_PLATFORM=linux/amd64

FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS admin-builder

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY web ./web
COPY mods ./mods
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mod-manager ./cmd/mod-manager

FROM --platform=$DST_IMAGE_PLATFORM cm2network/steamcmd:root

ARG DEBIAN_FRONTEND=noninteractive
ARG STEAM_APPID=343050
ARG DST_DOWNLOAD_AT_BUILD=false
ARG USE_BUILD_PROXY=false
ARG USE_APT_PROXY=false
ARG BUILD_HTTP_PROXY=""
ARG BUILD_HTTPS_PROXY=""
ARG BUILD_NO_PROXY=""
ARG APT_MIRROR=""
ARG APT_SECURITY_MIRROR=""

ENV HTTP_PROXY="" \
    HTTPS_PROXY="" \
    http_proxy="" \
    https_proxy="" \
    NO_PROXY="" \
    no_proxy=""

ENV DST_ROOT=/opt/dst \
    DST_GAME_DIR=/opt/dst/game \
    DST_ADMIN_DIR=/opt/dst/admin \
    DST_RUNTIME_DIR=/opt/dst/runtime \
    DST_DATA_DIR=/data \
    DST_CLUSTER_NAME=Cluster_1 \
    DST_SERVER_NAME="DST Admin Server" \
    DST_GAME_MODE=survival \
    DST_MAX_PLAYERS=6 \
    DST_ENABLE_CAVES=true \
    DST_SKIP_GAME_UPDATE=true \
    DST_UPDATE_MODS_ON_START=false \
    STEAMCMD_PLATFORM_TYPE=linux \
    STEAMCMD_PLATFORM_BITNESS=64

RUN set -eux; \
    if [ "${USE_APT_PROXY}" = "true" ]; then \
      export HTTP_PROXY="${BUILD_HTTP_PROXY}" HTTPS_PROXY="${BUILD_HTTPS_PROXY}" \
        http_proxy="${BUILD_HTTP_PROXY}" https_proxy="${BUILD_HTTPS_PROXY}" \
        NO_PROXY="${BUILD_NO_PROXY}" no_proxy="${BUILD_NO_PROXY}"; \
    else \
      unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy NO_PROXY no_proxy; \
    fi; \
    if [ -n "${APT_MIRROR}" ]; then \
      find /etc/apt -type f \( -name '*.list' -o -name '*.sources' \) -print0 \
        | xargs -0 sed -i \
          -e "s|http://deb.debian.org/debian|${APT_MIRROR}|g" \
          -e "s|https://deb.debian.org/debian|${APT_MIRROR}|g"; \
    fi; \
    if [ -n "${APT_SECURITY_MIRROR}" ]; then \
      find /etc/apt -type f \( -name '*.list' -o -name '*.sources' \) -print0 \
        | xargs -0 sed -i \
          -e "s|http://deb.debian.org/debian-security|${APT_SECURITY_MIRROR}|g" \
          -e "s|https://deb.debian.org/debian-security|${APT_SECURITY_MIRROR}|g" \
          -e "s|http://security.debian.org/debian-security|${APT_SECURITY_MIRROR}|g" \
          -e "s|https://security.debian.org/debian-security|${APT_SECURITY_MIRROR}|g"; \
    fi; \
    dpkg --add-architecture i386; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      libcurl4-gnutls-dev:i386 \
      libcurl3-gnutls \
      libstdc++6:i386 \
      libgcc-s1:i386 \
      supervisor; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*; \
    mkdir -p "${DST_GAME_DIR}" "${DST_ADMIN_DIR}" "${DST_RUNTIME_DIR}" "${DST_DATA_DIR}"

RUN set -eux; \
    chown -R steam:steam "${DST_GAME_DIR}"

USER steam

RUN set -eux; \
    if [ "${USE_BUILD_PROXY}" = "true" ]; then \
      export HTTP_PROXY="${BUILD_HTTP_PROXY}" HTTPS_PROXY="${BUILD_HTTPS_PROXY}" \
        http_proxy="${BUILD_HTTP_PROXY}" https_proxy="${BUILD_HTTPS_PROXY}" \
        NO_PROXY="${BUILD_NO_PROXY}" no_proxy="${BUILD_NO_PROXY}"; \
    else \
      unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy NO_PROXY no_proxy; \
    fi; \
    if [ "${DST_DOWNLOAD_AT_BUILD}" = "true" ]; then \
      "${STEAMCMDDIR}/steamcmd.sh" +quit; \
    fi

RUN set -eux; \
    if [ "${USE_BUILD_PROXY}" = "true" ]; then \
      export HTTP_PROXY="${BUILD_HTTP_PROXY}" HTTPS_PROXY="${BUILD_HTTPS_PROXY}" \
        http_proxy="${BUILD_HTTP_PROXY}" https_proxy="${BUILD_HTTPS_PROXY}" \
        NO_PROXY="${BUILD_NO_PROXY}" no_proxy="${BUILD_NO_PROXY}"; \
    else \
      unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy NO_PROXY no_proxy; \
    fi; \
    if [ "${DST_DOWNLOAD_AT_BUILD}" = "true" ]; then \
      "${STEAMCMDDIR}/steamcmd.sh" \
        +@sSteamCmdForcePlatformType "${STEAMCMD_PLATFORM_TYPE}" \
        +@sSteamCmdForcePlatformBitness "${STEAMCMD_PLATFORM_BITNESS}" \
        +force_install_dir "${DST_GAME_DIR}" \
        +login anonymous \
        +app_update "${STEAM_APPID}" validate \
        +quit; \
    else \
      printf '%s\n' "Skipping DST download at build time. Set --build-arg DST_DOWNLOAD_AT_BUILD=true to enable it."; \
    fi

USER root

COPY --from=admin-builder /out/mod-manager /opt/dst/admin/mod-manager
COPY web /opt/dst/admin/web
COPY mods /opt/dst/admin/mods
COPY docs /opt/dst/admin/docs
COPY go.mod /opt/dst/admin/go.mod
COPY docker/entrypoint.sh /opt/dst/runtime/entrypoint.sh
COPY docker/supervisord.conf /opt/dst/runtime/supervisord.conf

RUN set -eux; \
    mkdir -p /opt/dst/admin/config /var/log/supervisor; \
    chmod 755 /opt/dst/admin/mod-manager /opt/dst/runtime/entrypoint.sh

VOLUME ["/data"]
EXPOSE 8788/tcp 10999/udp 11000/udp 12346/udp 12347/udp

ENTRYPOINT ["/opt/dst/runtime/entrypoint.sh"]
CMD ["supervisord", "-c", "/opt/dst/runtime/supervisord.conf", "-n"]
