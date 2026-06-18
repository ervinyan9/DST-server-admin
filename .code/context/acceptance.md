# 当前阶段验收标准

## 目标（Goal）

仓库已完成两阶段收敛：

- **Phase A**：单栈 `dst-waystone + Compose`，仓库只保留构建镜像、启动容器、最少示例和文档。
- **Phase B**：管理端业务代码迁移到 `dst-waystone` 容器契约（`/data`、`/opt/dst/admin`、`supervisorctl`），并合并保存路径为一步式。

最终形态遵循“管理端优先、最大程度减少配置文件和环境变量”的原则：运行时配置由管理端写入 `/data`，仅保留 token 写入与可选 SteamCMD 更新由 entrypoint 处理。

## 必须满足

### 仓库结构

- 根目录存在 `README.md`、`AGENTS.md`，描述 `dst-waystone` 单栈方案。
- `.code/context/{project-map,acceptance}.md` 与本阶段一致。
- `mods/README.md` 仅描述种子 vs 运行产物两类。
- `docker/{Dockerfile, Dockerfile.dockerignore, entrypoint.sh, supervisord.conf, compose.yml, .env.example, README.md}` 全部存在。
- `examples/worldgenoverride/{Master,Caves}.lua` 存在。
- `.gitignore` 至少包含：`/bin/`、`*.exe`、`mods/generated/`、`config/generated/`、`/tmp/`、`secrets/`、`*.local`、`*.secret`。

### 管理端契约

- `cmd/mod-manager/main.go` 默认 `-root=/opt/dst/admin`、`-dst-dir=/data`、`-supervisor-conf=/opt/dst/runtime/supervisord.conf`；不再含 `/opt/dst-server/` 或 `jamesits/dst-server` 引用。
- 状态文件：`/data/admin/server-mods.json`、`/data/admin/server-settings.json`。
- 配置直写 `/data/cluster/Cluster_1/`：`cluster.ini`、`Master/server.ini`、`Caves/server.ini`、`Master/modoverrides.lua`、`Caves/modoverrides.lua`、`Cluster_1/mods/dedicated_server_mods_setup.lua`。
- HTTP 路由：保存配置 `/api/settings`、写入 token `/api/cluster-token`、重启 `/api/restart`（走 `supervisorctl`），不再有 `/api/save` 或 `/api/generate`。
- MOD CRUD（`/api/mods`、`/api/mods/toggle`、`/api/mods/remove`）在保存状态后立刻 `applyConfig()` 同步 cluster 目录。
- SteamCMD 调用使用 `${STEAMCMDDIR:-/home/steam/steamcmd}/steamcmd.sh`，UGC 目录 `/data/ugc_mods`。
- 不含历史敏感默认值（如 `717815`、`EX 娇柔的饥荒之旅`）。

### Docker 编排

- `docker/.env.example` 仅含 `DST_ADMIN_KEY`、`DST_CLUSTER_TOKEN`、`DST_SKIP_GAME_UPDATE`、`DST_UPDATE_MODS_ON_START` 四项。
- `docker/entrypoint.sh` 仅做：`init_layout`、`init_token`、可选 `update_game_if_requested`、可选 `update_mods_if_requested`，然后 `exec "$@"`。
- `docker/supervisord.conf` 含 `[unix_http_server]`、`[supervisorctl]`、`[rpcinterface:supervisor]`，`dst-admin` autostart=true，`dst-master`/`dst-caves` autostart=false。

## 不应发生

- 不再出现旧目录：`deploy/`、`scripts/install-dst-server.sh`、`docker/integrated/`。
- 业务代码不再出现 `/opt/dst-server/`、`jamesits/dst-server`、`docker compose` 直调。
- 不再出现 `install-options.env` 生成链。
- 不引入新的运行时依赖。
- 不写入真实 token、密码、玩家 Klei ID、存档或 Workshop 下载内容。
- 不复制第三方 MOD 或镜像项目的受限源码。
- `bin/` 不再被 git 跟踪。

## 验证方式

```bash
# 入口文件存在
test -f README.md
test -f AGENTS.md
test -f .code/README.md
test -f .code/context/project-map.md
test -f .code/context/acceptance.md
test -f mods/README.md

# docker 单栈
test -f docker/Dockerfile
test -f docker/Dockerfile.dockerignore
test -f docker/entrypoint.sh
test -f docker/supervisord.conf
test -f docker/compose.yml
test -f docker/.env.example
test -f docker/README.md

# 示例
test -f examples/worldgenoverride/Master.lua
test -f examples/worldgenoverride/Caves.lua

# 旧目录 / 旧脚本已清理
test ! -d deploy
test ! -d scripts
test ! -d docker/integrated

# 旧路径无残留（应无命中，历史镜像分析文档除外）
rg -n --glob '!.code/context/acceptance.md' \
   --glob '!docs/image/docker-image-source.md' \
   --glob '!docs/image/docker-image-candidates.md' \
   "/opt/dst-server/|install-options\\.env|jamesits/dst-server"

# 构建产物未跟踪
git ls-files bin/ | wc -l   # 应为 0

# 敏感信息检索（应无命中真实密钥/密码/玩家 ID）
rg -n --glob '!.code/context/acceptance.md' \
   "pds-[A-Za-z0-9]|DST_ADMIN_KEY=[A-Za-z0-9+/=_-]{12,}|KU_[A-Za-z0-9]{6,}|cluster_password.*717815|server_password.*717815"

# 构建/语法检查
go build -o /tmp/dst-mod-manager-check ./cmd/mod-manager
bash -n docker/entrypoint.sh
```

预期：

- `test` 全部通过（包括 `test !` 的“不存在”断言）。
- 旧路径检索除历史镜像分析文档外无命中。
- `git ls-files bin/` 为空。
- 敏感信息检索无命中。
- `go build` 与 `bash -n` 均成功。
