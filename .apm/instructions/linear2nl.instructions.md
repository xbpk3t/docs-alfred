---
description: linear2nl report generation, dry-run, config, template, and API rules
applyTo: "cmd/linear2nl/**"
---

# linear2nl Rules

## 命令结构

- `linear2nl` 使用 Cobra，入口在 `cmd/linear2nl/cmd/root.go`。
- 当前子命令：`morning`、`evening`。
- 保持 `--config/-c` 默认路径 `cmd/linear2nl/linear2nl.yml` 和 `--dry-run` 语义兼容。

## 副作用

- `morning` 和 `evening` 默认会调用 Linear/AI 并发送邮件。
- agent 默认必须使用 `--dry-run`、mock 或测试；真实发送邮件必须任务明确要求。
- 不新增会修改 Linear issue/state/comment 的行为，除非任务明确要求。

## 配置与时间

- Linear key 使用 `linear.apiKey` 或 `LINEAR_API_KEY`。
- Resend token 使用 `resend.token` 或 `RESEND_TOKEN`。
- AI 配置走已有 `cmd/linear2nl/internal` 和 `pkg/ai` env fallback，不写死 key/base URL。
- 保持当前 CST/UTC+8 报告语义；不要改成本机隐式时区。
- 修改配置 schema 时保持向后兼容，除非任务明确要求 breaking change。

## 模板与 AI

- prompt 位于 `cmd/linear2nl/internal/prompts/`，template 位于 `cmd/linear2nl/cmd/templates/`。
- prompt 输出变化必须兼容 HTML 渲染和 fallback 行为。
- `template.HTML` 只用于已知安全或已转换内容；保留简短安全说明。
- AI 调用失败应降级或返回带上下文错误，不要导致空邮件静默发送。

## 验证

- 改 config loader 跑 `go test ./cmd/linear2nl/internal`。
- 改 Linear client 跑 `go test ./internal/linear`，测试用 fake server/mock。
- 改 command/template/prompt 后，跑相关 package 测试；需要人工验证时使用 `--dry-run`。
