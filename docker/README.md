# dst-waystone Build Context

This is the first-stage Docker build context for `dst-waystone`.

This repository keeps the Dockerfile, default runtime configuration, entrypoint, and supervisor configuration. Production is expected to perform the actual image build, secret injection, data mounting, and service orchestration.

It builds by default:

- A Linux `dst-admin` binary from `cmd/mod-manager`.
- Static admin assets from `web/`.
- A SteamCMD-based runtime that can download or update DST AppID `343050`.
- Repository-owned entrypoint and supervisor configuration.

It intentionally does not copy code from `jamesits/docker-dst-server` or `superjump22/dontstarve-server-docker`.

## Build

```bash
docker build -f docker/Dockerfile -t dst-waystone:local .
```

To attempt downloading DST during image build:

```bash
docker build \
  --build-arg DST_DOWNLOAD_AT_BUILD=true \
  -f docker/Dockerfile \
  -t dst-waystone:local .
```

Build-time DST download depends on Steam network and app configuration. The default build keeps that step at runtime.

## Run Admin Only

Without a Klei token, the image initializes `/data`, starts `dst-admin`, and keeps DST shards disabled.

```bash
docker run --rm -p 8788:8788 dst-waystone:local
```

## Run With DST Token

```bash
docker run --rm \
  -e DST_CLUSTER_TOKEN="<set-klei-token>" \
  -e DST_ADMIN_KEY="<set-admin-key>" \
  -e DST_SKIP_GAME_UPDATE=false \
  -p 8788:8788 \
  -p 10999:10999/udp \
  -p 11000:11000/udp \
  -p 12346:12346/udp \
  -p 12347:12347/udp \
  -v dst-admin-data:/data \
  dst-waystone:local
```

## Notes

- DST server files are downloaded by SteamCMD when `DST_SKIP_GAME_UPDATE=false`, or at build time when `DST_DOWNLOAD_AT_BUILD=true`.
- Klei token, server password, player IDs, saves, logs, Steam cache, and Workshop content are not stored in Git.
- `dst-master` / `dst-caves` are configured with `autostart=false` in supervisor; the admin owns their lifecycle through `supervisorctl`. Without a Klei token, the admin keeps both shards stopped.
- Cluster config (`cluster.ini`, `Master/server.ini`, `Caves/server.ini`, `modoverrides.lua`, `dedicated_server_mods_setup.lua`) is written directly into `/data/cluster/Cluster_1/` by the admin; restart via the admin UI to apply.
