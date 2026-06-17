# Don't Starve Together Docker Server

这个仓库保存《饥荒联机版》专用服务器的部署脚本、默认配置和操作记录。

设计原则：

- 服务器是 Linux/Ubuntu，脚本直接在服务器上运行。
- 本地只保存脚本和文档，方便备份、审阅和之后同步。
- 除了 Klei server token，其他配置都可以由脚本自动生成。
- Docker 镜像在服务器端拉取，存档和配置保存在 `/opt/dst-server/data`。

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
- 本地已有 SSH 别名 `dst-hk`，目标是 `43.161.236.162`，默认用户 `root`。
- 服务器当前系统是 OpenCloudOS 9.4，已有 Docker 29.5.3 和 Docker Compose v5.1.4。
- SSH host key 已经写入本地 `known_hosts`。

## 当前服务器部署方式

本地把脚本上传到服务器：

```powershell
scp .\scripts\install-dst-server.sh dst-hk:/root/install-dst-server.sh
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
