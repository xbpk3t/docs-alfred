---
title: CHANGELOG
description: 记录架构级重要变更
---

## v3.0.0 [2026-06-24]

### 补全测试覆盖 [2026-06-23]

[`20b4edc`](https://github.com/xbpk3t/docs-alfred/commit/20b4edc7d70b736d92bea8aec7102947bbd3410b)

项目之前几乎没有测试，核心逻辑完全靠人工验证。随着连续两个大 refactor（pkg/md 管道
+ 目录重组）改了大量代码，没有测试兜底意味着每次改动都在裸奔。

- 可测试性改造：`pwgen` 的 HMAC/JS 函数提升为包级变量，`litter` 的 base URL 改为
  `var` + struct 覆盖，`linear` 新增 `NewClientWithHTTP()`
- 提取 CSV 列名、driver 名等 magic string 为常量
- 新增 `go build` pre-commit hook 和 `test-coverprofile` 任务
- `goconst` linter 配置调整：`min-occurrences` 4→6，`ignore-tests: true`



### 项目目录重组为 cmd/ + internal/ 标准布局 [2026-06-22]

[`6c04e25`](https://github.com/xbpk3t/docs-alfred/commit/6c04e2565a68c5866e73c3f2e3fb3a56eac51b01)

8 个二进制目录占满顶层 `ls` 输出，`service/` 和 `internal/` 职责边界模糊。这次把所有
二进制移到 `cmd/`，`service/` 和 `internal/` 合并按业务域分组，符合 Go 社区标准布局。

- `cmd/` 放所有二进制 main.go（ccx/data-cli/docs-cli/gh-alfred/linear2nl/pwgen/rss2nl/xzb）
- `internal/` 按域分：gh/docs/rss/linear/data
- 消除 `service/service.go`（只有一个 `ServiceType` 枚举）
- `go install` 路径和 CI workflow 同步更新


### 用 pkg/md 管道替代 gohtml 模板 [2026-06-21]

[`4d30323`](https://github.com/xbpk3t/docs-alfred/commit/4d3032385d2cea656562e31af4e88ee8a9642d04)

5 个 gohtml 模板的主要工作量是 CSS（~650 行），数据流是 AI JSON → Go 预渲染 HTML →
模板管道输出。`html/template` 的自动转义已被 `//nolint:gosec` 全面绕过，模板价值有限。

- 新建 `pkg/md`：Section 接口驱动的组件管道 `data → go-pretty(md) → goldmark → HTML`
- 消除全部 `html/template` 依赖、`template.HTML` 类型和 5 个重复 goldmark 实例
- 接受视觉退化（卡片变文本、彩色标签变纯文本），换来零 CSS 维护成本
- 5 个模板删除（1142 行），新增 `pkg/md` 5 文件（604 行），净减 ~950 行


### AI 输出从 markdown 改为结构化 JSON [2026-06-13]

[`bfdc136`](https://github.com/xbpk3t/docs-alfred/commit/bfdc1365676318f2a19bc1f4d7aeb1d702cf673f)

AI 返回自由格式的 markdown，解析脆弱、无法结构化处理。改为返回结构化 JSON，是后续
pkg/md 组件化的前提。

- evening 按 issue 分 review（progress/knowledge/review），模板加卡片布局 + 暗色模式
- morning 按优先级分组（FIXME/MAYBE/REMOVE），AI 返回 reason/impact/action
- evening 查询用 errgroup 并发执行
- 提取 `UnmarshalStrictJSON` 到 `pkg/ai`（关联：`9c38f9e`、`5890f7c`）


### 用库替换手写解析 [2026-06-06]

[`cec3f83`](https://github.com/xbpk3t/docs-alfred/commit/cec3f8364b608934006c8ac6e1f8d338105d8d74)

HTML 解析用正则、URL 提取用字符串分割、HTTP retry 手写 backoff——维护成本高、边界
case 多。统一换成成熟库，净减 800 行。

- goquery（HTML）、xurls（URL）、astisub（字幕）、purell（URL 规范化）
- resty（HTTP retry）、go-readability（文章提取）



### 项目重组为多 CLI 布局 [2026-06-03]

[`b30edf3`](https://github.com/xbpk3t/docs-alfred/commit/b30edf355658e531b1df093485e8677eacdb85c8)

从"一个二进制包所有命令"拆成 `docs-cli`、`data-cli`、`gh-alfred`、`rss2nl`、
`linear2nl` 等独立二进制，各自有自己的 `cmd/` 和 `main.go`。


### 从 TS 版迁入 Go 实现 [2026-06-03]

[`218bc18`](https://github.com/xbpk3t/docs-alfred/commit/218bc18ba5e76d47ba7bb34bd5f2fc76daa80033)

之前很多 CLI 命令是 TODO stub。这次从 docs 仓库的 TypeScript 版一次性迁入完整实现，
是整个 Go 版本可用的起点。

- 实现 data、blog、dotfiles、images、gh 命令
- rss2nl 的 hunt/trns/wiki 和 AI client
- 共享验证包（checkutil、data、gh、images、dotfiles、blog）


---

## v2.0.0 [2025-10-15]

### alfred 文件夹替换为 workflow

[`a98a08d`](https://github.com/xbpk3t/docs-alfred/commit/a98a08d6829014a43d36ef3c919360a5e68f33b)

`alfred/gh/` 和 `alfred/pwgen/` 是独立 Go module，维护两套 go.mod/go.sum。合并进主
module，同时迁移到 `.workflow/` 结构保证不同 launcher 下的迁移性。


### 早期项目结构化 [2024-12]

`e999b71` + `0448e9e` + `9134f06` + `c44af76`

项目从"一个大 main.go"走向有结构。

- alfred 代码独立到 `alfred/` 文件夹
- parser/merger 抽象（几乎全量重写）
- error code 统一，移除所有 `fmt.Errorf()`
- `pkg/` 和 `service/` 分层
