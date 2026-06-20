# dst-waystone 生产部署 SOP

## 目标

把 `dst-admin` 与 `dst-waystone` 镜像更新部署到生产节点 `paycn`，同时保护运行态数据、密钥和可回滚能力。

## 适用范围

- 生产主机：`paycn`
- 部署目录：`/opt/dst-waystone`
- Compose 目录：`/opt/dst-waystone/docker`
- Compose 服务：`dst-waystone`
- 容器名：`dst-waystone`
- 镜像名：`dst-waystone:local`
- 运行态数据：Docker named volume 挂载到容器 `/data`

## 关键原则

- 不输出真实 `.env`、Klei token、管理端密码、Admin key。
- 部署前必须备份 `/data/admin/dst-admin.db`、`/data/cluster/Cluster_1/` 和生产 `.env`。
- `dst-admin` 的 SQLite 数据库是管理端状态主存储；`cluster.ini`、`server.ini`、`cluster_token.txt` 是运行态文件，必须纳入备份和验收。
- 默认先 dry-run，只有显式 `--execute` 才允许修改生产。
- 失败后优先回滚镜像和备份，不在生产主机上临场改密钥或手工重写配置。

## 前置条件

1. 本地改动已完成测试：

   ```bash
   GOTOOLCHAIN=go1.22.12 go test ./...
   go build -o /tmp/dst-mod-manager-check ./cmd/mod-manager
   bash -n docker/entrypoint.sh
   ```

2. 生产主机可 SSH：

   ```bash
   ssh paycn 'hostname'
   ```

3. 生产目录存在：

   ```bash
   ssh paycn 'test -d /opt/dst-waystone && test -d /opt/dst-waystone/docker'
   ```

## 标准流程

### 1. Dry-run

先查看将执行哪些远端命令：

```bash
scripts/deploy-prod.sh --dry-run
```

或通过 Makefile：

```bash
make prod-deploy
```

### 2. 执行部署

确认 dry-run 输出合理后执行：

```bash
scripts/deploy-prod.sh --execute
```

或：

```bash
make prod-deploy EXECUTE=1
```

### 3. 部署脚本做的事

1. 读取生产当前提交、容器状态、镜像信息。
2. 在生产主机创建 `/opt/dst-waystone/backups/<timestamp>/`。
3. 备份：
   - `/opt/dst-waystone/docker/.env`
   - 容器内 `/data/admin/dst-admin.db`
   - 容器内 `/data/cluster/Cluster_1/`
4. 在生产目录执行 `git fetch` 和 `git checkout <ref>`。
5. 在生产主机构建 `dst-waystone:local`。
6. 执行 `docker compose up -d --no-deps dst-waystone`。
7. 验收管理端、supervisor 状态、DST 二进制、端口监听和最近日志。

## 验收标准

部署后必须确认：

```bash
ssh paycn 'cd /opt/dst-waystone/docker && docker compose ps'
ssh paycn 'docker exec dst-waystone supervisorctl -c /opt/dst/runtime/supervisord.conf status'
ssh paycn 'docker exec dst-waystone sh -lc "test -f /data/admin/dst-admin.db && echo db=present"'
ssh paycn 'docker exec dst-waystone sh -lc "test -x /opt/dst/game/bin64/dontstarve_dedicated_server_nullrenderer_x64 && echo binary=present || echo binary=missing"'
ssh paycn 'ss -lunp | grep -E ":(10999|11000|12346|12347)\b" || true'
```

管理端状态接口应能返回 JSON，且不回显 token 原文。

## 回滚

脚本输出会显示本次备份目录，例如：

```text
/opt/dst-waystone/backups/20260620-153000
```

如部署失败，优先回滚：

1. 切回旧提交：

   ```bash
   ssh paycn 'cd /opt/dst-waystone && git checkout <old-sha>'
   ```

2. 恢复运行态备份：

   ```bash
   ssh paycn 'docker cp /opt/dst-waystone/backups/<timestamp>/dst-admin.db dst-waystone:/data/admin/dst-admin.db'
   ssh paycn 'docker cp /opt/dst-waystone/backups/<timestamp>/Cluster_1 dst-waystone:/data/cluster/'
   ```

3. 重建并启动：

   ```bash
   ssh paycn 'cd /opt/dst-waystone/docker && docker compose up -d --build'
   ```

4. 重新执行验收命令。

## 常见风险

- Docker 构建失败：保留旧容器，不继续重启。
- SQLite 文件缺失：先查备份目录，再从容器 `/data/admin/` 验证。
- Token 状态异常：查看管理端 Token 状态和 `/data/cluster/Cluster_1/Master/server_log.txt`，不要在日志或文档中输出 token 原文。
- DST 端口未监听：先核对 Compose 端口映射和宿主机防火墙，再看 `Master/server_log.txt`。
