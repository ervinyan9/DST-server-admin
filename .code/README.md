# .code 上下文工程

本目录用于保存面向后续开发的项目上下文、验收标准、知识卡和技能卡。它不是运行时代码目录，也不保存密钥、存档、下载的 Workshop 内容或 Steam 缓存。

## 目录

```text
.code/
  README.md
  context/
    acceptance.md
    project-map.md
  knowledge/
    dst-mod-development/
      README.md
      source-boundaries.md
  skills/
    dst-mod-development.md
```

## 使用方式

1. 做仓库结构、Docker、部署、管理端或 MOD 开发前，先读 `context/project-map.md`。
2. 做阶段性交付前，对照 `context/acceptance.md` 做验收。
3. 做饥荒 MOD 开发或 MOD 整合前，先读 `knowledge/dst-mod-development/README.md` 和 `skills/dst-mod-development.md`。
4. 新增知识只能记录可公开的结论、来源、行为描述和重写策略，不保存第三方受限源码。

## 当前目标

当前阶段目标是把仓库整理成可继续演进的开发工作台：

- 根目录有项目级 `AGENTS.md`。
- `.code/` 有清晰的上下文入口和验收标准。
- 饥荒 MOD 开发知识与技能有独立入口。
- 现有部署资产、管理端和 `dst-waystone` 构建上下文不被移动或破坏。
