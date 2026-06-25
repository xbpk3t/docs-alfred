---
description: docs-alfred global project rules, safety boundaries, and validation policy
applyTo: "**"
---

> 本文件由 APM 管理，是 `.claude/rules/` 和 per-CLI `CLAUDE.md` 的 source of truth。修改规则请编辑本文件，不要手动修改生成产物。

## 项目结构

- `cmd/`：CLI 入口（docs-cli、data-cli、rss2nl、linear2nl、ccx、gh-alfred、xzb、pwgen）。只做 Cobra wiring、flag 解析、用户输出、exit code。CLI 级编排放 `cmd/<name>/internal/`。
- `internal/`：按领域分组的共享包（gh、docs、rss、linear、data）。领域能力放这里。
- `pkg/`：跨 CLI 复用的基础能力，不依赖领域概念。
- 新文件优先放进现有领域目录，不为单个函数新建抽象包。

## 代码质量

> 通用 Go 规则（错误 wrap、并发安全、测试模式等）由 linter + skills 自动覆盖，不在此重复。以下只列项目特有约定。

### HTTP Client

- 必须：HTTP 调用走 `pkg/httputil`（resty 封装 + retry + backoff）
- 禁止：裸用 `http.Get` / `http.DefaultClient`
- 约定：库需要 `*http.Client` 时用 `httputil.StdHTTPClient()`

### 配置加载

- 必须：用 `configutil.LoadYAMLConfig[T]`（泛型加载器）
- 约定：默认值用 `defaults.MustSet`，校验用 `validator.Struct`
- 约定：config struct 用 `yaml` tag；环境变量覆盖声明在 `EnvOverride` 表
- 约定：config 加载错误用 `errors.As` 拿 `*configutil.LoadError`，按 `Stage` 分段处理
- 约定：修改 config schema 保持向后兼容，除非任务要求 breaking change

### 错误处理

- 约定：业务错误用 `fmt.Errorf("...: %w", err)` wrap 上下文
- 约定：区分永久失败和可重试错误（参考 `feed.FeedError` 的 `Transient` 字段、`ingest` 的 `fetchFailureError` vs `classifyRetryError`）
- 禁止：吞掉网络、AI、邮件、文件写入失败；允许降级时保留原因

### 并发

- 必须：并发用 `errgroup` + `SetLimit`（永远有界）
- 禁止：裸 goroutine（无生命周期管理）
- 约定：重试用 `avast/retry-go`，不手写 retry 循环

### 日志

- 约定：日志用 `log/slog`，不在 package 中随意 `fmt.Println`
- 约定：复用日志 key 常量（参考 `internal/rss/feed/constants.go` 的 `LogKeyURL` 等），避免内联字符串

### CLI 入口

- 约定：`Execute()` 入口固定顺序：`carboninit.Setup()` → `validator.Setup()` → Cobra → `slog.Error` + `os.Exit(1)`
- 约定：flag 定义用局部匿名 struct，`RunE` 处理错误（不用 `Run`）
- 约定：外部命令调用走 `pkg/cmdutil`，不裸用 `os/exec`

### 测试

- 约定：用 `testify`（assert/require）+ `httptest.NewServer`
- 约定：mock 用 `go:generate mockgen`（`go.uber.org/mock`）
- 约定：fixture 文件放 `testdata/` 目录
- 禁止：不依赖真实第三方服务

### 文件操作

- 约定：原子写入用 `pkg/fileutil.AtomicWriteFile`
- 约定：文件权限用 `fileutil` 常量（`FilePermPrivate 0600` 等）
- 约定：缓存路径用 `fileutil.CachePath`（XDG 规范）
- 禁止：path traversal

### 输出格式

- 约定：多格式输出走 `pkg/wf.Formatter`（Alfred/Rofi/Plain）
- 禁止：不随意改 `pkg/wf` 的字段名、排序、JSON shape

### AI 调用

- 约定：AI 调用走 `pkg/ai.ChatContext`，不直接调 langchaingo
- 约定：AI 配置从 env 获取（`OPENAI_API_KEY` / `OPENAI_BASE_URL`），不写死

### 依赖约定

- 约定：函数式辅助用 `samber/lo`（Map/Filter/Uniq 等），不手写
- 约定：安全指针转换用 `samber/mo`
- 约定：接口实现有编译时断言：`var _ Interface = (*Impl)(nil)`
- git 操作：`go-git` → `pkg/gitutil`
- 邮件发送：`resend` → 直接用 `resend.NewClient()` + `client.Emails.SendWithContext()`

## 领域能力

### 解析

- 禁止：手写 YAML/RSS/HTML/Markdown/URL parser，除非说明现有工具无法覆盖
- YAML 解析：`goccy/go-yaml` → `pkg/parser`, `pkg/yamlutil`
- RSS/Atom 解析：`gofeed` → `internal/rss/feed`
- HTML/DOM 查询：`goquery`
- Markdown 渲染：`goldmark` → `pkg/md`
- Markdown frontmatter：`adfg/frontmatter` → `internal/docs/wiki`
- URL 规范化：`purell` → `pkg/urlutil`
- HTML→Markdown：`html-to-markdown` → `pkg/md`
- 字幕解析：`go-astisub` → `internal/rss/transcript`

### 渲染

- 约定：输出渲染用 `pkg/md` builder 模式（`NewDocument()` + `NamedSection`/`Paragraph`/`Link` 等）+ `doc.ToHTML()`，不用 Go 模板引擎
- 约定：AI prompt 模板用 `text/template` + `template.ParseFS()` + `Option("missingkey=error")`

## 安全与副作用

- 默认允许 `check`、`dry-run`、mock、局部测试。
- 真实发送邮件、真实 Linear/API/AI/付费搜索、大规模抓取、真实写 workspace，必须任务明确要求。
- 写 repo-tracked 内容的命令（如 `data render`、`images --apply`）只有任务明确要求才执行。
- 修改 Actions schedule、secret 名、release 配置时必须说明影响。

## 验证

- 小改动跑相关 package 的 `go test`；跨 CLI 跑 `go test ./...`。
- 未运行应跑的验证，在总结里说明原因和剩余风险。
- `task test`：全量测试。`task lint` 会自动修改文件；运行前确认任务允许。
