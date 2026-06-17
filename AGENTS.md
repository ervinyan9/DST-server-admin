# DST-server-admin 项目协作说明

## 目标

本仓库用于保存《饥荒联机版》自建服务器的可复现部署资产、`dst-admin` 管理端、`dst-waystone` 生产构建上下文，以及后续饥荒 MOD 开发资料。

## 当前边界

- 线上运行资产在 `deploy/`，其中敏感值必须保持占位或示例。
- 管理端代码在 `cmd/mod-manager/`、`web/`、`mods/`。
- `dst-waystone` 构建上下文在 `docker/`，当前只维护 Dockerfile、entrypoint、supervisor 和说明文档；生产环境负责实际构建、密钥注入、数据挂载和服务编排。
- 饥荒 MOD 开发资料先沉淀在 `.code/knowledge/dst-mod-development/`，进入实际代码开发前必须先补齐机制理解、设计、验证和来源边界。

## 工作纪律

- 所有回答和项目说明优先使用中文；面向开源分发的用户文档后续补英文版。
- 修改前先读现有文件，不凭记忆改部署、镜像或 MOD 配置。
- 只做用户要求的范围，不顺手重构、不移动目录、不清理无关文件。
- 不提交 Klei token、服务器密码、玩家 ID、Steam 缓存、Workshop 下载内容、存档或真实管理密钥。
- 不复制 `jamesits/docker-dst-server`、`superjump22/dontstarve-server-docker` 或 Workshop MOD 的受限源码；允许记录来源、行为观察和重写计划。

## 上下文入口

- `.code/README.md`：上下文工程总入口。
- `.code/context/project-map.md`：仓库目录和职责索引。
- `.code/context/acceptance.md`：当前阶段验收标准。
- `.code/knowledge/dst-mod-development/README.md`：饥荒 MOD 开发知识库入口。
- `.code/skills/dst-mod-development.md`：饥荒 MOD 开发技能卡。

## 常用验证

```bash
go test ./...
go build -o /tmp/dst-mod-manager-check ./cmd/mod-manager
bash -n docker/entrypoint.sh
docker build -f docker/Dockerfile -t dst-waystone:local .
```

按任务风险选择验证粒度。文档和上下文改动至少做路径存在性检查、旧口径检索和敏感信息检索。
