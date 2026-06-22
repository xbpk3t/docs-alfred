---
description: docs-cli Cobra command, usecase, and docs workflow compatibility rules
applyTo: "cmd/docs-cli/**"
---

# docs-cli Rules

## 命令结构

- `docs-cli` 使用 Cobra，入口在 `cmd/docs-cli/cmd/root.go`。
- 保持当前命令形状：`data`、`catalog`、`workspace`。
- `gh` 是 `catalog` 的 deprecated alias；除非任务明确要求 breaking change，不要移除兼容入口。
- `workspace` 下聚合 `images`、`dotfiles`、`blog`；不要恢复旧的 split command，除非任务要求兼容。

## 分层

- `cmd/docs-cli/cmd` 只做 command/flag/arg wiring 和用户输出。
- CLI 编排放在 `cmd/docs-cli/internal/usecase`。
- 可复用校验、解析、渲染逻辑放入已有 `internal/gh/domrules`、`internal/gh/data`、`internal/docs/workspace/images`、`internal/docs/workspace/dotfiles`、`internal/docs/workspace/blog`。
- 调整上游 docs 内容规则时，不要只改 command；必须同步底层 package 和测试。

## 数据与兼容

- `/Users/luck/Desktop/docs` 是 `data/gh`、docs-images、blog、dotfiles 映射规则的 source of truth。
- 不在本仓库凭空修改 YAML schema、topic 目录规则、record 规则或 image mapping。
- `data gh append-record` 会写 YAML；只有任务明确要求写数据时才运行或扩展。
- `images --apply` 会修复/创建内容；默认只跑 check/list 类行为。
- 输出格式如果走 `pkg/wf`，必须保持 Alfred/plain/raw/rofi 兼容。

## 验证

- 改 `cmd/docs-cli/cmd` 路由、flag、默认值时，补或运行 command/usecase 相关测试。
- 改 `internal/gh/domrules`、`internal/gh/index`、`internal/data/render` 时，跑对应 package 测试。
- 改 `internal/gh/data` walker/find/append/check 时，跑 `go test ./internal/gh/data ./internal/data/ops/...`。
- 跨 workspace consistency 行为时，跑相关 `internal/docs/workspace/images`、`internal/docs/workspace/dotfiles`、`internal/docs/workspace/blog` 测试。
