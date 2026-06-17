# 当前阶段验收标准

## 目标（Goal）

把仓库目录结构整理为可持续维护的开发工作台，目标对齐线上运行布局并消除命名歧义：

1. **运行产物不入仓库**：`bin/`、`mods/generated/`、`config/generated/` 一律 git ignore，仓库内只保留可复现的源、模板、配置和快照。
2. **`deploy/` 与线上 `Cluster_1` 完全对齐**：`deploy/server/Cluster_1/{cluster.ini, Master, Caves, mods}` 一一对应线上 `/opt/dst-server/data/DoNotStarveTogether/Cluster_1`，恢复时可整目录拷贝，不再用扁平 `master-*` / `caves-*` 前缀。
3. **管理端线上快照单一入口**：原 `deploy/admin/` 与 `deploy/state/` 合并为 `deploy/admin-snapshot/`，去掉误导性的 `generated-` 前缀；与 `mods/server-mods.json`（仓库种子）边界清晰。
4. **安装脚本只剩唯一源**：`scripts/install-dst-server.sh` 是唯一源，`deploy/scripts/install-dst-server.remote.sh` 已删除。
5. **docker 构建上下文扁平化**：`docker/integrated/` 提到 `docker/` 一级，与新部署的开发者认知一致。
6. **文档分类**：`docs/admin/`（管理端、MOD 整合）与 `docs/image/`（镜像设计、来源、候选）两个一级分类，避免根 `docs/` 目录平铺混淆。
7. **上下文工程入口仍存在**：`AGENTS.md`、`.code/README.md`、`.code/context/{project-map,acceptance}.md`、`.code/knowledge/dst-mod-development/`、`.code/skills/dst-mod-development.md` 全部保留并同步到新路径。

## 必须满足

- 根目录存在 `AGENTS.md`，并已更新到新路径（`docker/`、`docs/admin/`、`docs/image/`）。
- `.code/README.md`、`.code/context/project-map.md`、`.code/context/acceptance.md` 存在并指向当前结构。
- `.code/knowledge/dst-mod-development/{README.md,source-boundaries.md}` 与 `.code/skills/dst-mod-development.md` 存在。
- `mods/README.md` 存在，并明确"种子 vs 运行产物 vs 线上快照"三者边界。
- `deploy/server/Cluster_1/` 子目录布局存在，`deploy/admin-snapshot/` 存在。
- `docker/Dockerfile`、`docker/entrypoint.sh`、`docker/supervisord.conf`、`docker/README.md`、`docker/Dockerfile.dockerignore` 存在。
- `docs/admin/{mod-manager,mod-consolidation-plan}.md` 与 `docs/image/{integrated-image-design,docker-image-source,docker-image-candidates}.md` 存在。
- `.gitignore` 至少包含：`/bin/`、`*.exe`、`mods/generated/`、`config/generated/`、`/tmp/`、`secrets/`、`*.local`、`*.secret`。

## 不应发生

- 不再出现旧路径残留：`docker/integrated/`、`deploy/admin/`、`deploy/state/`、`deploy/scripts/`、`master-*.ini`、`caves-*.ini`、`generated-cluster.ini`、`generated-install-options.env`、`generated-server-settings.json`、`docs/mod-manager.md`、`docs/integrated-image-design.md`、`docs/docker-image-source.md`、`docs/docker-image-candidates.md`、`docs/mod-consolidation-plan.md`。
- 不修改管理端业务代码，不动 `cmd/mod-manager/main.go` 的运行时路径（仍使用 `mods/server-mods.json`、`mods/generated/`）。
- 不引入新的运行时依赖。
- 不写入真实 token、密码、玩家 Klei ID、存档或 Workshop 下载内容。
- 不复制第三方 MOD 或镜像项目的受限源码。
- `bin/` 不再被 git 跟踪。

## 验证方式

```bash
# 入口文件存在
test -f AGENTS.md
test -f .code/README.md
test -f .code/context/project-map.md
test -f .code/context/acceptance.md
test -f .code/knowledge/dst-mod-development/README.md
test -f .code/knowledge/dst-mod-development/source-boundaries.md
test -f .code/skills/dst-mod-development.md
test -f mods/README.md

# 新目录布局
test -d deploy/server/Cluster_1/Master
test -d deploy/server/Cluster_1/Caves
test -d deploy/server/Cluster_1/mods
test -f deploy/server/Cluster_1/cluster.ini
test -d deploy/admin-snapshot
test -f deploy/admin-snapshot/server-mods.json
test -f deploy/admin-snapshot/install-options.env
test -f deploy/admin-snapshot/server-settings.json

# docker 扁平化
test -f docker/Dockerfile
test -f docker/entrypoint.sh
test -f docker/supervisord.conf
test -f docker/README.md
test -f docker/Dockerfile.dockerignore
test ! -d docker/integrated

# docs 分类
test -f docs/admin/mod-manager.md
test -f docs/admin/mod-consolidation-plan.md
test -f docs/image/integrated-image-design.md
test -f docs/image/docker-image-source.md
test -f docs/image/docker-image-candidates.md

# 旧目录已清理
test ! -d deploy/admin
test ! -d deploy/state
test ! -d deploy/scripts
test ! -d deploy/docs
test ! -d .code/knowledge/jigsaw-model

# 旧路径无残留（应无命中）
rg -n --glob '!.code/context/acceptance.md' \
   "docker/integrated/|deploy/admin/|deploy/state/|deploy/scripts/|master-server\\.ini|caves-server\\.ini|generated-cluster\\.ini|generated-install-options\\.env|generated-server-settings\\.json"

# 旧 docs 平铺路径无残留（应无命中）
rg -n --glob '!.code/context/acceptance.md' \
   "docs/mod-manager\\.md|docs/mod-consolidation-plan\\.md|docs/integrated-image-design\\.md|docs/docker-image-source\\.md|docs/docker-image-candidates\\.md"

# 构建产物未跟踪
git ls-files bin/ | wc -l   # 应为 0

# 敏感信息检索（应无命中真实密钥/密码/玩家 ID）
rg -n --glob '!.code/context/acceptance.md' \
   "pds-[A-Za-z0-9]|DST_ADMIN_KEY=[A-Za-z0-9+/=_-]{12,}|KU_[A-Za-z0-9]{6,}|cluster_password.*717815|server_password.*717815"

# 构建/语法检查
go build -o /tmp/dst-mod-manager-check ./cmd/mod-manager
bash -n scripts/install-dst-server.sh
bash -n docker/entrypoint.sh
```

预期：

- `test` 全部通过（包括 `test !` 的"不存在"断言）。
- 两个旧路径检索都无命中。
- `git ls-files bin/` 为空。
- 敏感信息检索无命中。
- `go build` 与 `bash -n` 均成功。
