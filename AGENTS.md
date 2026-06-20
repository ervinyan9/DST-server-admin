# DST-server-admin 项目协作说明

## 目标

本仓库维护《饥荒联机版》自建服务器的 `dst-waystone` 生产构建上下文、`dst-admin` 管理端，以及后续饥荒 MOD 开发资料。运行态（token、密码、存档、镜像、Workshop 下载）由部署环境负责，不进入 Git。

## 当前边界

- 镜像构建上下文在 `docker/`：`Dockerfile` 用于构建 `dst-waystone:local`，`compose.yml` + `.env.example` 用于启动容器，`entrypoint.sh` 与 `supervisord.conf` 是运行时编排。`dst-master` / `dst-caves` 在 supervisor 中 `autostart=false`，由管理端通过 `supervisorctl` 控制启停。
- 管理端代码在 `cmd/mod-manager/`、`web/`、`mods/`。已迁移到 `dst-waystone` 容器契约：`-root=/opt/dst/admin`、`-dst-dir=/data`，状态写入 `/data/admin/`，cluster 配置直接落到 `/data/cluster/Cluster_1/`。
- 配方示例在 `examples/`，只放与本仓库自有内容相关的最小样板。
- 饥荒 MOD 开发资料沉淀在 `.code/knowledge/dst-mod-development/`，进入实际代码开发前必须先补齐机制理解、设计、验证和来源边界。

## 工作纪律

- 所有回答和项目说明优先使用中文；面向开源分发的用户文档后续补英文版。
- 修改前先读现有文件，不凭记忆改镜像构建上下文或 MOD 配置。
- 只做用户要求的范围，不顺手重构、不移动目录、不清理无关文件。
- 不提交 Klei token、服务器密码、玩家 ID、Steam 缓存、Workshop 下载内容、存档或真实管理密钥。
- 不复制 `jamesits/docker-dst-server`、`superjump22/dontstarve-server-docker` 或 Workshop MOD 的受限源码；允许记录来源、行为观察和重写计划。

## 上下文入口

- `.code/README.md`：上下文工程总入口。
- `.code/context/project-map.md`：仓库目录和职责索引。
- `.code/context/acceptance.md`：当前阶段验收标准。
- `.code/knowledge/dst-mod-development/README.md`：饥荒 MOD 开发知识库入口。
- `.code/skills/dst-mod-development.md`：饥荒 MOD 开发技能卡。
- `.code/skills/production-deploy.md`：生产构建部署、备份、验收和回滚技能卡。

## 常用验证

```bash
go test ./...
go build -o /tmp/dst-mod-manager-check ./cmd/mod-manager
bash -n docker/entrypoint.sh
docker build -f docker/Dockerfile -t dst-waystone:local .
```

按任务风险选择验证粒度。文档和上下文改动至少做路径存在性检查、旧口径检索和敏感信息检索。

## Repo-local 技能路由

- 涉及生产部署、生产构建、发布、回滚、部署 SOP 或 `make prod-deploy` 时，先阅读 `.code/skills/production-deploy.md`。
- 涉及 DST MOD 开发、MOD 整合、资源减负、Lua 模块、游戏内 UI、RPC 或管理员命令时，先阅读 `.code/skills/dst-mod-development.md`。
