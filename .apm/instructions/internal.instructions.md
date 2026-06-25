---
description: Domain layer rules for wiki fetch/classify/write constraints
applyTo: "internal/**"
---

## internal/docs/wiki

- `internal/docs/wiki` 负责 fetch、classify、write 的核心逻辑；`cmd/docs-cli` 的 wiki 子命令只做流程入口。
- 写 wiki 文件默认使用 dry-run、临时目录或测试 fixture；真实 workspace 写入必须任务明确要求。
- `WriteOptions.DryRun` 是优先验证方式。
- 写入前必须校验目标路径不逃逸 wiki root。
- AI key 只从既有 env/config 获取，不写死 token、base URL 或敏感 payload。
- 修改 `internal/docs/wiki/prompts/` 时，保持分类、摘要、失败落盘结构兼容。
- fetch/classify 并发必须有上限，timeout/retry 使用配置值或 caller-provided context。
