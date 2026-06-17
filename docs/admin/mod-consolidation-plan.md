# DST Mod 合并计划

记录日期：2026-06-17

> 注：文中的 `/opt/dst-server/...` 路径来自旧版宿主机部署，反映采集 MOD 源码时的实际目录。
> 仓库当前目标是 `dst-waystone` 镜像内 `/data/ugc_mods/`、`/data/cluster/Cluster_1/mods/` 等路径；
> 迁移到容器目录是后续独立任务，本文未同步。

本文档记录当前服务器已安装 Mod 的能力审计、重叠关系、风险判断和后续合并计划。目标是参考 Insight 的模块化思路，逐步整合成一个统一的大型管理 Mod，并配套游戏内面板或服务器管理面板。

## 当前结论

生产服务器当前已安装 25 个 Workshop Mod。源码审计确认：这些 Mod 基本都能读取到 Lua 源文件，没有发现 `.luac` 字节码黑盒。

源码主要位于两类目录：

```text
/opt/dst-server/data/ugc/content/322330/<mod_id>
/opt/dst-server/data/DoNotStarveTogether/Cluster_1/mods/workshop-<mod_id>
```

其中 Insight 源码最复杂，约 398 个 Lua 文件，结构接近完整的客户端/服务端信息系统。其他多数 Mod 是较小的 `modmain.lua` 或少量脚本。

需要注意：源码可读不等于可以直接复制后公开发布。Insight 源码头部声明为 shared source，不适合直接再分发。后续合并应以“理解行为后重写”为主。

## 能力分类

### 信息与 UI

| Mod | ID | 当前能力 | 合并判断 |
| --- | --- | --- | --- |
| Insight (Show Me+) | 2189004162 | 鼠标悬停信息、物品/食物/耐久/血量/世界事件信息、Boss 指示器、攻击范围、客户端 UI、RPC 信息同步 | 暂时保留为外部 Mod；新 Mod 参考其架构，不直接复制 |
| Display Food Values | 1410800795 | 在物品栏 tooltip 显示食物饥饿/生命/理智/腐烂时间 | 与 Insight 高度重叠，优先作为停用候选 |
| Global positioning system | 1860955902 | 给玩家添加 `compassbearer`、`maprevealer` 和 `maprevealer` 组件，实现地图定位/揭示相关能力 | 可整合到信息模块或地图模块 |
| Map Revealer for DST | 363112314 | 管理员使用快捷键全图揭示、关闭战争迷雾 | 可整合到管理员工具模块 |
| 怪物击杀公告 / Boss 公告 | 631648169 | Boss 出现、死亡、被攻击、坐标公告 | 可整合到公告中心 |

Insight 已覆盖大量信息显示能力，因此信息类的长期方向应是：保留 Insight 或自研 Insight-lite，而不是同时保留多套 tooltip/信息 UI。

### 服务器管理

| Mod | ID | 当前能力 | 合并判断 |
| --- | --- | --- | --- |
| Restart | 462434129 | `#restart`、`#restart_d`、`#resurrect`、`#kill` 聊天命令，带冷却、管理员豁免、欢迎提示 | 适合整合成玩家命令模块 |
| Less lags | 597417408 | 定期清理地面实体；可配置各 prefab 最大数量；可让鸟/兔子吃更多物品；修改 `herd` 组件清理行为 | 高风险，默认应关闭，作为管理员维护工具 |

`Less lags` 不是单纯优化，它会主动删除实体并改变生物吃物逻辑。后续合并时必须提供明确的白名单、预览、手动执行和日志记录。

### 死亡与复活

| Mod | ID | 当前能力 | 合并判断 |
| --- | --- | --- | --- |
| Don't Drop Everything | 661253977 | 玩家死亡时不全掉落；可控制身体、背包、装备、护符/复活心掉落 | 可整合为死亡惩罚模块 |
| Fire resurrection | 676297854 | 火坑/营火/吸热火可被鬼魂作祟复活；可配置复活后状态和是否生成骷髅 | 被增强版覆盖 |
| Fire resurrection Consist of Functional Medal | 2989465629 | 普通火坑复活的增强版，额外支持勋章火坑、黑曜石火坑等 | 保留其能力，淘汰普通版 |

两个火坑复活 Mod 明显重复。后续只保留增强版能力，并统一配置复活来源、复活惩罚、是否雷击、是否生成骷髅。

### 资源控制

| Mod | ID | 当前能力 | 合并判断 |
| --- | --- | --- | --- |
| World Regrowth | 514758022 | 记录世界初始资源数量，每天循环检查并补生成资源/生物/建筑类 prefab | 高风险，需做成资源再生策略 |
| Multi Rocks | 604761020 | 矿石多次开采、额外掉落石头/硝石/金子/宝石、矿石再生 | 可整合到资源产出模块 |
| Triple Harvest | 3494615834 | 采集、砍树、挖掘、采矿倍率，默认全局 3 倍，并带小幸运掉落 | 可整合到倍率模块 |
| Single Player Health | 1126497610 | 直接修改大量 Boss 和普通生物血量；默认包括非 Boss；可调整远古织影者召唤频率 | 可整合到平衡模块，但需谨慎 |

资源类会显著改变服务器平衡。`World Regrowth` 和 `Less lags` 在策略上冲突：一个补资源，一个删实体。合并后应统一为“资源生命周期策略”，避免两个系统互相打架。

### 操作增强

| Mod | ID | 当前能力 | 合并判断 |
| --- | --- | --- | --- |
| Quick Actions | 700236083 | 缩短烹饪、治疗、收获、建造、采集等动作动画；修改 `wilson` stategraph action handler | 可整合 |
| Quick Pick | 501385076 | 给草、树枝、浆果、仙人掌、荧光果等设置 `quickpick`，可快速收获/烹饪 | 与 Quick Actions 合并 |
| Auto Door | 2074508776 | 门附近有玩家/随从时自动开关门；每 0.2 秒扫描附近实体 | 可整合，但需优化扫描频率 |
| Work Faster | 648064643 | 通过覆盖 `axe`/`pickaxe` prefab，将砍树/挖矿效率调到 15 | 不建议照搬，改为 post-init 实现 |
| Drop & Stack | 1998081438 | 掉落物自动堆叠；支持交易、风滚草、拆包、地皮、腐烂、刮剃等场景 | 可整合，需关注性能 |

操作增强类适合统一成一个“操作”配置页。重点是避免多个 Mod 同时改 stategraph、action handler、component method。

### 物品、耐久与保鲜

| Mod | ID | 当前能力 | 合并判断 |
| --- | --- | --- | --- |
| 无限耐久 | 3484720277 | 可配置大量物品无限耐久、隐藏百分比、容器保鲜倍率、船只血量/无敌 | 可整合，但需拆成多个模块 |
| No Thermal Stone Durability | 466732225 | 移除暖石 `fueled` 耐久消耗 | 可整合 |
| Infinite Tent Uses | 356930882 | 帐篷、凉棚、便携帐篷次数设为极大值 | 可整合 |
| 冰箱永久保鲜，返鲜 | 1898181913 | `TUNING.PERISH_FRIDGE_MULT = -100000`，让冰箱内物品返鲜 | 与无限耐久的保鲜设置冲突 |
| better hammer | 3722867081 | 锤毁建筑返还 100% 材料，烧毁建筑也返还 100% | 可整合 |
| 更好的灭火器 | 2536574008 | 可改灭火器燃料消耗、检测范围、忽略指定火源 | 可整合 |

当前存在一个重要冲突：`冰箱永久保鲜` 和 `无限耐久` 都会写 `TUNING.PERISH_FRIDGE_MULT`。当前加载顺序里 `无限耐久` 后加载，可能覆盖冰箱返鲜效果。合并后必须统一成一个“保鲜策略”，不要让多个模块写同一个 TUNING 值。

## 当前重复与停用候选

优先停用候选：

| 候选 | 原因 |
| --- | --- |
| Display Food Values | Insight 默认已经显示食物、腐烂、物品信息，功能高度重叠 |
| Fire resurrection | 增强版 `2989465629` 是其超集 |
| 冰箱永久保鲜，返鲜 | 与 `无限耐久` 的保鲜配置冲突；应统一重写 |

需要实测后决定：

| 候选 | 原因 |
| --- | --- |
| Work Faster | 当前实现覆盖 prefab，方式较老；需要确认是否真的比 Quick Actions/Quick Pick 多出必要能力 |
| Less lags | 有实际维护价值，但副作用大，不适合默认常开 |
| World Regrowth | 长期服有价值，但会改变世界资源生态，需和清理策略统一 |

## 合并架构建议

参考 Insight 的模块化思路，新 Mod 建议命名为 `DST Admin Suite` 或 `EX Admin Suite`，结构如下：

```text
modmain.lua
modinfo.lua
scripts/
  ex_admin/
    config.lua
    logger.lua
    rpc.lua
    permissions.lua
    modules/
      info.lua
      announcements.lua
      admin_commands.lua
      death_respawn.lua
      operations.lua
      resources.lua
      durability.lua
      freshness.lua
      balance.lua
      maintenance.lua
    ui/
      panel.lua
      tabs/
        server.lua
        resources.lua
        operations.lua
        items.lua
        info.lua
```

模块原则：

- 每个能力必须有独立开关。
- 默认值应保守，不能默认启用高风险清理/再生/倍率功能。
- 所有会删除实体、生成实体、改血量、改掉落倍率的功能都要在日志中记录。
- 服务器端能力和客户端 UI 分离，UI 通过 RPC 请求服务器状态或执行管理员操作。
- 不直接复制 Insight 代码；可以参考其 descriptor/RPC/UI 分层。

## 分阶段计划

### 第一阶段：低风险基础模块

目标：先做一个可运行的大 Mod 框架，并合并最简单、最稳定的功能。

范围：

- `better hammer`
- `Infinite Tent Uses`
- `No Thermal Stone Durability`
- `更好的灭火器`
- 基础保鲜策略

交付：

- `modinfo.lua` 配置项。
- `modmain.lua` 加载模块。
- `durability.lua`、`freshness.lua`、`fire_suppressor.lua`、`refund.lua`。
- 本地测试世界验证无报错。

### 第二阶段：操作增强模块

范围：

- `Quick Pick`
- `Quick Actions`
- `Auto Door`
- `Drop & Stack`
- 重写 `Work Faster`

交付：

- `operations.lua`
- `auto_door.lua`
- `drop_stack.lua`
- 统一动作速度配置。
- 自动门扫描频率和范围可配置。

风险：

- stategraph/action handler 冲突。
- 大量掉落物自动堆叠可能影响性能。
- 自动门周期扫描需要做节流。

### 第三阶段：资源与平衡模块

范围：

- `Triple Harvest`
- `Multi Rocks`
- `World Regrowth`
- `Single Player Health`

交付：

- `resources.lua`
- `balance.lua`
- 资源倍率配置。
- Boss/普通生物血量配置。
- 世界资源再生策略。

风险：

- 改变服务器平衡。
- 资源再生可能与清理策略冲突。
- 生成实体必须避免卡在不可达地形或无效 tile。

### 第四阶段：死亡、复活与管理模块

范围：

- `Restart`
- `Don't Drop Everything`
- 增强版 `Fire resurrection`

交付：

- `admin_commands.lua`
- `death_respawn.lua`
- 聊天命令解析。
- 冷却、管理员豁免、死亡掉落策略。
- 火坑复活来源配置。

风险：

- 玩家生命周期、复活、掉落逻辑容易影响体验。
- 需要确认洞穴/主世界分片行为一致。

### 第五阶段：统一面板

目标：做一个类似 Insight 风格的游戏内管理面板。

范围：

- 客户端 UI。
- 服务器配置读取。
- 管理员权限检查。
- RPC 执行服务器操作。
- 模块开关、倍率、公告、清理策略、资源策略可视化。

注意：

- 普通 server-only Mod 的 `configuration_options` 主要是开服前配置。
- 真正游戏内可点击面板需要客户端 UI + RPC + 服务器端执行。
- 若要动态改配置，需要决定是否写入 shard 存档、mod 配置文件或外部管理服务。

## 建议的近期执行顺序

1. 建立新 Mod 框架，不替换现有服务器 Mod。
2. 合并第一阶段低风险功能，在本地/测试服验证。
3. 生产服先只启用新 Mod 的低风险模块，与旧 Mod 并行对比。
4. 逐个停用被替代的旧 Mod。
5. 再进入操作增强和资源控制模块。
6. 最后做游戏内统一面板。

## 当前已知风险清单

| 风险 | 说明 | 处理建议 |
| --- | --- | --- |
| 许可证风险 | Insight 源码不是自由开源 | 只参考架构，不复制代码 |
| 加载顺序风险 | 多个 Mod 写同一个 `TUNING` 或组件方法 | 合并后统一写入点 |
| 性能风险 | 自动门扫描、掉落堆叠、Less lags 全局遍历实体 | 加节流、范围限制、日志和开关 |
| 平衡风险 | 采集倍率、矿石再生、Boss 血量、死亡不掉落 | 配置默认保守，并明确标识 |
| 存档风险 | 资源再生/实体删除会改变世界状态 | 做备份、先测试、管理员手动确认 |
| 客户端兼容 | 游戏内面板需要客户端安装 Mod | server-only 功能和 client UI 分层 |

## 下一步

建议下一步不是立刻替换生产服，而是先创建一个新的本地 Mod 目录，完成第一阶段框架和低风险模块。待测试稳定后，再逐步把生产服上的重复 Mod 下线。
