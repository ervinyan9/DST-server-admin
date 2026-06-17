# Don't Starve Together Docker Server

这个仓库保存《饥荒联机版》专用服务器的部署脚本、默认配置、远程管理端和操作记录。

设计原则：

- 服务器是 Linux/Ubuntu，脚本直接在服务器上运行。
- 本地保存脚本、管理端代码、可复现部署配置和文档，方便备份、审阅和之后同步。
- 除了 Klei server token，其他配置都可以由脚本自动生成。
- Docker 镜像在服务器端拉取，存档和配置保存在 `/opt/dst-server/data`。

## 当前整合包

服务器当前可复现部署资产已保存到：

```text
deploy/
```

其中包含：

- `deploy/server/docker-compose.yml`：当前线上 DST Docker Compose 配置。
- `deploy/server/Cluster_1/`：当前线上 `cluster.ini`、Master/Caves 分片配置和 MOD Lua 配置，敏感字段已占位。
- `deploy/systemd/dst-admin.service`：当前线上管理端 systemd 服务。
- `deploy/systemd/dst-admin.env.example`：管理密钥环境文件示例，真实值不进仓库。
- `deploy/nginx/dst-admin-subpath.conf`：当前管理端 subpath 反代配置片段。
- `deploy/admin-snapshot/`：从线上拉取的管理端配置与 MOD 状态快照，敏感字段已占位。
- `deploy/runtime-state/2026-06-17-webhk.md`：本次从线上采集的服务、镜像、端口和防火墙状态。

恢复或迁移服务器时，优先阅读 `deploy/README.md`，再按需使用 `scripts/install-dst-server.sh`。

当前使用的 Docker 镜像来源、构建文件、协议和线上镜像内部配置见 `docs/image/docker-image-source.md`。可替代镜像候选和推荐路径见 `docs/image/docker-image-candidates.md`。

## dst-waystone 生产构建上下文

本仓库正在推进 `dst-waystone` 服务封装上下文，目标是维护 Dockerfile、默认配置模板和自写启动编排，后续由生产环境负责实际镜像构建、密钥注入、数据挂载和服务编排。

设计文档：

```text
docs/image/integrated-image-design.md
```

本地构建：

```bash
docker build -f docker/Dockerfile -t dst-waystone:local .
```

详细运行方式见 `docker/README.md`。该构建上下文通过 SteamCMD 在运行时下载或更新 DST AppID `343050`，也可以用 `--build-arg DST_DOWNLOAD_AT_BUILD=true` 尝试在构建期下载。它不复制 `jamesits/docker-dst-server` 或 `superjump22/dontstarve-server-docker` 的 Dockerfile、entrypoint、supervisor 配置或脚本。

## 快速部署

在一台新的 Linux 服务器上：

```bash
sudo apt-get update
sudo apt-get install -y curl
curl -fsSL https://example.invalid/install-dst-server.sh -o install-dst-server.sh
sudo bash install-dst-server.sh \
  --token '替换成你的 Klei cluster token' \
  --server-name '服务器名称' \
  --server-password '服务器密码'
```

如果脚本已经在当前目录，可以直接：

```bash
sudo bash scripts/install-dst-server.sh \
  --token '替换成你的 Klei cluster token' \
  --server-name 'Our DST' \
  --server-password 'secret'
```

也可以把 token 写到服务器上的临时文件：

```bash
sudo bash scripts/install-dst-server.sh \
  --token-file /root/dst-token.txt \
  --server-name 'Our DST' \
  --server-password 'secret'
```

## 服务器端口

云厂商安全组和服务器防火墙都需要开放 UDP：

```text
10999
11000
12346
12347
```

玩家连接主要使用 `10999/udp`。如果启用洞穴，脚本会同时生成 `Master` 和 `Caves` 分片配置。

## 目录结构

脚本默认写入：

```text
/opt/dst-server/
  docker-compose.yml
  install-options.env
  data/
    DoNotStarveTogether/
      Cluster_1/
        cluster.ini
        cluster_token.txt
        Master/
          server.ini
          leveldataoverride.lua
          modoverrides.lua
        Caves/
          server.ini
          leveldataoverride.lua
          modoverrides.lua
        mods/
          dedicated_server_mods_setup.lua
```

`cluster_token.txt` 是私密文件，不要提交到公开仓库。

## 常用维护命令

```bash
cd /opt/dst-server
sudo docker compose ps
sudo docker compose logs -f --tail=120
sudo docker compose restart
sudo docker compose pull && sudo docker compose up -d
```

备份：

```bash
sudo tar -czf "/root/dst-backup-$(date +%F-%H%M).tar.gz" -C /opt/dst-server data
```

恢复时先停服，再把 `data` 解回 `/opt/dst-server`：

```bash
cd /opt/dst-server
sudo docker compose down
sudo tar -xzf /root/dst-backup-YYYY-MM-DD-HHMM.tar.gz -C /opt/dst-server
sudo docker compose up -d
```

## 获取 Klei Token

1. 打开 Klei Accounts。
2. 进入 Don't Starve Together 的 Game Servers 页面。
3. 创建一个新的 server/cluster。
4. 复制生成的 cluster token。
5. 部署时通过 `--token` 或 `--token-file` 提供。

Token 是服务器归属凭证，不要发到聊天记录或公开仓库里。

## 本地当前自检记录

- 本地 Windows 当前没有 `docker` 命令，所以本地不作为运行环境。
- 本地当前可用 SSH 别名是 `webhk`，目标是 `43.161.236.162`，默认用户 `root`。
- 服务器当前系统是 OpenCloudOS 9.4，已有 Docker 29.5.3 和 Docker Compose v5.1.4。
- SSH host key 已经写入本地 `known_hosts`。

## 当前服务器部署方式

本地把脚本上传到服务器：

```powershell
scp .\scripts\install-dst-server.sh webhk:/root/install-dst-server.sh
```

服务器上运行：

```bash
sudo bash /root/install-dst-server.sh \
  --token '替换成你的 Klei cluster token' \
  --server-name 'Our DST' \
  --server-password 'secret'
```

如果 token 放在服务器文件里：

```bash
sudo bash /root/install-dst-server.sh \
  --token-file /root/dst-token.txt \
  --server-name 'Our DST' \
  --server-password 'secret'
```
