---
description: CLI layer rules for all commands, side-effect boundaries, and per-CLI constraints
applyTo: "cmd/**"
---

## 验证

- 改 command/flag 跑 `go test ./cmd/<name>/...`。
- 改 CLI internal 跑 `go test ./cmd/<name>/internal/...`。
- 跨 CLI 行为跑 `go test ./...`。

---

## docs-cli

子命令：`images`、`dotfiles`、`wiki`。

- 上游 `$HOME/Desktop/docs` 是 source of truth。不凭空修改 YAML schema、topic 目录规则、record 规则或 image mapping。
- `images --apply` 会修复/创建内容；默认只跑 check/list。
- `wiki --inbox` 会处理并刷新 inbox，属于真实 workspace 写入。

## data-cli

子命令：`render`、`check`、`dedup`。

- 遵循上游 docs 项目 schema 规则，不凭空修改。
- `render` 写 repo-tracked 内容，只有任务明确要求才执行。

## rss2nl

子命令：`send`、`trns`（含 `check` 子命令）、`hunt`。

- `send` 默认会发送 newsletter；agent 只能使用 `--check` 或显式 dry-run。
- `hunt --send-mail` 会发邮件；默认不运行。
- Exa、Tavily、ASR、AI summary、temporary upload 是外部/付费副作用；默认使用 mock 或关闭开关。
- `.cache/rss2nl/**` 是运行产物，不作为业务 source of truth。
- RSS 解析使用 `internal/rss/feed`（gofeed），不手写 parser。
- 单个 feed 失败要进入 failure report，不静默丢失。

## linear2nl

子命令：`morning`、`evening`、`export`。

- 默认会调用 Linear/AI 并发送邮件；agent 必须使用 `--dry-run` 或 mock。
- 不新增会修改 Linear issue/state/comment 的行为，除非任务明确要求。
- 保持 CST/UTC+8 报告语义；不改成本机隐式时区。
- prompt 在 `cmd/linear2nl/internal/prompts/*.txt`，修改时保持输出结构兼容。
- AI 调用失败应降级或返回带上下文错误，不导致空邮件静默发送。

## ccx

子命令：`session`（含 `chain`、`export`）。

遵循通用规则，无额外约束。

## gh-alfred

子命令：`search`、`sync`、`export`、`validate`。

遵循通用规则，无额外约束。

## pwgen

单命令，无子命令。

遵循通用规则，无额外约束。

## xzb

子命令：`sync`、`export`、`sql`、`d1`。

遵循通用规则，无额外约束。
