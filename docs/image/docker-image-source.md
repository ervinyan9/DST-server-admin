# Docker 镜像来源与复现评估

记录日期：2026-06-17

本文只记录当前线上使用的 DST Docker 镜像来源、构建文件、协议、镜像内部配置和后续复现建议。当前阶段不复制上游镜像源码到本仓库。

## 当前结论

当前线上使用：

```text
jamesits/dst-server:latest
```

该镜像可以追溯到上游仓库：

```text
https://github.com/Jamesits/docker-dst-server
```

Docker Hub：

```text
https://hub.docker.com/r/jamesits/dst-server
```

上游仓库包含 `Dockerfile`、`azure-pipelines.yaml`、`docker-compose.yml`、默认 DST 配置、SteamCMD 脚本、系统入口脚本和 supervisor 配置，因此构建文件是可获得的。

许可证是 GPL-2.0。若后续复制、修改或分发上游源码/派生镜像，需要遵守 GPL-2.0 的源码提供、版权声明、许可证保留和派生分发要求。当前仓库仅引用和记录来源，不内嵌上游源码。

## 线上镜像指纹

从 `webhk` 服务器读取：

```text
Image ID: sha256:fa61065f8d2d770bc5d45f1a160b87b1deada3fd5903d9524b771321ca98dc58
Repo digest: jamesits/dst-server@sha256:fa61065f8d2d770bc5d45f1a160b87b1deada3fd5903d9524b771321ca98dc58
Created: 2022-05-21T03:28:51.583961111Z
Maintainer: James Swineson <docker@public.swineson.me>
Entrypoint: ["entrypoint.sh"]
Cmd: ["supervisord","-c","/etc/supervisor/supervisor.conf","-n"]
Volume: /data
Exposed UDP ports: 10999, 11000, 12346, 12347
```

Docker Hub `latest` tag 查询结果：

```text
Digest: sha256:fa61065f8d2d770bc5d45f1a160b87b1deada3fd5903d9524b771321ca98dc58
Last pushed: 2022-05-21T03:32:31Z
Architecture: linux/amd64
Size: about 2.15 GB
```

线上 digest 与 Docker Hub `latest` 当前 digest 一致。

## 上游仓库结构

上游 `master` 分支包含：

```text
Dockerfile
LICENSE
README.md
azure-pipelines.yaml
daocloud.yml
docker-compose.yml
dst_default_config/
scripts_steam/
scripts_system/
supervisor/
```

关键文件：

- `Dockerfile`：以 `debian:buster-slim` 为默认基础镜像，安装 SteamCMD、supervisor、i386 依赖和 DST server。
- `scripts_system/entrypoint.sh`：容器启动入口。
- `scripts_system/healthcheck.sh`：通过 `supervisorctl status` 判断健康状态。
- `scripts_system/dontstarve_dedicated_server_nullrenderer`：根据 `DST_SERVER_ARCH` 选择 x86 或 amd64 DST server。
- `scripts_steam/install_dst_server`：运行 SteamCMD `app_update 343050`。
- `scripts_steam/install_dst_server_initial`：首次构建时带 `validate` 的 SteamCMD 安装脚本。
- `supervisor/supervisor.conf`：启动 Master 和 Caves 两个 DST 进程。
- `dst_default_config/DoNotStarveTogether/Cluster_1`：默认集群配置。

## Dockerfile 构建逻辑

上游 Dockerfile 的核心逻辑：

1. 默认基础镜像是 `debian:buster-slim`。
2. 添加 i386 架构。
3. 安装 `ca-certificates`、`lib32stdc++6`、`libcurl3-gnutls:i386`、`wget`、`tar`、`supervisor` 等依赖。
4. 创建默认用户和组：`dst:dst`。
5. 安装 SteamCMD 到 `/opt/steamcmd/steamcmd.sh`。
6. 复制系统脚本到 `/usr/local/bin/`。
7. 复制 SteamCMD 脚本到 `/opt/steamcmd_scripts/`。
8. 下载 DST dedicated server 到 `/opt/dst_server`。
9. 复制默认配置到 `/opt/dst_default_config`。
10. 暴露 UDP `10999-11000` 和 `12346-12347`。
11. 使用 `entrypoint.sh` + `supervisord` 启动服务。

可用 build args：

```text
BASE_IMAGE
STEAMCMD_PATH
DST_DOWNLOAD
DST_USER
DST_GROUP
```

Azure Pipelines 中的主要变体：

- `latest` / `vanilla`
- `latest-slim` / `vanilla-slim`
- `steamcmd-rebase`
- `steamcmd-rebase-slim`
- `nightly`

上游 README 说明：`latest` 由 Docker Hub autobuild 构建，其他多数变体由 Azure DevOps CI 构建。

## 镜像内部关键配置

从线上容器只读检查得到。

### 目录约定

```text
/data                         DST 用户数据挂载点
/opt/dst_server               DST server 安装目录
/opt/dst_default_config       默认 DST 配置
/opt/steamcmd                 SteamCMD
/opt/steamcmd_scripts         SteamCMD runscript
/etc/supervisor               supervisor 配置
/usr/local/bin                entrypoint、healthcheck、wrapper
```

当前 compose 额外挂载：

```text
/opt/dst-server/data   -> /data
/opt/dst-server/server -> /opt/dst_server
/opt/dst-server/steam  -> /root/Steam
```

`/opt/dst-server/server` 挂载会覆盖镜像内预装的 DST server 目录，因此线上实际 DST server 文件存放在宿主机，容器启动时继续由 entrypoint 更新。

### 入口脚本行为

`entrypoint.sh` 在启动 `supervisord` 或直接启动 DST server 时执行：

1. 若 `/data/DoNotStarveTogether` 不存在，复制 `/opt/dst_default_config` 生成默认配置。
2. 若设置 `DST_CLUSTER_TOKEN` 环境变量，则写入 `cluster_token.txt`。
3. 检查 `cluster_token.txt`，并移除末尾换行。
4. 修复 `/data` 权限为 `dst:dst`。
5. 运行 SteamCMD 更新 `/opt/dst_server`。
6. 如果用户 mods 目录不存在，复制默认 mods 配置。
7. 将 `/opt/dst_server/mods` 替换为指向 `/data/DoNotStarveTogether/Cluster_1/mods` 的软链接。
8. 执行 DST 的 `-only_update_server_mods` 来下载 Workshop MOD。
9. 准备 supervisor socket。
10. 执行原始命令。

### supervisor 进程

`supervisor.conf` 启动两个程序：

```text
dst-server-master
dst-server-cave
```

两者都使用：

```text
dontstarve_dedicated_server_nullrenderer
  -skip_update_server_mods
  -persistent_storage_root /data
  -ugc_directory /data/ugc
  -cluster Cluster_1
```

Master 使用 `-shard Master`，Caves 使用 `-shard Caves`。

### SteamCMD 脚本

普通更新脚本：

```text
force_install_dir /opt/dst_server
login anonymous
app_update 343050
quit
```

初始安装脚本增加 `validate`：

```text
app_update 343050 validate
```

### 默认配置

镜像内默认配置位于：

```text
/opt/dst_default_config/DoNotStarveTogether/Cluster_1
```

包括：

```text
cluster.ini
Master/server.ini
Master/worldgenoverride.lua
Caves/server.ini
Caves/worldgenoverride.lua
mods/dedicated_server_mods_setup.lua
mods/modsettings.lua
adminlist.txt
blocklist.txt
whitelist.txt
```

默认 `cluster.ini` 使用 `game_mode = endless`、`max_players = 64`、`pvp = true`，并提示用户修改服务器名和描述。当前仓库 `deploy/server/Cluster_1/` 中保存的是我们线上实际配置，不是镜像默认配置。

## 与当前仓库的关系

当前仓库已经保存了线上宿主机层配置：

- `deploy/server/docker-compose.yml`
- `deploy/server/Cluster_1/cluster.ini`
- `deploy/server/Cluster_1/Master/server.ini`、`deploy/server/Cluster_1/Caves/server.ini`
- `deploy/server/Cluster_1/Master/worldgenoverride.lua`、`deploy/server/Cluster_1/Caves/worldgenoverride.lua`
- `deploy/server/Cluster_1/Master/modoverrides.lua`、`deploy/server/Cluster_1/Caves/modoverrides.lua`
- `deploy/server/Cluster_1/mods/dedicated_server_mods_setup.lua`
- `deploy/systemd/dst-admin.service`
- `deploy/nginx/dst-admin-subpath.conf`

但当前仓库还没有保存上游镜像源码，也没有 fork 或自建镜像。

## 开源分发建议

阶段一建议先采用“引用上游镜像”的方式：

1. README 明确注明 `jamesits/dst-server` 来源、Docker Hub、GitHub 和 GPL-2.0。
2. 本仓库只保存我们自己的 Compose、管理端、部署脚本和配置模板。
3. 镜像使用 digest pinning，避免 `latest` 漂移：

```yaml
image: jamesits/dst-server@sha256:fa61065f8d2d770bc5d45f1a160b87b1deada3fd5903d9524b771321ca98dc58
```

4. 如果后续需要修改镜像 entrypoint、supervisor 或默认配置，再建立 `third_party/docker-dst-server/` 或独立 fork，并完整保留 GPL-2.0 LICENSE、版权声明和源码分发说明。
5. 不把 Klei token、服务器密码、玩家 ID、存档、Steam 下载产物、Workshop UGC 内容和 DST 游戏二进制纳入开源仓库。

## 待决策

- 是否将 `deploy/server/docker-compose.yml` 从 `latest` 改为 digest pinning。
- 是否只引用上游，还是 fork 上游镜像。
- 如果 fork，上游 GPL-2.0 将影响镜像相关派生代码的分发方式，需要单独设计 `NOTICE` / `LICENSES` / `third_party` 结构。

可替代镜像候选和推荐路径见 `docs/image/docker-image-candidates.md`。
