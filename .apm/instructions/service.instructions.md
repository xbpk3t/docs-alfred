---
description: Domain layer logic, rendering, network, and validation rules
applyTo: "internal/**"
---

# Domain Layer Rules

- `internal/*` 承载领域服务和渲染逻辑，避免把领域逻辑塞回 Cobra command。
- 领域层可以依赖 `pkg/*`，不能依赖具体 CLI 的 `cmd` package。
- 新领域逻辑优先放入已有领域包；不要为了单个函数新建宽泛包。
- 领域包返回结构化结果和 error，由 CLI 决定输出、exit code、是否发送/写入。
- 网络、AI、文件写入必须有 timeout/context、路径校验和带上下文错误。
- 测试使用临时目录、fixture、fake server 或 mock，不依赖真实 workspace/API。
- 触碰旧代码时，可以修明显低风险问题；不要做无关大重构。

## internal/docs/wiki

- `internal/docs/wiki` 负责 fetch、classify、write 的核心逻辑；`cmd/wiki` 只做流程入口。
- 写 wiki 文件默认使用 dry-run、临时目录或测试 fixture；真实 workspace 写入必须任务明确要求。
- `WriteOptions.DryRun` 是优先验证方式。
- 写入前必须校验目标路径不逃逸 wiki root。
- AI key 只从既有 env/config 获取，不写死 token、base URL 或敏感 payload。
- 修改 `internal/docs/wiki/prompts/` 时，保持分类、摘要、失败落盘结构兼容。
- fetch/classify 并发必须有上限，timeout/retry 使用配置值或 caller-provided context。
- 改写入逻辑跑 `go test ./internal/docs/wiki`。
