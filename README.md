# Don't Starve Together Self-Hosted Server

本仓库维护一个自建《饥荒联机版》服务器栈：

- **`dst-waystone` 镜像构建上下文**：DST dedicated server + `dst-admin` 管理端的多阶段 Dockerfile，运行时由 supervisor 同时管理 Master、Caves 和管理端进程。
- **`dst-admin` 管理端**：Go 写的 HTTP 管理端，源码在 `cmd/mod-manager/`、`web/`、`mods/`。
- **饥荒 MOD 开发资料**：在 `.code/knowledge/dst-mod-development/` 沉淀机制理解和来源边界。

仓库只保存源、模板和文档；运行态（Klei token、密码、镜像、Workshop 下载、存档）不进 Git。

## 仓库结构

```text
docker/
  Dockerfile                镜像构建定义（cm2network/steamcmd:root + golang:1.22-bookworm）
  Dockerfile.dockerignore   构建上下文裁剪
  compose.yml               单服务 Compose 启动脚本
  .env.example              运行时环境变量模板
  entrypoint.sh             容器启动初始化逻辑
  supervisord.conf          dst-admin / dst-master / dst-caves 三进程编排
  README.md                 镜像构建与运行说明

cmd/mod-manager/            管理端入口（默认 `-root=/opt/dst/admin`、`-dst-dir=/data`，支持 supervisorctl）
web/                        管理端模板与静态资源
mods/                       管理端 MOD 状态种子（容器运行时写入 /data/admin/）

examples/
  worldgenoverride/         世界生成覆盖样板（仓库自有内容，可作为容器内默认配置参考）

docs/
  admin/                    管理端与 MOD 整合文档
  image/                    镜像设计、来源、候选评估
  production-environment.md 生产环境部署事实与运维入口

.code/                      上下文工程入口（README、project-map、acceptance、knowledge、skills）
```

## 快速运行

仓库根目录的 `Makefile` 收纳了常用命令，`make help` 可查看完整列表。

### 本机开发管理端

```bash
make dev
# 默认监听 http://127.0.0.1:8788/，使用 ./.tmp-data 模拟容器内的 /data
# 本地无 supervisord，重启/状态接口会报错；MOD 配置生成、cluster 文件落盘
# 等流程仍可正常验证
```

可用的环境变量覆盖：`PORT`、`LISTEN`、`DEV_DATA`、`ADMIN_KEY`。

### 构建镜像

```bash
cd docker
docker build -f Dockerfile -t dst-waystone:local ..
```

准备运行时环境变量：

```bash
cd docker
cp .env.example .env
# 编辑 .env，填入 DST_CLUSTER_TOKEN、DST_ADMIN_KEY 等
```

启动容器：

```bash
docker compose up -d
docker compose logs -f --tail=200
```

DST 数据持久化到名为 `dst-waystone_dst-data` 的 docker named volume，容器内挂载点是 `/data`。

## 端口

```text
8788/tcp    管理端 HTTP
10999/udp   Master shard
11000/udp   Caves shard
12346/udp   Master Steam
12347/udp   Caves Steam
```

云厂商安全组和服务器防火墙都需要放行上述 UDP。`8788/tcp` 通常通过 Nginx 反代或 SSH 端口转发暴露，不直接面向公网。

## Klei cluster token

未提供 `DST_CLUSTER_TOKEN` 时，`entrypoint.sh` 不会写入 token，管理端会启动但 `dst-master`/`dst-caves` 保持停止。Token 也可登录管理端后在 "Klei Cluster Token" 卡片填入；管理端会把 token 原文保存到 `/data/admin/dst-admin.db`，并写回 `/data/cluster/Cluster_1/cluster_token.txt`（mode 0600）。API 和页面只显示保存状态、指纹和最近校验结果，不回显 token 原文。获取方式：

1. 登录 Klei Accounts。
2. 进入 Don't Starve Together → Game Servers。
3. 创建一个新的 server/cluster，复制生成的 token。

Token 是服务器归属凭证，不要进入公开仓库或聊天记录。

## 镜像来源与协议

`dst-waystone` 是从零编写的构建上下文，不复制 `jamesits/docker-dst-server` 或 `superjump22/dontstarve-server-docker` 的 Dockerfile、entrypoint、supervisor 配置或脚本。运行时通过 SteamCMD 下载 DST AppID `343050`。详细来源、协议边界和镜像内部配置见：

```text
docs/image/integrated-image-design.md
docs/image/docker-image-source.md
docs/image/docker-image-candidates.md
```

## 协作约定与上下文

- 协作守则：`AGENTS.md`
- 上下文工程入口：`.code/README.md`
- 当前阶段验收标准：`.code/context/acceptance.md`
