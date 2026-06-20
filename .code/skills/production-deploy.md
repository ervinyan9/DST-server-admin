# 生产构建部署技能卡

## 适用场景

当任务涉及 `dst-waystone` / `dst-admin` 的生产部署、生产构建、发布前检查、生产备份、回滚、部署 SOP 调整或执行 `make prod-deploy` 时，先阅读本文件。

## 事实入口

- 生产环境事实：`docs/production-environment.md`
- 标准部署 SOP：`docs/plan/production-deploy-sop.md`
- 部署脚本：`scripts/deploy-prod.sh`
- Makefile 入口：`make prod-deploy`

## 执行原则

1. 先 dry-run
   - 默认只运行 `make prod-deploy` 或 `scripts/deploy-prod.sh --dry-run`。
   - 不要在没有明确指令时执行 `--execute`。

2. 先验证本地
   - 至少执行：
     ```bash
     GOTOOLCHAIN=go1.22.12 go test ./...
     go build -o /tmp/dst-mod-manager-check ./cmd/mod-manager
     bash -n docker/entrypoint.sh
     bash -n scripts/deploy-prod.sh
     ```
   - 本机 Docker 不可用时，说明 `docker build` 未验证，不把它写成已通过。

3. 先备份生产
   - 部署前必须备份：
     - `/opt/dst-waystone/docker/.env`
     - 容器内 `/data/admin/dst-admin.db`
     - 容器内 `/data/cluster/Cluster_1/`
   - 不输出真实 `.env`、Klei token、管理端密码、Admin key。

4. 再构建和重启
   - 生产构建在 `paycn:/opt/dst-waystone/docker` 执行。
   - 默认目标镜像为 `dst-waystone:local`。
   - Compose 服务名为 `dst-waystone`。

5. 部署后验收
   - 核对 `docker compose ps`。
   - 核对 `supervisorctl status`。
   - 核对 `/data/admin/dst-admin.db` 存在。
   - 核对 DST 二进制存在。
   - 核对 UDP 端口 `10999/11000/12346/12347`。
   - 查看 `Master/server_log.txt` / `Caves/server_log.txt`，确认没有新的 token 或启动致命错误。

6. 失败先回滚
   - 使用脚本输出的备份目录恢复 SQLite 和 `Cluster_1`。
   - 切回旧提交或旧镜像后重新 `docker compose up -d`。
   - 不在生产主机临场改密钥或手写配置绕过流程。

## 常用命令

```bash
make prod-deploy
make prod-deploy EXECUTE=1
scripts/deploy-prod.sh --dry-run
scripts/deploy-prod.sh --execute
```

## 输出要求

部署或部署排障结束时，必须说明：

- 本次是否实际执行生产变更。
- 使用的 ref / commit。
- 备份目录路径。
- 本地验证结果。
- 生产验收结果。
- 未完成项和风险。

## 禁止事项

- 不在聊天、日志、文档中输出真实 token、密码、Admin key。
- 不跳过 dry-run 和备份直接部署。
- 不把历史运行态结论写成当前事实，生产状态必须重新核验。
- 不把 `dst-admin.db`、存档、Workshop 下载内容、Steam 缓存提交进 Git。
