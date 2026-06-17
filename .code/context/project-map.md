# 项目地图

## 根目录职责

```text
README.md                 项目总入口
AGENTS.md                 AI 协作和项目工作约束
go.mod                    Go 模块声明
cmd/mod-manager/          dst-admin Go HTTP 管理端（含旧版宿主机部署硬编码路径，待迁移）
web/                      管理端页面模板和静态资源
mods/                     管理端 MOD 状态种子和生成配置（运行产物已 git ignore）
docker/                   dst-waystone 镜像构建上下文 + Compose 启动
examples/                 仓库自有示例（worldgenoverride 等）
docs/                     设计、来源、镜像候选、MOD 整合文档
.code/                    上下文工程和开发知识库
```

## 镜像构建与运行

- `docker/Dockerfile`：多阶段构建，builder 阶段产出 `mod-manager`，runtime 阶段在 `cm2network/steamcmd:root` 上装运行依赖。
- `docker/compose.yml`：单服务启动，挂载 named volume `dst-data:/data`，从 `.env` 读环境变量。
- `docker/.env.example`：`DST_CLUSTER_TOKEN`、`DST_ADMIN_KEY` 等运行时变量模板，复制为 `.env` 后填入真实值。
- `docker/entrypoint.sh`：容器启动时初始化 `/data` 目录、写入 cluster token、按需运行 SteamCMD/MOD 更新。
- `docker/supervisord.conf`：管理 `dst-admin`、`dst-master`、`dst-caves` 三个进程。
- `docker/README.md`：构建和运行说明。
- 设计与来源边界：`docs/image/integrated-image-design.md`、`docs/image/docker-image-source.md`、`docs/image/docker-image-candidates.md`。

## 管理端

- 入口：`cmd/mod-manager/main.go`
- 端口：默认 `8788`
- 认证：`DST_ADMIN_KEY`，为空时 API 不受保护，仅适合本地验证。
- 状态：`mods/server-mods.json`（仓库种子，运行时被覆盖）
- 生成物：`mods/generated/`、`config/generated/`（已 ignore）
- 前端：`web/templates/index.html`、`web/static/app.css`、`web/static/app.js`
- 文档：`docs/admin/mod-manager.md`、`docs/admin/mod-consolidation-plan.md`

注：`main.go` 当前仍硬编码 `/opt/dst-server/...` 旧宿主机路径和 `jamesits/dst-server:latest` compose 模板。迁移到 `dst-waystone` 容器内 `/data` 与 `/opt/dst/admin` 是后续独立任务。

## 饥荒 MOD 开发资料

- 现有整合计划：`docs/admin/mod-consolidation-plan.md`
- 管理端说明：`docs/admin/mod-manager.md`
- 饥荒 MOD 开发知识库：`.code/knowledge/dst-mod-development/`
- 饥荒 MOD 开发技能卡：`.code/skills/dst-mod-development.md`
