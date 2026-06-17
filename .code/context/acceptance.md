# 当前阶段验收标准

## 目标（Goal）

将仓库收敛为 **单栈 dst-waystone + Compose**，遵循一般工程的精简原则：仓库只保留构建镜像、启动容器、最少示例和文档，运行态由部署环境负责。

1. **删除宿主机直装链路**：`deploy/`、`scripts/install-dst-server.sh` 整体移除。线上后续按 `dst-waystone` 镜像 + `docker compose` 部署。
2. **`docker/` 是唯一构建与启动入口**：`Dockerfile`、`Dockerfile.dockerignore`、`entrypoint.sh`、`supervisord.conf`、`compose.yml`、`.env.example`、`README.md`。
3. **示例仅保留仓库自有最小样板**：`examples/worldgenoverride/{Master,Caves}.lua`。
4. **`mods/` 三类边界清晰**：`server-mods.json`（种子）、`generated/`（运行产物，git ignore）；不再混入"线上快照"。
5. **运行态不入仓库**：`bin/`、`mods/generated/`、`config/generated/`、`secrets/`、`*.local`、`*.secret` 全部 ignore。
6. **管理端业务代码不动**：`cmd/mod-manager/main.go` 的 `/opt/dst-server/...` 与 `jamesits/dst-server:latest` 硬编码保留，文档中已注明为遗留，待后续独立任务迁移到 `dst-waystone` 容器。
7. **上下文工程入口同步到新结构**：`AGENTS.md`、`README.md`、`.code/README.md`、`.code/context/{project-map,acceptance}.md`、`.code/knowledge/dst-mod-development/`、`.code/skills/dst-mod-development.md` 全部对齐。

## 必须满足

- 根目录存在 `README.md`、`AGENTS.md`，描述 `dst-waystone` 单栈方案，不再提 `deploy/` 或宿主机直装脚本。
- `.code/context/project-map.md`、`.code/context/acceptance.md` 与本阶段一致。
- `mods/README.md` 仅描述种子 vs 运行产物两类。
- `docker/{Dockerfile, Dockerfile.dockerignore, entrypoint.sh, supervisord.conf, compose.yml, .env.example, README.md}` 全部存在。
- `examples/worldgenoverride/{Master,Caves}.lua` 存在。
- `.gitignore` 至少包含：`/bin/`、`*.exe`、`mods/generated/`、`config/generated/`、`/tmp/`、`secrets/`、`*.local`、`*.secret`。

## 不应发生

- 不再出现旧目录：`deploy/`、`scripts/install-dst-server.sh`、`docker/integrated/`。
- 不修改 `cmd/mod-manager/main.go` 业务代码（运行时路径与 compose 模板暂留为遗留，待独立任务）。
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

# 旧路径无残留（应无命中，业务代码与历史镜像分析文档除外）
rg -n --glob '!.code/context/acceptance.md' \
   --glob '!cmd/mod-manager/main.go' \
   --glob '!docs/image/docker-image-source.md' \
   --glob '!docs/image/docker-image-candidates.md' \
   "deploy/|install-dst-server\\.sh|/opt/dst-server/"

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

- `test` 全部通过（包括 `test !` 的"不存在"断言）。
- 旧路径检索除业务代码与历史镜像分析文档外无命中。
- `git ls-files bin/` 为空。
- 敏感信息检索无命中。
- `go build` 与 `bash -n` 均成功。
