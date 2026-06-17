# DST 部署整合包

本目录保存从 `webhk` 服务器拉取的可复现部署资产。目标是让仓库同时覆盖：

- DST Docker 服务的 Compose 和分片配置。
- `dst-admin` 管理端的 systemd、Nginx 和运行配置模板。
- 当前线上 MOD 列表和生成的 Lua 配置。
- 线上镜像、端口、服务状态的审计快照。

不保存私密或大体积运行态：Klei cluster token、管理端 API key、玩家 Klei ID、服务器密码、世界存档、日志、Steam/UGC 下载目录和游戏二进制。

当前 DST Docker 镜像的来源、构建文件、GPL-2.0 协议边界和容器内关键配置见 `../docs/image/docker-image-source.md`。

## 目录说明

```text
deploy/
  server/
    docker-compose.yml      DST Compose 配置
    Cluster_1/              与线上 /opt/dst-server/data/DoNotStarveTogether/Cluster_1 对齐
      cluster.ini
      Master/{server.ini, leveldataoverride.lua, worldgenoverride.lua, modoverrides.lua}
      Caves/{server.ini, leveldataoverride.lua, worldgenoverride.lua, modoverrides.lua}
      mods/dedicated_server_mods_setup.lua
  admin-snapshot/           管理端线上配置与 MOD 状态快照（敏感字段占位）
    cluster.ini
    install-options.env
    server-settings.json
    server-mods.json
  nginx/                    管理端反代片段
  systemd/                  dst-admin.service 与环境变量示例
  runtime-state/            线上状态审计快照
```

服务器侧的安装脚本只保留唯一源 `scripts/install-dst-server.sh`；服务器上 `/root/install-dst-server.sh` 等同于此文件，差异请用 `git diff` 比对。

## 新服务器恢复顺序

1. 安装基础依赖：

```bash
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable --now docker
```

2. 部署 DST 服务：

```bash
sudo mkdir -p /opt/dst-server
sudo cp deploy/server/docker-compose.yml /opt/dst-server/docker-compose.yml
sudo cp deploy/admin-snapshot/install-options.env /opt/dst-server/install-options.env
```

3. 复制集群配置（与线上目录结构一致，整体拷贝）：

```bash
sudo mkdir -p /opt/dst-server/data/DoNotStarveTogether
sudo cp -R deploy/server/Cluster_1 /opt/dst-server/data/DoNotStarveTogether/Cluster_1
```

4. 在服务器本地创建私密文件：

```bash
sudo install -m 600 /dev/null /opt/dst-server/data/DoNotStarveTogether/Cluster_1/cluster_token.txt
sudo editor /opt/dst-server/data/DoNotStarveTogether/Cluster_1/cluster_token.txt
```

同时把 `deploy/server/Cluster_1/cluster.ini` 里的 `cluster_password = <set-on-server>` 改成真实服务器密码，或部署后通过管理端保存一次配置。

5. 启动 DST：

```bash
cd /opt/dst-server
sudo docker compose pull
sudo docker compose up -d
sudo docker compose ps
```

6. 部署管理端：

```bash
sudo mkdir -p /opt/dst-admin
sudo cp -R cmd docs go.mod mods web /opt/dst-admin/
sudo mkdir -p /opt/dst-admin/config
sudo cp deploy/admin-snapshot/server-mods.json /opt/dst-admin/mods/server-mods.json
sudo cp deploy/admin-snapshot/server-settings.json /opt/dst-admin/config/server-settings.json
sudo cp deploy/systemd/dst-admin.env.example /etc/dst-admin.env
sudo editor /etc/dst-admin.env
```

构建并安装二进制：

```bash
go build -o /tmp/mod-manager ./cmd/mod-manager
sudo cp /tmp/mod-manager /opt/dst-admin/mod-manager
sudo chmod 755 /opt/dst-admin/mod-manager
```

7. 安装 systemd 服务：

```bash
sudo cp deploy/systemd/dst-admin.service /etc/systemd/system/dst-admin.service
sudo systemctl daemon-reload
sudo systemctl enable --now dst-admin
sudo systemctl status dst-admin
```

8. 配置 Nginx：

把 `deploy/nginx/dst-admin-subpath.conf` 合并到目标 server block，或按宝塔面板的站点反代入口配置到 `127.0.0.1:8788`。

验证：

```bash
curl -i http://127.0.0.1:8788/api/auth/verify
curl -i http://127.0.0.1/dst-admin/
```

未带 `X-Admin-Key` 时，`/api/auth/verify` 应返回 `401 Unauthorized`。

## 当前线上差异提示

- `deploy/server/Cluster_1/cluster.ini` 是 DST 当前实际配置，`game_mode = endless`、`cluster_name = EXJR`。
- `deploy/admin-snapshot/server-settings.json` 是管理端草稿配置，当前显示 `game_mode = survival`、服务器名为中文名。
- 如果部署后点击管理端"保存配置"，管理端草稿会覆盖 DST 的 `cluster.ini` 和 `/opt/dst-server/install-options.env`。部署前应先确认采用哪一份作为真实源。
