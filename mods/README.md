# 管理端 MOD 状态目录

本目录是 `dst-admin` 管理端（`cmd/mod-manager`）持有的 MOD 状态目录，包含两类内容：

- `server-mods.json`：仓库内的**种子状态**。第一次启动管理端时作为初始 MOD 列表。线上运行后，管理端会原地覆盖这个文件以反映最新状态。仓库内只保留可公开的占位/示例值，敏感字段（如真实 Klei ID、token）不进仓库。
- `generated/`：管理端运行时生成的 Lua 配置（`dedicated_server_mods_setup.lua`、`modoverrides.lua`），由 `cmd/mod-manager/main.go` 写入。**不进仓库**，已在 `.gitignore` 中忽略。

| 文件 | 角色 | 是否提交 |
|---|---|---|
| `mods/server-mods.json` | 仓库种子 / 默认初始状态 | 是 |
| `mods/generated/*.lua` | 运行时产物 | 否 |
