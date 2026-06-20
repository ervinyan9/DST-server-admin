# 生产环境说明

更新时间：2026-06-18

本文只记录可复用的生产部署事实和运维入口。真实 Klei token、管理端密码、派生 Admin key、存档、Workshop 下载内容不进入 Git。

## 当前生产节点

- SSH 别名：`paycn`
- 部署目录：`/opt/dst-waystone`
- Compose 目录：`/opt/dst-waystone/docker`
- Compose 服务：`dst-waystone`
- 容器名：`dst-waystone`
- 镜像：`dst-waystone:local`
- 数据卷：Compose named volume 挂载到容器 `/data`
- 管理端容器内启动参数：`-root=/opt/dst/admin -dst-dir=/data -listen=0.0.0.0 -port=8788 -supervisor-conf=/opt/dst/runtime/supervisord.conf`

## 公网入口

- 域名：`dst.keleeke.com`
- 公网 IP：`134.175.227.83`
- 宝塔站点 ID：`30`
- 宝塔反向代理名：`dst-admin`
- 反向代理上游：`http://127.0.0.1:8788`
- 管理端公网入口：`https://dst.keleeke.com/`

`8788/tcp` 是容器和宿主机内部管理端端口，公网访问优先走宝塔/Nginx 反向代理，不直接依赖裸端口。

## 容器端口

```text
8788/tcp      dst-admin HTTP
10999/udp     Master shard
11000/udp     Caves shard
12346/udp     Master Steam
12347/udp     Caves Steam
```

## 运行时环境变量

生产 `.env` 位于 `/opt/dst-waystone/docker/.env`，权限应保持 `0600`。可记录的非密钥项如下：

```text
DST_SKIP_GAME_UPDATE=true
DST_UPDATE_MODS_ON_START=false
DST_WORKSHOP_DOWNLOAD_TIMEOUT=10m
HTTP_PROXY=http://172.17.0.1:7890
HTTPS_PROXY=http://172.17.0.1:7890
http_proxy=http://172.17.0.1:7890
https_proxy=http://172.17.0.1:7890
NO_PROXY=localhost,127.0.0.1,::1
no_proxy=localhost,127.0.0.1,::1
```

以下变量存在于生产 `.env`，但不得记录真实值：

```text
DST_ADMIN_USERNAME
DST_ADMIN_PASSWORD
DST_ADMIN_KEY
DST_CLUSTER_TOKEN
```

## 代理与加速

当前生产使用已有 Mihomo 代理，不在本仓库安装或管理 Mihomo。

- Mihomo 监听：`172.17.0.1:7890`
- 容器 `.env` 注入：`HTTP_PROXY` / `HTTPS_PROXY` / 小写同名变量
- 已验证：宿主机通过 `--proxy http://172.17.0.1:7890` 访问 Steam Web 返回 `200`
- 构建期可通过 Docker build args 使用代理：
  - `USE_BUILD_PROXY=true`
  - `BUILD_HTTP_PROXY=http://172.17.0.1:7890`
  - `BUILD_HTTPS_PROXY=http://172.17.0.1:7890`
  - `BUILD_NO_PROXY=localhost,127.0.0.1,::1`

APT 构建加速使用腾讯 Debian HTTP 源：

```text
APT_MIRROR=http://mirrors.tencent.com/debian
APT_SECURITY_MIRROR=http://mirrors.tencent.com/debian-security
```

## 当前运行状态

2026-06-18 核验状态：

- 容器 `dst-waystone`：`Up`
- 部署提交：`a02f8f7`
- 镜像 ID：`d8097b3692d8`
- `dst-admin`：`RUNNING`
- `dst-master`：`STOPPED`
- `dst-caves`：`STOPPED`
- Klei token 文件：已存在，权限 `0600`
- DST 服务端二进制：缺失
- `/opt/dst/game`：为空

因此当前管理端可用，保存 Klei token 可用，但真正启动 DST shard 前仍需先完成 SteamCMD 下载 AppID `343050`。在游戏二进制缺失时，管理端 `/api/restart` 会返回明确错误，而不是 supervisor 的 `no such file`。

## 常用核验命令

不要输出真实 `.env` 内容。需要查看时必须先脱敏：

```bash
ssh paycn 'cd /opt/dst-waystone/docker && docker compose ps'
ssh paycn 'cd /opt/dst-waystone/docker && docker images dst-waystone:local --format "{{.ID}} {{.Size}} {{.CreatedSince}}"'
ssh paycn 'cd /opt/dst-waystone/docker && sed -E "s/^([^#=]*(TOKEN|PASSWORD|PASS|KEY|SECRET|ADMIN)[^#=]*=).*/\1<redacted>/I" .env'
ssh paycn 'docker exec dst-waystone supervisorctl -c /opt/dst/runtime/supervisord.conf status'
ssh paycn 'docker exec dst-waystone sh -lc "test -x /opt/dst/game/bin64/dontstarve_dedicated_server_nullrenderer_x64 && echo present || echo missing"'
ssh paycn 'ss -ltnp | grep -E ":7890\b" || true'
ssh paycn 'curl -sS -o /tmp/steam-proxy-check.txt -w "%{http_code}\n" --proxy http://172.17.0.1:7890 --connect-timeout 8 --max-time 20 https://store.steampowered.com/'
```

## 旧生产节点记录

旧 `webhk` 节点曾运行旧版 `dst-server` / `dst-admin` 栈。会话 `019ed8c5-7882-7de1-b72c-de284d16b6a0` 中已执行过备份、停服务、清理 Nginx 入口、防火墙端口、旧目录和旧镜像的下线流程。当前生产以 `paycn` 为准。
