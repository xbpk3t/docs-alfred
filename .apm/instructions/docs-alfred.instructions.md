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
- 不新增或修改 `.apm/commands`，除非任务明确要求命令工作流。

## 上游关系

- `/Users/luck/Desktop/docs` 是内容、schema、workflow 的 source of truth。
- 本仓库只实现和校验这些规则；不要在本仓库凭空收紧或放宽上游内容 schema。
- 涉及 `data/gh`、`docs-images`、blog、dotfiles 映射规则时，先参考上游 docs 仓库 `.apm` 和现有数据约定。

## 副作用边界

- 默认允许运行 `check`、`dry-run`、mock、局部测试。
- 真实发送邮件、真实 Linear/API/AI/付费搜索、大规模网络抓取、真实写 wiki workspace，必须任务明确要求。
- 会写 repo-tracked 内容的命令只有任务明确要求才执行，例如 `append-record`、`data render`、`images --apply`。
- 修改 GitHub Actions schedule、secret 名、release 配置或任何会触发真实发送/发布的 workflow 时，必须明确说明影响。
- 不自动 commit/push。
- 不写死 token、API key、cookie、邮箱凭证、私密 URL 或真实收件人变更。

## 配置与生成物

- 配置默认值集中在 config loader 或已有 config package，不在业务代码里散落新默认值。
- env var 读取集中在配置层；业务函数优先接收显式 config struct。
- 修改 YAML/config schema 时保持向后兼容，除非任务明确要求 breaking change。
- 不提交临时 HTML/report/cache、真实本地配置、AI/API 输出缓存或个人运行产物，例如 `.cache/**`、`newsletter_*.html`、`linear2nl_*.html`。
- 不随意改 `go.sum`；只有新增/移除依赖或合理的 `go mod tidy` 结果才保留。

## 验证策略

- 小改动优先跑相关 package 的 `go test`。
- 触碰 `pkg/`、`service/` 或跨 CLI 行为时，优先跑 `go test ./...`。
- `task test` 是全量测试入口。
- `task lint` 会执行 `golangci-lint run --fix`、`gofumpt -w`、`go mod tidy`、`pre-commit`、`nilaway`，不要把它当只读检查。
- `pre-commit run --all-files` 也可能自动修改文件；运行前确认任务允许自动修复。
- 如果没有运行应跑的验证，在总结里说明原因和剩余风险。

## 依赖策略

- 优先使用标准库和 `go.mod` 已有依赖。
- 新增第三方依赖前说明：现有依赖/标准库为什么不够、该依赖是否成熟维护、引入成本是否合理。
- 不引入大框架替代现有 Cobra、koanf、goccy/go-yaml、gofeed、goquery、goldmark、resty/http helper、conc pool 等约定。

## Go 质量规则

- library/package 返回 error，不调用 `os.Exit`、`log.Fatal`；CLI 入口负责 exit code。
- 业务错误要 wrap 上下文，例如 `fmt.Errorf("read config %s: %w", path, err)`。
- 日志使用 `log/slog`；除 CLI 用户输出外，不在 package 中随意 `fmt.Println`。
- 不吞掉网络、AI、邮件、文件写入失败；允许降级时也要保留原因。
- 不打印 token、cookie、邮箱认证信息、完整敏感 payload。

## 现有能力优先

- YAML 使用现有 `goccy/go-yaml`、`koanf`、`pkg/parser` 或已有配置 loader。
- RSS 使用 `mmcdole/gofeed` 和 `pkg/rss`。
- HTML/DOM 使用 `goquery`、readability 或已有 fetch/extract helper。
- Markdown/HTML 渲染使用 `goldmark`、`html/template` 和已有模板。
- HTTP 使用 `pkg/httputil` 或项目既有 client 风格。
- 并发使用已有 bounded pool、`context`、`errgroup`/`conc` 模式。
- 不手写 YAML/RSS/HTML/Markdown/URL parser，除非先说明现有工具无法覆盖。

## 网络、文件与输出

- 外部请求必须有 timeout 或 caller-provided context。
- 不开 unbounded goroutine；并发量必须有明确上限或来自配置。
- retry 使用已有 retry/client 能力，不写散落的 sleep loop。
- 测试默认使用 fake server、fixture、mock，不依赖真实第三方服务。
- 写敏感或个人产物时使用私有权限，例如 `0600` 或 `fileutil.FilePermPrivate`。
- 文件写入前明确目标路径，避免 path traversal；优先复用 `fileutil`。
- 默认依赖 `html/template` escaping；只有已知安全 HTML 才能使用 `template.HTML`，并在代码旁保留简短安全说明。
- `pkg/wf` 的 `alfred`、`plain`、`raw`、`rofi` 输出可能被 Alfred/Rofi/脚本消费，不随意改字段名、排序、分隔符、JSON shape 或默认输出格式。
