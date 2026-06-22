---
description: rss2nl newsletter, transcript, source hunt, template, and side-effect rules
applyTo: "cmd/rss2nl/**"
---

# rss2nl Rules

## 命令结构

- `rss2nl` 使用 Cobra，入口在 `cmd/rss2nl/cmd/root.go`。
- 当前子命令：`send`、`trns`、`trns check`、`hunt`。
- 保持默认 config path、flag 名和输出路径兼容，除非任务明确要求迁移。

## 副作用

- `rss2nl send` 默认会发送 newsletter；agent 默认只能使用 `--check`、渲染测试或显式 dry-run 路径。
- `hunt --send-mail` 会发邮件；默认不运行。
- Exa、Tavily、ASR、AI summary、temporary upload 都是外部/付费/网络副作用；默认使用 mock、fixture 或关闭相关开关。
- `.cache/rss2nl/**` 是运行产物/cache，不作为业务 source of truth。

## 配置与密钥

- `RESEND_TOKEN`、AI key、provider key 只能来自 env 或配置，不写死到代码、测试、模板。
- `cmd/rss2nl/rss2newsletter.yml` 可作为本地/示例配置参考；不要提交真实 token 或私密收件人变更。
- 配置默认值集中在 `internal/rss/feed/config.go` 或已有 config loader，不在调用点散落。

## RSS、网络、模板

- RSS 解析使用 `gofeed` 和 `pkg/rss`，不要手写 RSS/XML 解析。
- 网络请求必须有 timeout/context，并保留 feed 失败原因。
- 单个 feed 失败时要进入 failure report 或 category failure，不要静默丢失。
- `.gohtml` 使用 `html/template`；`template.HTML` 只能用于已知安全内容，并保留安全边界说明。
- 修改 newsletter、hunt、trns 模板后，至少跑对应渲染测试或 dry-run 渲染路径。

## 验证

- 改 `internal/rss/feed` 跑 `go test ./internal/rss/feed`。
- 改 `cmd/rss2nl/cmd` 跑 `go test ./cmd/rss2nl/cmd`。
- 改 transcript pipeline 跑 `go test ./cmd/rss2nl/...`，并使用 fixture/mock 避免真实网络和 ASR。
