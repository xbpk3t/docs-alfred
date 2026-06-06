---
description: Service layer domain logic, rendering, network, and validation rules
applyTo: "service/**"
---

# Service Layer Rules

- `service/*` 承载领域服务和渲染逻辑，避免把领域逻辑塞回 Cobra command。
- 服务层可以依赖 `pkg/*`，不能依赖具体 CLI 的 `cmd` package。
- 新领域逻辑优先放入已有 service 包；不要为了单个函数新建宽泛 service。
- service 返回结构化结果和 error，由 CLI 决定输出、exit code、是否发送/写入。
- 网络、AI、文件写入必须有 timeout/context、路径校验和带上下文错误。
- 测试使用临时目录、fixture、fake server 或 mock，不依赖真实 workspace/API。
- 触碰旧 service 时，可以修明显低风险问题；不要做无关大重构。
