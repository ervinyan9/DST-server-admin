# DST MOD Manager

> 注：本文档记录 `cmd/mod-manager` 当前实现，路径仍指向旧版宿主机部署（`/opt/dst-server/...`）。
> 仓库目标已迁移到 `dst-waystone` 单镜像方案（`/data/...`），但业务代码尚未跟进；
> 文中的 `/opt/dst-server/...` 应理解为遗留契约，待后续独立任务迁移。

This project includes a small local MOD manager for Don't Starve Together.

It is intentionally simple:

- Go HTTP server with plain HTML/CSS/JS.
- No database.
- No backend framework.
- State is stored in `mods/server-mods.json`.
- Basic server settings are cached separately in `config/server-settings.json`.
- Generated Lua files are stored in `mods/generated/`.

## Start

From `D:\Projects\DST`, after Go is installed:

```powershell
go run .\cmd\mod-manager -root . -port 8788
```

Open:

```text
http://127.0.0.1:8788/
```

## Remote Admin Deployment

The remote admin service is deployed on the DST server:

```text
/opt/dst-admin
```

It runs as a systemd service:

```bash
systemctl status dst-admin
systemctl restart dst-admin
```

Public URL:

```text
http://43.161.236.162/dst-admin/
```

The API is protected by an admin key. The local copy is stored in:

```text
secrets/admin-key.txt
```

On the server, the key is stored in:

```text
/etc/dst-admin.env
```

The web page stores the key in the current browser's local storage and sends it as:

```text
X-Admin-Key
```

## Save And Restart

The web UI has separate operations:

- 保存配置到服务器: writes generated config files into `/opt/dst-server/data/DoNotStarveTogether/Cluster_1`.
- 重启服务器: runs `docker compose restart` in `/opt/dst-server`.

Saving does not restart the game server. Restarting applies the saved settings and makes DST download or update configured Workshop MODs.

## Server Status

The web UI has a `刷新状态` button near the restart button.

It calls:

```text
GET /api/server/status
```

The endpoint reads Docker Compose status from `/opt/dst-server`, inspects container start time/health, and returns the last 80 lines of compose logs. After `重启服务器`, the page automatically refreshes this status every 5 seconds for up to about 2 minutes, stopping early when the server becomes `running` or reports an error.

Static assets are loaded with a startup version query string, for example `/static/app.js?v=...`, and both HTML/static responses use `Cache-Control: no-store, max-age=0`. This avoids the domain/CDN returning stale JavaScript after a deployment.

The UI layout is designed for real data volume:

- The whole page can scroll.
- Search results, selected MODs, players, and logs each have their own scroll containers.
- Main panels must not overlap when selected MODs or search results contain many rows/cards.

Saving to the server writes both the live DST config and the install options snapshot:

```text
/opt/dst-server/data/DoNotStarveTogether/Cluster_1/cluster.ini
/opt/dst-server/install-options.env
```

Keeping these two files in sync prevents future setup/restart workflows from reusing stale install options and reverting basic settings.

The admin page reads basic server settings from:

```text
config/server-settings.json
```

This file is runtime state. Do not overwrite it during deployment unless you explicitly want to reset the server name, password, game mode, player count, PVP, pause, and cave settings. If this file is missing, the admin migrates settings from `mods/server-mods.json`; if neither has settings, it uses defaults.

## SPA Admin Console

The current admin UI is a lightweight SPA:

- Go still serves one HTML page and JSON APIs.
- Alpine.js is vendored locally in `web/static/vendor/alpine-3.15.12.min.js`.
- CSS is shipped as one static file in `web/static/app.css`.
- There is no Node/Vite/npm runtime requirement on the server.

The page uses API key login and stores the key in browser local storage. After login, the main views are:

- 服务器: status, Docker service health, recent logs, basic settings, save and restart.
- MOD 管理: Workshop search, popular recommendations, installed MOD list, download and diagnostics.
- 玩家权限: detected players and admin list management.

Buttons should always show a visible state change:

- Loading actions are disabled while running.
- Success, warning, and error messages appear as toasts.
- Saving and restarting are separate actions.

## MOD Diagnostics And SteamCMD Download

The admin exposes:

```text
GET /api/mods/diagnostics
POST /api/mods/download
```

Diagnostics checks each selected MOD against:

- `dedicated_server_mods_setup.lua`
- `Master/modoverrides.lua`
- `Caves/modoverrides.lua`
- `/opt/dst-server/data/ugc/content/322330/<mod-id>`
- recent Docker/DST logs

The download endpoint runs SteamCMD inside the DST container and then copies the downloaded Workshop directory into the UGC content directory used by the DST server.

If SteamCMD only downloads a `*_legacy.bin` file, the UI marks it as a legacy package. That means the download command succeeded, but the current DST container may not be able to load it directly.

## What The Manager Does

The page can:

- Search the DST Steam Workshop.
- Extract Workshop PublishedFileId values.
- Fetch item details from Steam's public `GetPublishedFileDetails` endpoint.
- Add or remove MODs from the local list.
- Enable or disable selected MODs.
- Edit basic server settings, including survival/endless mode.
- Generate:
  - `mods/generated/dedicated_server_mods_setup.lua`
  - `mods/generated/modoverrides.lua`
  - `config/generated/server-settings.json`
  - `config/generated/cluster.ini`

## How DST Server MODs Work

`dedicated_server_mods_setup.lua` downloads MODs:

```lua
ServerModSetup("378160973")
```

`modoverrides.lua` enables MODs:

```lua
return {
  ["workshop-378160973"] = { enabled = true },
}
```

For this server, the generated files eventually need to be copied to:

```text
/opt/dst-server/data/DoNotStarveTogether/Cluster_1/mods/dedicated_server_mods_setup.lua
/opt/dst-server/data/DoNotStarveTogether/Cluster_1/Master/modoverrides.lua
/opt/dst-server/data/DoNotStarveTogether/Cluster_1/Caves/modoverrides.lua
```

Generated server settings:

```text
config/generated/server-settings.json
config/generated/cluster.ini
```

`cluster.ini` contains the game mode:

```ini
[GAMEPLAY]
game_mode = survival
```

Use `survival` for 生存模式 and `endless` for 无尽模式.

Then restart:

```bash
cd /opt/dst-server
docker compose restart
```

## Current Limitation

The first version only generates basic enable/disable entries.

For MODs with custom `configuration_options`, the safest workflow is:

1. Create a local temporary DST world in the game client.
2. Select and configure server MODs through the game's UI.
3. Copy the generated `modoverrides.lua`.
4. Use that file as the source of truth for Master and Caves.

## Manual Search Flow

DST Workshop AppID:

```text
322330
```

Search URL format:

```text
https://steamcommunity.com/workshop/browse/?appid=322330&browsesort=textsearch&section=readytouseitems&searchtext=Global%20Positions
```

Every MOD page URL contains the ID:

```text
https://steamcommunity.com/sharedfiles/filedetails/?id=378160973
```

The ID is:

```text
378160973
```
