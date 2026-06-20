# 项目地图

## 根目录职责

```text
README.md                 项目总入口
AGENTS.md                 AI 协作和项目工作约束
go.mod                    Go 模块声明
cmd/mod-manager/          dst-admin Go HTTP 管理端（默认 -root=/opt/dst/admin、-dst-dir=/data）
web/                      管理端页面模板和静态资源
mods/                     管理端 MOD 状态种子（容器内运行时写入 /data/admin/）
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
- 生产环境说明：`docs/production-environment.md`。
- 设计与来源边界：`docs/image/integrated-image-design.md`、`docs/image/docker-image-source.md`、`docs/image/docker-image-candidates.md`。

## 管理端

- 入口：`cmd/mod-manager/main.go`
- 端口：默认 `8788`
- 容器路径：`-root=/opt/dst/admin`、`-dst-dir=/data`、`-supervisor-conf=/opt/dst/runtime/supervisord.conf`
- 认证：`DST_ADMIN_KEY`，为空时 API 不受保护，仅适合本地验证。
- 状态：`/data/admin/server-mods.json`、`/data/admin/server-settings.json`（容器运行时；仓库内 `mods/server-mods.json` 仅作种子）
- 落地：`cluster.ini`、`Master/server.ini`、`Caves/server.ini`、`modoverrides.lua`、`dedicated_server_mods_setup.lua` 由管理端直接写入 `/data/cluster/Cluster_1/`
- 进程控制：通过 `supervisorctl -c /opt/dst/runtime/supervisord.conf` 管理 `dst-master`/`dst-caves`，supervisor 中两者 `autostart=false`
- 前端：`web/templates/index.html`、`web/static/app.css`、`web/static/app.js`（Alpine.js SPA）
- 文档：`docs/admin/mod-manager.md`、`docs/admin/mod-consolidation-plan.md`

## 饥荒 MOD 开发资料

- 现有整合计划：`docs/admin/mod-consolidation-plan.md`
- 管理端说明：`docs/admin/mod-manager.md`
- 饥荒 MOD 开发知识库：`.code/knowledge/dst-mod-development/`
- 饥荒 MOD 开发技能卡：`.code/skills/dst-mod-development.md`
