---
description: wiki URL classification, AI prompt, workspace write, and inbox rules
applyTo: "wiki/**"
---

# Wiki CLI Rules

## 职责

- `wiki` CLI 负责 URL fetch、AI 分类、摘要生成、写入 wiki workspace。
- `wiki/cmd` 负责 Cobra wiring 和流程入口；核心 fetch/classify/write 逻辑放在 `service/wiki`。
- prompt 位于 `service/wiki/prompts/`，修改时要保持输出结构兼容。

## 副作用

- `wiki --inbox` 会处理并刷新 `wiki/inbox.md`，属于真实 workspace 写入。
- 默认只能使用 dry-run、测试 fixture 或临时目录；真实写入必须任务明确要求。
- `service/wiki.WriteOptions.DryRun` 是优先验证方式。
- 网络 fetch、opencli fallback、AI 调用都属于外部副作用；测试中使用 mock/fake。

## 配置与密钥

- 默认 wiki root、topics URL、并发、超时、重试来自 config/defaultConfig。
- AI key 只从 `OPENAI_API_KEY` 或 `LLM_AxonHub` 等既有 env 获取。
- 不把私密 base URL、token、prompt payload 中的敏感内容写入日志。

## 写入与安全

- 写 wiki 文件前校验路径不逃逸 wiki root。
- 失败分类要写入明确 failure entry 或返回带上下文错误，不静默丢失 URL。
- fetch/classify 并发必须有上限，timeout/retry 使用配置值。
- 不手写 HTML/Markdown/URL parser；使用现有 fetch、readability、goquery、URL helper。

## 验证

- 改写入逻辑跑 `go test ./service/wiki`。
- 改 CLI inbox/URL 流程跑 `go test ./wiki/...`。
- 改 prompt 或分类 schema 时，补 fixture 或 dry-run 验证分类、摘要、失败落盘路径。
