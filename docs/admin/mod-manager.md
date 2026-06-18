# DST MOD Manager

`cmd/mod-manager` 是 `dst-waystone` 镜像内运行的管理端。它是一个 Go HTTP 服务 + Alpine.js 单页前端，负责：

- 读写 DST cluster 配置（`cluster.ini`、`Master/server.ini`、`Caves/server.ini`、`modoverrides.lua`、`dedicated_server_mods_setup.lua`）。
- 通过 Steam Workshop API 搜索 / 下载 MOD（SteamCMD 子进程）。
- 通过 `supervisorctl` 启停 `dst-master` / `dst-caves` 进程。
- 写入 Klei cluster token。

设计原则：管理端是运行时配置的唯一真相源；entrypoint 只做最小化初始化；运行时尽量减少环境变量和外部配置文件。

## 容器内契约

| 项 | 路径/值 |
| --- | --- |
| 启动参数 | `-root=/opt/dst/admin -dst-dir=/data -listen=0.0.0.0 -port=8788 -supervisor-conf=/opt/dst/runtime/supervisord.conf` |
| 静态资源 | `/opt/dst/admin/web/` |
| 状态文件 | `/data/admin/server-mods.json`、`/data/admin/server-settings.json` |
| Cluster 目录 | `/data/cluster/Cluster_1/`（Master、Caves、mods 子目录） |
| Klei token | `/data/cluster/Cluster_1/cluster_token.txt`（mode 0600） |
| MOD UGC | `/data/ugc_mods/content/322330/<id>` |
| SteamCMD | `${STEAMCMDDIR:-/home/steam/steamcmd}/steamcmd.sh` |
| 进程控制 | `supervisorctl -c /opt/dst/runtime/supervisord.conf {restart,start,stop} {dst-master,dst-caves}` |

仓库内 `mods/server-mods.json` 仅作种子参考，运行时不会被读取。

## 一步式保存

管理端不再区分 “保存草稿” / “生成本地配置” / “保存到服务器” 三步。所有写入路径走同一个 `applyConfig()`：

- `POST /api/settings`：先写状态文件，再立刻把 `cluster.ini`、`Master/server.ini`、`Caves/server.ini` 落到 cluster 目录。
- `POST /api/mods`、`POST /api/mods/toggle`、`POST /api/mods/remove`：先更新状态，再立刻重新生成 `dedicated_server_mods_setup.lua` 与 `Master|Caves/modoverrides.lua`。

返回结构含 `state`（最新设置）和 `applied.written`（本次写入的文件清单），前端据此提示。

## Token 流程

```
POST /api/cluster-token  body: { "token": "pds-..." }
```

写入 `/data/cluster/Cluster_1/cluster_token.txt`，权限 0600，不回显。也可在容器启动时通过 `DST_CLUSTER_TOKEN` 环境变量首次写入（仅当文件不存在时）。

## 启停与状态

| 路由 | 行为 |
| --- | --- |
| `POST /api/restart` | `supervisorctl restart dst-master`；按 `enable_caves` 决定 `dst-caves` 走 `restart` 还是 `stop` |
| `GET /api/server/status` | 解析 `supervisorctl status`，附最近日志（supervisor 转发到 stdout） |

前端在重启后会每 5 秒轮询一次 `status`，最长约 2 分钟。

## MOD 下载

```
POST /api/mods/download  body: { "id": "<workshop-id>" }
```

流程：

1. 调用 SteamCMD：`+force_install_dir /data/ugc_mods +login anonymous +workshop_download_item 322330 <id> +quit`。
2. 把 `/data/ugc_mods/steamapps/workshop/content/322330/<id>` 拷贝到 `/data/ugc_mods/content/322330/<id>`。
3. 返回诊断结构（配置存在性、文件存在性、日志加载状态）。

`GET /api/mods/diagnostics` 给出全量 MOD 的诊断。

## 玩家与权限

| 路由 | 行为 |
| --- | --- |
| `GET /api/players` | 从 DST 日志中提取玩家 KU id 与最近活跃时间 |
| `POST /api/players/admin/add` | 加入 `Master/adminlist.txt`、`Caves/adminlist.txt` |
| `POST /api/players/admin/remove` | 反向操作 |

修改后需重启服务器才能生效。

## 本地开发

仓库根目录直接运行：

```bash
go run ./cmd/mod-manager -root . -dst-dir ./.tmp-data -port 8788
```

会以仓库的 `web/` 作为静态根，`./.tmp-data` 作为模拟的 `/data`。本地通常没有 supervisor，因此 `/api/restart` 与 `/api/server/status` 会报错；MOD 配置生成与 cluster 文件写入仍可验证。

## 限制

- 自定义 `configuration_options` 仍只生成 `enabled = true`。如需复杂配置，建议先在游戏客户端建立临时世界、配置好 MOD 后复制 `modoverrides.lua` 内容作为参考。
- SteamCMD 在某些 MOD 上仅返回 `*_legacy.bin`，前端会在诊断里标记为 legacy；表示下载成功但 DST 容器可能无法直接加载，需等待 MOD 作者更新。
