# 项目地图

## 根目录职责

```text
README.md                 项目总入口
AGENTS.md                 AI 协作和项目工作约束
go.mod                    Go 模块声明
cmd/mod-manager/          dst-admin Go HTTP 管理端
web/                      管理端页面模板和静态资源
mods/                     管理端 MOD 状态种子和生成配置（运行产物已 git ignore）
scripts/                  本地/服务器安装脚本（唯一源）
deploy/                   线上部署资产快照和恢复说明
docker/                   dst-waystone 生产构建上下文（Dockerfile/entrypoint/supervisor）
docs/                     设计、来源、镜像候选、MOD 整合文档
.code/                    上下文工程和开发知识库
```

## 运行与部署资产

- `deploy/server/docker-compose.yml`：DST Compose 配置。
- `deploy/server/Cluster_1/`：与线上 `Cluster_1` 完全对齐的集群配置（`cluster.ini`、`Master/`、`Caves/`、`mods/`），敏感字段需占位。
- `deploy/admin-snapshot/`：管理端线上配置与 MOD 状态快照（`cluster.ini`、`install-options.env`、`server-settings.json`、`server-mods.json`）。
- `deploy/systemd/`：管理端 systemd 配置和 env 示例。
- `deploy/nginx/`：管理端反向代理片段。
- `deploy/runtime-state/`：线上巡检记录，只记录必要状态，不记录密钥。

## 管理端

- 入口：`cmd/mod-manager/main.go`
- 端口：默认 `8788`
- 认证：`DST_ADMIN_KEY`，为空时 API 不受保护，仅适合本地验证。
- 状态：`mods/server-mods.json`（仓库种子，运行时被覆盖）
- 生成物：`mods/generated/`、`config/generated/`（已 ignore）
- 前端：`web/templates/index.html`、`web/static/app.css`、`web/static/app.js`
- 文档：`docs/admin/mod-manager.md`、`docs/admin/mod-consolidation-plan.md`

## dst-waystone

- 构建入口：`docker/Dockerfile`
- 运行入口：`docker/entrypoint.sh`
- 进程配置：`docker/supervisord.conf`
- 说明：`docker/README.md`
- 设计：`docs/image/integrated-image-design.md`
- 来源边界：`docs/image/docker-image-source.md`、`docs/image/docker-image-candidates.md`

当前定位：生产构建上下文。仓库负责维护 Dockerfile 和配置模板；生产环境负责构建、发布、密钥注入、数据挂载和服务编排。

## 饥荒 MOD 开发资料

- 现有整合计划：`docs/admin/mod-consolidation-plan.md`
- 管理端说明：`docs/admin/mod-manager.md`
- 饥荒 MOD 开发知识库：`.code/knowledge/dst-mod-development/`
- 饥荒 MOD 开发技能卡：`.code/skills/dst-mod-development.md`
