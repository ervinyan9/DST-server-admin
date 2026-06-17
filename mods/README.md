# 管理端 MOD 状态目录

本目录是 `dst-admin` 管理端（`cmd/mod-manager`）持有的 MOD 状态目录，包含两类内容：

- `server-mods.json`：仓库内的**种子状态**。第一次启动管理端时作为初始 MOD 列表。线上运行后，管理端会原地覆盖这个文件以反映最新状态。仓库内只保留可公开的占位/示例值，敏感字段（如真实 Klei ID、token）不进仓库。
- `generated/`：管理端运行时生成的 Lua 配置（`dedicated_server_mods_setup.lua`、`modoverrides.lua`），由 `cmd/mod-manager/main.go` 写入。**不进仓库**，已在 `.gitignore` 中忽略。

## 与 `deploy/` 的区别

`deploy/admin-snapshot/server-mods.json` 是从**线上服务器**采集的 MOD 状态快照，用于灾难恢复和审计；不要把它和本目录的种子文件搞混。

| 文件 | 角色 | 是否提交 | 何时使用 |
|---|---|---|---|
| `mods/server-mods.json` | 仓库种子 / 默认初始状态 | 是 | 新部署首次启动 |
| `mods/generated/*.lua` | 运行时产物 | 否 | 由管理端写入运行目录 |
| `deploy/admin-snapshot/server-mods.json` | 线上状态快照 | 是 | 灾难恢复 / 审计参考 |
