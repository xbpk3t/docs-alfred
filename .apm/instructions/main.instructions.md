---
description: docs-alfred global project rules, safety boundaries, and validation policy
---

## 项目结构与分层

### 结构

- `cmd/`：所有 CLI 入口（docs-cli、rss2nl、linear2nl、ccx、gh-alfred、xzb、pwgen、data-cli）。
- `internal/`：按领域分组的内部包。
  - `internal/gh/`：GitHub 数据领域（data、domrules、index、content、goods、task、enrich）。
  - `internal/docs/`：文档/wiki 领域（wiki、ingest、workspace、check）。
  - `internal/rss/`：RSS + 播客领域（feed、transcript）。
  - `internal/linear/`：Linear GraphQL client。
  - `internal/data/`：跨领域数据处理（ops、render）。
- `pkg/`：跨 CLI 复用的基础能力，不依赖具体 CLI。

### 分层约束

- CLI 级业务编排放在 `cmd/<name>/internal/` 或当前 CLI 已有服务层。
- 通用能力放入现有 `pkg/*`；领域能力放入现有 `internal/*`。
- 新文件优先放进现有领域目录，不为了单个函数新建抽象包。
- 触碰到的旧代码如果存在明显、低风险、可测试的问题，可以顺手修；不要为了符合规则做无关大重构。
- 不新增或修改 `.apm/commands`，除非任务明确要求命令工作流。

### 上游关系

- `/Users/luck/Desktop/docs` 是内容、schema、workflow 的 source of truth。
- 本仓库只实现和校验这些规则；不要在本仓库凭空收紧或放宽上游内容 schema。
- 涉及 `data/gh`、`docs-images`、blog、dotfiles 映射规则时，先参考上游 docs 仓库 `.apm` 和现有数据约定。

## 代码质量与依赖

### MUST

- library/package 返回 error，不调用 `os.Exit`、`log.Fatal`；CLI 入口负责 exit code。
- 业务错误要 wrap 上下文，例如 `fmt.Errorf("read config %s: %w", path, err)`。
- 日志使用 `log/slog`；除 CLI 用户输出外，不在 package 中随意 `fmt.Println`。

### MUST NOT

- 不吞掉网络、AI、邮件、文件写入失败；允许降级时也要保留原因。
- 不打印 token、cookie、邮箱认证信息、完整敏感 payload。
- 不手写 YAML/RSS/HTML/Markdown/URL parser，除非先说明现有工具无法覆盖。



### 依赖与工具

优先使用标准库和 `go.mod` 已有依赖。新增第三方依赖前说明理由。不引入大框架替代现有约定。

#### 框架与基础设施

- **Cobra**：CLI 框架，所有 `cmd/` 入口的 command 注册与 flag 解析。
- **koanf**：配置加载，支持 YAML/env/flag 多源合并。
- **goccy/go-yaml** + **gopkg.in/yaml.v3**：YAML 解析与 AST 操作，`pkg/parser` 和 `pkg/yamlutil` 基于此。
- **resty**：HTTP client，`pkg/httputil` 封装了 retry + backoff。
- **log/slog**：结构化日志标准库，全项目统一使用，不在 package 中随意 `fmt.Println`。

#### code-style

- **lo**（samber/lo）：泛型工具函数（map/filter/contains）
- mo:

#### 内容处理

- **gofeed**：RSS/Atom 解析，`internal/rss/feed` 和 `internal/rss/transcript` 基于此。
- **goquery**：HTML/DOM 查询与提取，wiki fetch、readability 均依赖。
- **goldmark**：Markdown 解析与渲染，`pkg/md` 基于此。
- **adfg/frontmatter**：Markdown frontmatter 解析，wiki write/check 使用。
- **html-to-markdown**：HTML 转 Markdown，`pkg/md/converter.go` 使用。
- **purell**：URL 标准化，`pkg/urlutil` 和 rss2nl hunt 使用。
- **go-astisub**：字幕解析，`internal/rss/transcript` 使用。
- **html/template**：模板渲染，newsletter `.gohtml`、linear2nl 报告均依赖。默认 escaping；`template.HTML` 只用于已知安全内容。

#### AI 与外部服务

- **langchaingo**：AI/LLM 调用抽象，`pkg/ai` 封装，供 linear2nl、rss2nl transcript summary、wiki classify、ccx export 使用。
- **genqlient**：GraphQL codegen，`internal/linear` 的 Linear API client 基于此。
- **go-github**：GitHub API client，`internal/docs/wiki/fetch.go` 使用。
- **go-git**：Git 操作，`pkg/gitutil` 封装，wiki fetch 和 ingest 使用。
- **resend**：邮件发送，rss2nl newsletter 和 linear2nl 报告使用。
- **cloudflare-go**：Cloudflare D1/Workers API，xzb sync 和 wiki fetch 使用。

#### 数据与工具

- **excelize**：Excel 解析，xzb 的微信/支付宝账单导入使用。
- **carbon**：日期时间处理，ccx、docs-cli、rss2nl、xzb 使用。
- **testify**：测试断言与 mock，59 个测试文件统一使用。
- **errgroup**：并发错误收集，rss2nl send、linear2nl evening、rss feed、docs ingest 使用。


## 安全与副作用

### MUST / MUST NOT

- 默认允许运行 `check`、`dry-run`、mock、局部测试。
- 真实发送邮件、真实 Linear/API/AI/付费搜索、大规模网络抓取、真实写 wiki workspace，必须任务明确要求。
- 会写 repo-tracked 内容的命令只有任务明确要求才执行，例如 `data render`、`images --apply`。
- 修改 GitHub Actions schedule、secret 名、release 配置或任何会触发真实发送/发布的 workflow 时，必须明确说明影响。




### 配置与生成物

- 配置默认值集中在 config loader 或已有 config package，不在业务代码里散落新默认值。
- env var 读取集中在配置层；业务函数优先接收显式 config struct。
- 修改 YAML/config schema 时保持向后兼容，除非任务明确要求 breaking change。
- 不提交临时 HTML/report/cache、真实本地配置、AI/API 输出缓存或个人运行产物，例如 `.cache/**`、`newsletter_*.html`、`linear2nl_*.html`。
- 不随意改 `go.sum`；只有新增/移除依赖或合理的 `go mod tidy` 结果才保留。

### 网络、文件与输出

- 外部请求必须有 timeout 或 caller-provided context。
- 不开 unbounded goroutine；并发量必须有明确上限或来自配置。
- retry 使用已有 retry/client 能力，不写散落的 sleep loop。
- 测试默认使用 fake server、fixture、mock，不依赖真实第三方服务。
- 写敏感或个人产物时使用私有权限，例如 `0600` 或 `fileutil.FilePermPrivate`。
- 文件写入前明确目标路径，避免 path traversal；优先复用 `fileutil`。
- 默认依赖 `html/template` escaping；只有已知安全 HTML 才能使用 `template.HTML`，并在代码旁保留简短安全说明。
- `pkg/wf` 的 `alfred`、`plain`、`raw`、`rofi` 输出可能被 Alfred/Rofi/脚本消费，不随意改字段名、排序、分隔符、JSON shape 或默认输出格式。

## 验证

### MUST

- 使用 `testify` 写测试用例
- 小改动优先跑相关 package 的 `go test`
- 触碰 `pkg/`、`internal/` 或跨 CLI 行为时，优先跑 `go test ./...`
- 如果没有运行应跑的验证，在总结里说明原因和剩余风险。

### 测试与 lint

- `task test` 是全量测试入口。
- `task lint` 会执行 `golangci-lint run --fix`、`gofumpt -w`、`go mod tidy`、`pre-commit`、`nilaway`，不要把它当只读检查。
- `pre-commit run --all-files`
