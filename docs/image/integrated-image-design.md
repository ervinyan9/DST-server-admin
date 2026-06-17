# dst-waystone 生产构建上下文设计

记录日期：2026-06-17

## 目标

维护一个本仓库自有的 `dst-waystone` 生产构建上下文，内容包含：

- DST dedicated server 运行环境的 Dockerfile 定义。
- 本仓库的 `dst-admin` Web 管理端构建步骤。
- 本仓库自写的启动编排脚本。
- 本仓库自写的 supervisor 配置。
- 默认配置模板与运行时初始化逻辑。

第一阶段成功标准：`dst-waystone` 镜像可以根据本构建上下文构建出来，并能在不复制 `jamesits/docker-dst-server` GPL 源码的前提下形成清晰的后续生产接入基础。

## 非目标

第一阶段不实现：

- 线上服务迁移。
- 游戏内减负 MOD。
- 完整多语言 UI 改造。
- 自动发布到 Docker Hub/GHCR。
- 在本仓库内替代生产环境完成镜像发布、密钥注入、数据挂载和服务编排。
- 在构建产物中提交 Klei token、服务器密码、玩家 ID、存档、Steam/UGC 下载目录。

## 来源与协议边界

参考来源：

- `superjump22/dontstarve-server-docker`：参考“基于 SteamCMD 下载 DST dedicated server”的思路，不复制代码。
- `steamcmd/steamcmd` 或 `cm2network/steamcmd`：作为基础镜像候选。
- Valve SteamCMD：用于下载 DST dedicated server。
- Klei DST dedicated server：运行时/构建时通过 SteamCMD 下载，二进制不进入 Git。

明确边界：

- 不复制 `jamesits/docker-dst-server` 的 Dockerfile、entrypoint、supervisor 或脚本。
- 不复制 `superjump22/dontstarve-server-docker` 的 Dockerfile 或脚本。
- 本仓库 Dockerfile、entrypoint 和 supervisor 从零编写。
- 后续公开分发时，README/NOTICE 需注明 SteamCMD、Klei DST dedicated server、参考项目和对应协议。

## 基础镜像选择

第一阶段采用：

```text
cm2network/steamcmd:root
```

原因：

- 已包含 SteamCMD。
- `superjump22/dontstarve-server-docker` 也使用该基础镜像，说明其对 DST server 构建可行。
- root 变体便于第一阶段安装系统依赖和构建 PoC。

后续可以评估切换到：

```text
steamcmd/steamcmd:debian
```

切换条件：

- 官方镜像能稳定安装 DST x64 运行依赖。
- 目录和用户权限契约更适合 `dst-waystone` 生产构建上下文。

## 镜像目录结构

镜像内：

```text
/opt/dst/game              DST dedicated server 安装目录
/opt/dst/admin             dst-admin 管理端目录
/opt/dst/runtime           自写运行脚本和 supervisor 配置
/data                      用户持久化数据根目录
/data/cluster              DST persistent_storage_root/conf_dir 根目录
/data/mods                 DST legacy mods 目录
/data/ugc_mods             DST UGC mods 目录
/data/admin                管理端运行状态目录
```

第一阶段保留现有 admin 代码的目录习惯：

```text
/opt/dst/admin/mods/server-mods.json
/opt/dst/admin/config/server-settings.json
/opt/dst/admin/web
```

## 构建策略

第一阶段默认不在镜像构建时下载 DST server，而是先验证 `dst-waystone` 构建上下文：

```text
docker build -f docker/Dockerfile -t dst-waystone:local .
```

原因：

- 当前 SteamCMD 可启动并匿名登录，但 `app_update 343050` 在验证环境中返回 `Missing configuration`，不适合作为第一阶段镜像构建的硬阻塞。
- 本地 Docker 是 arm64，SteamCMD 32 位程序在 amd64 仿真下会 `Segmentation fault`，不能作为 DST 下载验证环境。
- 第一阶段先保证 Dockerfile、管理端、entrypoint 和 supervisor 能构建成可运行镜像。
- 运行时可通过 `DST_SKIP_GAME_UPDATE=false` 拉取或更新 DST。

如果需要构建期下载 DST server，可以显式启用：

```text
docker build \
  --build-arg DST_DOWNLOAD_AT_BUILD=true \
  -f docker/Dockerfile \
  -t dst-waystone:local .
```

构建期下载命令：

```text
steamcmd +force_install_dir /opt/dst/game +login anonymous +app_update 343050 validate +quit
```

风险：

- 构建期下载会让镜像体积较大。
- 构建期下载依赖 Steam 网络和 Steam app 配置状态。
- 生产环境或公开分发产物如包含 DST 二进制，需要说明 DST 二进制来自 SteamCMD 下载，不属于本仓库源码。

## 进程模型

第一阶段使用 supervisor 管理三个进程：

```text
dst-admin
dst-master
dst-caves
```

`dst-admin`：

```text
/opt/dst/admin/mod-manager \
  -root /opt/dst/admin \
  -listen 0.0.0.0 \
  -port 8788 \
  -dst-dir /data
```

`dst-master`：

```text
/opt/dst/game/bin64/dontstarve_dedicated_server_nullrenderer_x64 \
  -skip_update_server_mods \
  -ugc_directory /data/ugc_mods \
  -persistent_storage_root /data \
  -conf_dir cluster \
  -cluster Cluster_1 \
  -shard Master
```

`dst-caves`：

```text
/opt/dst/game/bin64/dontstarve_dedicated_server_nullrenderer_x64 \
  -skip_update_server_mods \
  -ugc_directory /data/ugc_mods \
  -persistent_storage_root /data \
  -conf_dir cluster \
  -cluster Cluster_1 \
  -shard Caves
```

说明：

- `-persistent_storage_root /data` 与 `-conf_dir cluster` 组合后，DST 配置路径为 `/data/cluster/Cluster_1`。
- `dst-admin` 当前 `-dst-dir` 仍面向旧 `/opt/dst-server` compose 目录；第一阶段先让构建上下文可构建，后续再让 admin 适配 `/data` 运行目录。
- 运行时如未提供 Klei token，entrypoint 只初始化文件并提示缺 token；是否启动 DST 由 entrypoint 控制。

## 运行时配置

环境变量：

```text
DST_CLUSTER_TOKEN          Klei cluster token，可选；也可挂载 cluster_token.txt
DST_ADMIN_KEY              管理端 API key，建议必填
DST_SERVER_NAME            默认服务器名
DST_SERVER_PASSWORD        默认服务器密码
DST_GAME_MODE              默认 survival
DST_MAX_PLAYERS            默认 6
DST_ENABLE_CAVES           默认 true
DST_SKIP_GAME_UPDATE       默认 true；设为 false 时启动时执行 SteamCMD 更新
DST_UPDATE_MODS_ON_START   默认 false
```

持久化卷：

```text
/data
```

端口：

```text
8788/tcp       管理端
10999/udp      Master
11000/udp      Caves
12346/udp      Master Steam
12347/udp      Caves Steam
```

## 初始化逻辑

自写 entrypoint 执行：

1. 创建 `/data/cluster/Cluster_1/{Master,Caves,mods}`。
2. 创建 `/data/mods` 和 `/data/ugc_mods`。
3. 如 `/data/cluster/Cluster_1/cluster.ini` 不存在，生成默认配置。
4. 如 Master/Caves `server.ini` 不存在，生成默认分片配置。
5. 如 `dedicated_server_mods_setup.lua` 不存在，复制仓库当前生成配置或写入空模板。
6. 如 `modoverrides.lua` 不存在，复制仓库当前生成配置或写入空模板。
7. 如 `DST_CLUSTER_TOKEN` 存在，写入 `/data/cluster/Cluster_1/cluster_token.txt`。
8. 如 token 文件不存在或为空，不启动 `dst-master`/`dst-caves`，只启动 `dst-admin`，并输出清晰提示。
9. 如 `DST_UPDATE_MODS_ON_START=true`，启动前执行一次 `-only_update_server_mods`。
10. 启动 supervisor。

## 第一阶段实现范围

新增文件：

```text
docker/Dockerfile
docker/entrypoint.sh
docker/supervisord.conf
docker/README.md
docker/Dockerfile.dockerignore
```

不修改：

- 线上 `deploy/server/docker-compose.yml`
- 当前服务器
- 当前 admin API 行为

## 验证方式

本地验证：

```bash
docker build -f docker/Dockerfile -t dst-waystone:local .
```

默认构建成功信号：

- Go admin 在 builder stage 编译成功。
- 运行依赖安装成功。
- 镜像创建成功。

构建期下载 DST 的额外成功信号：

- SteamCMD 成功下载 AppID `343050`。

轻量检查：

```bash
docker image inspect dst-waystone:local
```

可选运行检查：

```bash
docker run --rm -p 8788:8788 dst-waystone:local
```

未提供 token 时，预期只启动管理端并提示需要配置 Klei token。

## 后续阶段

阶段二：

- 让 `dst-admin` 适配 `/data` 目录，不再依赖旧 compose 目录。
- 管理端支持直接控制 supervisor 进程或内部脚本，而不是执行 `docker compose restart`。
- 增加中英文 README 与 `.env.example`。

阶段三：

- 添加资源减负 MOD / model 的配置契约。
- 先整合现有 MOD 配置，再逐步实现自研 Lua 模块。

阶段四：

- 生产环境接入 `dst-waystone` 构建上下文。
- CI 构建镜像。
- 发布 GHCR/Docker Hub。
- 完善许可证、NOTICE、第三方来源声明。
