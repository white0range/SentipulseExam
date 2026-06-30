# Sentipulse Plugin Execution System

一个面向文本处理场景的插件化执行系统，使用 Go 构建。主程序负责插件发现、依赖校验、执行编排、故障隔离与结果汇总；业务逻辑由独立插件进程实现，并通过 `stdin/stdout + JSON` 协议与主程序通信。

该仓库定位为一个可扩展的工程化工具，而不是某个固定业务流程的硬编码实现。

## Overview

- 语言与版本：Go `1.20+`
- 架构风格：`主程序 + 插件子进程 + JSON 协议`
- 运行模式：`list` / `run` / `enable` / `disable` / `watch`
- 第三方依赖：无，全部基于标准库实现

## Core Capabilities

- 插件目录发现与 manifest 校验
- 插件启用 / 禁用管理
- 插件元信息展示：名称、版本、状态、运行时、标签、依赖、失败策略
- 进程级执行隔离
- 超时控制
- 依赖关系管理与拓扑执行
- 版本约束校验
- 失败降级与自动禁用
- 热重载监控
- 多语言插件扩展能力

## Quick Start

查看插件：

```bash
go run . -mode list
```

执行已启用插件：

```bash
go run . -mode run -input examples/input.json
```

并发执行：

```bash
go run . -mode run -parallel -input examples/input.json
```

启用 / 禁用插件：

```bash
go run . -mode enable -plugin keyword-tagger
go run . -mode disable -plugin keyword-tagger
```

监控插件目录变化：

```bash
go run . -mode watch -watch-interval 2s
```

变化后自动重新执行输入：

```bash
go run . -mode watch -input examples/input.json -run-on-change
```

## Repository Layout

```text
.
├─ docs/
│  └─ architecture.md         架构图说明
├─ examples/                  示例输入
├─ internal/
│  ├─ app/                    CLI 入口与 watch 模式
│  ├─ config/                 参数解析
│  ├─ core/                   manager 与执行编排
│  ├─ executor/               插件进程执行器
│  ├─ loader/                 插件扫描与 manifest 校验
│  ├─ model/                  领域模型
│  ├─ registry/               插件注册表
│  └─ version/                版本约束匹配
├─ pkg/
│  ├─ protocol/               主程序 / 插件共享协议
│  └─ sdk/                    Go 插件辅助 SDK
├─ plugins/
│  ├─ insight-summary/        汇总型插件
│  ├─ keyword-tagger/         关键词标注插件
│  └─ wordcount/              文本统计插件
├─ main.go                    程序入口
└─ target.md                  原始需求文档
```

## Architecture

系统采用子进程插件模型，而不是 in-process 动态库模型：

- 主程序不直接引用具体插件实现
- 每个插件运行在独立进程中
- 插件只需要遵守协议即可，不绑定具体语言
- 依赖关系和执行顺序由主程序统一控制

详细图示见 [docs/architecture.md](docs/architecture.md)。

## Default Processing Chain

默认启用链路：

`wordcount -> insight-summary`

可选增强链路：

`keyword-tagger -> insight-summary`

说明：

- `wordcount` 负责文本统计
- `insight-summary` 基于上游结果生成结构化摘要
- `keyword-tagger` 默认禁用，启用后可为摘要提供额外关键词命中信息

## Plugin Contract

每个插件目录至少包含：

- `plugin.json`
- 一个可执行入口，例如 `main.go`

示例 manifest：

```json
{
  "name": "insight-summary",
  "version": "1.0.0",
  "description": "Builds a human-readable summary from upstream plugin results.",
  "runtime": "go",
  "tags": ["summary", "dependency-demo"],
  "enabled": true,
  "command": ["go", "run", "."],
  "work_dir": ".",
  "timeout_ms": 15000,
  "dependencies": [
    {
      "name": "wordcount",
      "version": ">=1.0.0"
    },
    {
      "name": "keyword-tagger",
      "version": ">=1.0.0",
      "optional": true,
      "require_success": false
    }
  ],
  "on_dependency_failure": "skip"
}
```

关键字段：

- `command`：插件启动命令
- `dependencies`：依赖插件及版本约束
- `optional`：依赖是否可选
- `require_success`：是否必须依赖成功
- `on_failure`：当前插件失败后的处理策略
- `on_dependency_failure`：依赖异常后的处理策略

## Execution Model

执行流程：

1. 扫描插件目录并读取 manifest。
2. 校验名称、版本、命令、工作目录等基础信息。
3. 校验依赖存在性、版本约束与循环依赖。
4. 将插件标记为 `enabled`、`disabled`、`blocked` 或 `error`。
5. 为已启用插件建立拓扑执行计划。
6. 按层串行或并发执行插件。
7. 将上游结果透传给依赖型插件。
8. 汇总执行结果，并按策略处理失败场景。

## Status Model

插件状态：

- `enabled`
- `disabled`
- `blocked`
- `error`

执行状态：

- `success`
- `failed`
- `timeout`
- `skipped`

## Capability Matrix

| 能力 | 状态 | 说明 |
| --- | --- | --- |
| 插件目录加载 | 已实现 | 扫描 `plugins/` 并读取 `plugin.json` |
| 插件启用 / 禁用 | 已实现 | 通过 CLI 修改 manifest |
| 插件元信息管理 | 已实现 | 名称、版本、状态、运行时、标签、依赖、策略 |
| 插件异常隔离 | 已实现 | 子进程执行，失败不影响主程序 |
| 插件超时控制 | 已实现 | `context.WithTimeout` |
| 执行结果汇总 | 已实现 | 结构化 JSON 输出 |
| 热加载 / 热卸载 | 已实现 | `watch` 模式监控目录变化 |
| 依赖关系管理 | 已实现 | 依赖声明、拓扑排序、循环依赖检测 |
| 版本约束 | 已实现 | 支持 `>=`, `<=`, `>`, `<`, `=` |
| 失败降级 | 已实现 | `skip` / `continue` / 自动禁用 |
| 多语言插件支持 | 已实现 | `command` 可运行任意可执行程序 |

## Verification

建议验证命令：

```bash
go test ./...
go run . -mode list
go run . -mode run -input examples/input.json
```

测试覆盖：

- 插件加载与启停
- 成功 / 失败 / 超时执行
- 依赖执行链路
- 依赖失败后的跳过策略
- 自动禁用策略
- 版本约束匹配
- watch 模式差异检测辅助逻辑

## Design Tradeoffs

- 选择子进程插件而不是 in-process 插件，以提升隔离性和跨语言扩展能力。
- 示例源码插件默认采用串行执行，以降低首次冷启动时的并发编译抖动。
- 热重载目前采用轮询式 `watch`，实现简单、跨平台稳定，后续可升级为事件驱动。
- 版本约束实现保持轻量，不引入外部依赖。

## Extension Directions

- 插件签名校验与可信来源控制
- 进程资源限制或容器级隔离
- 重试策略与结果缓存
- 文件系统事件驱动的实时热重载
- 执行指标与可观测性输出
