# DST Docker 镜像候选评估

记录日期：2026-06-17

目标：确认是否存在比当前 `jamesits/dst-server:latest` 更适合做一体化封装的 DST Docker 镜像。

## 当前结论

存在更新的镜像候选。最值得继续评估的是：

1. `superjump22/dontstarvetogether:latest`
2. `webhippie/dst:latest`
3. `jamesits/dst-server:nightly`

推荐路径：

- 如果目标是“最简单地重新封装一体化镜像”，优先基于 `superjump22/dontstarvetogether:latest` 做我们的上层镜像。
- 如果目标是“环境变量驱动配置生成能力最完整”，评估 `webhippie/dst:latest`。
- 不建议继续基于 `jamesits/dst-server:latest` 做长期方案；它的 `latest` 已是 2022 年镜像。
- `jamesits/dst-server:nightly` 比 `latest` 新，但上游项目整体仍旧，且许可证是 GPL-2.0，不是最省心的封装底座。

## 候选对比

| 镜像 | 最近推送 | 源码 | 许可证 | 优点 | 主要问题 |
| --- | --- | --- | --- | --- | --- |
| `jamesits/dst-server:latest` | 2022-05-21 | `Jamesits/docker-dst-server` | GPL-2.0 | 当前线上已验证；自带 entrypoint、supervisor、Master/Caves | 过旧；GPL 派生边界更重 |
| `jamesits/dst-server:nightly` | 2025-07-12 | `Jamesits/docker-dst-server` | GPL-2.0 | 比 `latest` 新；同项目迁移成本较低 | 项目源码最后主要更新仍旧；镜像更大；仍是 GPL-2.0 |
| `webhippie/dst:latest` | 2026-06-16 | `dockhippie/dst` | MIT | 活跃；配置项非常完整；GitHub Actions；MIT | 只暴露单 shard 运行模型；entrypoint 体系复杂；需要我们重新设计 Master/Caves/admin 编排 |
| `superjump22/dontstarvetogether:latest` | 2026-06-11 | `superjump22/dontstarve-server-docker` | MIT | 活跃；中英 README；Dockerfile 简洁；直接以官方 64-bit server 为 entrypoint；适合作为程序基础镜像 | 不提供完整专服编排；Master/Caves、MOD 更新、admin 需要我们自己封装 |
| `dstgo/dst-server-x64:latest` | 2025-06-17 | `dstgo/wilson` 相关 | 不明确 | 64-bit；基于官方 SteamCMD Debian 镜像；有面板生态线索 | 镜像仓库与源码/许可证关系不够清晰；不适合直接作为开源分发底座 |

## 详细记录

### 当前线上镜像

当前线上使用：

```text
jamesits/dst-server:latest
```

线上 digest：

```text
sha256:fa61065f8d2d770bc5d45f1a160b87b1deada3fd5903d9524b771321ca98dc58
```

Docker Hub `latest` 最近推送：

```text
2022-05-21T03:32:31Z
```

结论：稳定但过旧，不适合作为长期一体化镜像的唯一基础。

### `jamesits/dst-server:nightly`

Docker Hub：

```text
Image: jamesits/dst-server:nightly
Digest: sha256:60bb461a1607487d58ad1c238ac2432d3262fd12298581fb77fc5f0077311ca8
Last pushed: 2025-07-12T03:13:28Z
Size: about 3.15 GB
```

优点：

- 与当前线上镜像同源。
- 理论迁移成本低。
- 比 `latest` 新。

问题：

- 上游仓库 `Jamesits/docker-dst-server` 的主要源码更新仍停留在 2022 年。
- 许可证是 GPL-2.0。
- 如果我们复制或派生镜像源码，需要按 GPL-2.0 处理镜像相关派生代码。

### `webhippie/dst:latest`

Docker Hub：

```text
Image: webhippie/dst:latest
Digest: sha256:6fb8ac971f29a7df6cf0308f14a282c1440b6f3ceb8f5df2b883ac5f64c1cc53
Last pushed: 2026-06-16T02:51:03Z
Size: about 3.37 GB
```

GitHub：

```text
https://github.com/dockhippie/dst
```

许可证：

```text
MIT
```

源码状态：

- GitHub `pushed_at`: `2026-06-16T02:49:22Z`
- Dockerfile 路径：`latest/Dockerfile.amd64`
- 基础镜像：`ghcr.io/dockhippie/steamcmd:latest-amd64`
- 使用 `/usr/bin/container` 入口框架和 `/etc/entrypoint.d/*.sh`

特点：

- 环境变量配置非常完整，覆盖 cluster、gameplay、network、world override、mods 等大量 DST 配置。
- 支持 `DST_CLUSTER_TOKEN`、`DST_SERVER_MOD_SETUP`、`DST_SERVER_MOD_COLLECTION_SETUP`、`DST_MOD_OVERRIDES_RAW` 等配置。
- MIT 许可证更适合我们做开源封装。

问题：

- 默认模型更像“一个容器运行一个 shard”。
- 当前线上是一个容器内 supervisor 同时跑 Master/Caves；迁移到 `webhippie/dst` 需要重做编排。
- 它的入口脚本配置生成能力强，但也会增加我们管理端与镜像入口之间的契约复杂度。

适合场景：

- 我们希望尽量把 DST 配置全部环境变量化。
- 愿意接受更复杂的 entrypoint 契约。
- 后续可能拆成多容器 Master/Caves 架构。

### `superjump22/dontstarvetogether:latest`

Docker Hub：

```text
Image: superjump22/dontstarvetogether:latest
Digest: sha256:16e036776886a1255c63ef390afc817f4fa67357b61eb9d87ead6bb6e10ed8a4
amd64 image digest: sha256:e555f910d86064c0f914cebd593dc5a702a1229ad7846cf5f12b0ec629cb5bea
Last pushed: 2026-06-11T23:46:09Z
Size: about 3.54 GB
```

GitHub：

```text
https://github.com/superjump22/dontstarve-server-docker
```

许可证：

```text
MIT
```

源码状态：

- GitHub `pushed_at`: `2026-06-11T23:37:46Z`
- Dockerfile 路径：`game/Dockerfile`
- 基础镜像：`cm2network/steamcmd:root`
- 直接以 `dontstarve_dedicated_server_nullrenderer_x64` 作为 entrypoint。
- 提供英文 README 和中文 README。

特点：

- Dockerfile 简单。
- 只封装最新 DST dedicated server 程序。
- 运行参数基本等同官方 DST dedicated server 命令行参数。
- 明确区分 save、mods、ugc_mods。
- README 对 Master/Caves、MOD 更新给了示例。

问题：

- 它不是完整“一键专服管理镜像”，只是 DST server 程序镜像。
- Master/Caves 编排、配置初始化、MOD 下载、admin、健康检查都需要我们上层镜像实现。

适合场景：

- 我们要做自己的“一体化镜像”，并希望基础层尽量简单、活跃、MIT。
- 管理端成为控制中心，由我们生成配置、启动 Master/Caves、下载 MOD、维护状态。

### `dstgo/dst-server-x64:latest`

Docker Hub：

```text
Image: dstgo/dst-server-x64:latest
Last pushed: 2025-06-17T05:33:25Z
Size: about 3.13 GB
```

相关 GitHub：

```text
https://github.com/dstgo/wilson
```

问题：

- Docker Hub 描述指向 `dstgo/wilson`，但该仓库是面板后端，不是清晰的镜像源码根。
- GitHub API 显示 license 为 `NOASSERTION`。
- 不适合作为我们当前开源分发底座。

## 推荐方案

### 最简单可推进方案

基于 `superjump22/dontstarvetogether:latest` 做我们的整合镜像。

思路：

```text
superjump22/dontstarvetogether:latest
  + dst-admin 二进制
  + web 静态资源
  + 我们自己的 entrypoint / supervisor
  + 配置初始化脚本
  + MOD 配置生成
  + Master/Caves 启动编排
```

理由：

- 上游活跃。
- MIT 许可证。
- Dockerfile 简单，边界清楚。
- 中英 README 与我们的多语言目标一致。
- 由我们掌控 admin 与 DST 进程编排，后续做服务整合 model 更顺。

### 不推荐直接替换线上镜像

不建议现在把线上 `jamesits/dst-server:latest` 直接替换为 `webhippie/dst` 或 `superjump22/dontstarvetogether`。

原因：

- 目录结构不同。
- 启动入口不同。
- MOD 下载/UGC 目录不同。
- Master/Caves 编排不同。
- 当前 admin 的保存/重启逻辑绑定 `/opt/dst-server` 和现有 compose 目录。

更稳妥的路径是先做本地/测试服务器一体化镜像验证，再迁移线上。

## 下一步建议

1. 新建镜像设计文档，明确一体化镜像目录结构和进程模型。
2. 选择 `superjump22/dontstarvetogether` 作为基础镜像做 PoC。
3. 把 `dst-admin` 编译进镜像。
4. 用 supervisor 或自定义 entrypoint 同时运行：
   - `dst-admin`
   - Master shard
   - Caves shard
5. 持久化目录统一设计为：

```text
/data
  cluster/
  mods/
  ugc_mods/
  admin/
```

6. 先实现“配置 Klei token 后能启动完整 Master+Caves+Admin”的最小体验，再迁移 MOD/资源减负能力。
