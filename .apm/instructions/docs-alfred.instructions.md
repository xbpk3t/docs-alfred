---
description: docs-alfred global project rules, safety boundaries, and validation policy
---

# docs-alfred 全局规则

本仓库是 `github.com/xbpk3t/docs-alfred`，用于维护个人 docs 工作流相关的 Go CLI 和共享服务。它不是内容仓库本体。

## 项目结构

- `docs-cli/`：docs 内容、catalog、workspace consistency 的 CLI。
- `rss2nl/`：RSS newsletter、transcript、source hunt 工具。
- `linear2nl/`：Linear morning/evening 邮件报告工具。
- `wiki/`：URL 分类、摘要、写入 wiki workspace 的 CLI。
- `pwgen/`：确定性密码生成工具。
- `pkg/`：跨 CLI 复用的基础能力，不依赖具体 CLI。
- `service/`：领域服务与渲染逻辑。

## 分层约束

- `cmd/` 只做 Cobra command 注册、flag/arg 解析、用户可见输出和 exit code。
- CLI 级业务编排放在 `internal/usecase` 或当前 CLI 已有服务层。
- 通用能力放入现有 `pkg/*`；领域能力放入现有 `service/*`。
- 新文件优先放进现有领域目录，不为了单个函数新建抽象包。
- 触碰到的旧代码如果存在明显、低风险、可测试的问题，可以顺手修；不要为了符合规则做无关大重构。

## 上游关系

- `/Users/luck/Desktop/docs` 是内容、schema、workflow 的 source of truth。
- 本仓库只实现和校验这些规则；不要在本仓库凭空收紧或放宽上游内容 schema。
- 涉及 `data/gh`、`docs-images`、blog、dotfiles 映射规则时，先参考上游 docs 仓库 `.apm` 和现有数据约定。

## 副作用边界

- 默认允许运行 `check`、`dry-run`、mock、局部测试。
- 真实发送邮件、真实 Linear/API/AI/付费搜索、大规模网络抓取、真实写 wiki workspace，必须任务明确要求。
- 会写 repo-tracked 内容的命令只有任务明确要求才执行，例如 `append-record`、`data render`、`images --apply`。
- 不自动 commit/push。
- 不写死 token、API key、cookie、邮箱凭证、私密 URL 或真实收件人变更。

## 验证策略

- 小改动优先跑相关 package 的 `go test`。
- 触碰 `pkg/`、`service/` 或跨 CLI 行为时，优先跑 `go test ./...`。
- `task test` 是全量测试入口。
- `task lint` 会执行 `golangci-lint run --fix`、`gofumpt -w`、`go mod tidy`、`pre-commit`、`nilaway`，不要把它当只读检查。
- `pre-commit run --all-files` 也可能自动修改文件；运行前确认任务允许自动修复。

## 依赖策略

- 优先使用标准库和 `go.mod` 已有依赖。
- 新增第三方依赖前说明：现有依赖/标准库为什么不够、该依赖是否成熟维护、引入成本是否合理。
- 不引入大框架替代现有 Cobra、koanf、goccy/go-yaml、gofeed、goquery、goldmark、resty/http helper、conc pool 等约定。
