---
title: CHANGELOG
description: 记录架构级重要变更
---

## Unreleased

### skx 单 skill + 子命令路由与 schema 校验 [2026-08-16]

[`5ffde58`](https://github.com/xbpk3t/docs-alfred/commit/5ffde5846ede14585da88747d190600e10611c4b)
（关联 [`be74bf1`](https://github.com/xbpk3t/docs-alfred/commit/be74bf1f80e0dd49100a5f9a8db88b7c8e271f61)
[`daf3cda`](https://github.com/xbpk3t/docs-alfred/commit/daf3cdab59b9228cdf861461ad1d52b0deb06faf)
[`7c56615`](https://github.com/xbpk3t/docs-alfred/commit/7c5661579ee9cdadc9b99836bda6cbeaf04a37e4)
[`08c1027`](https://github.com/xbpk3t/docs-alfred/commit/08c10275aaf2f1e99c3021f6989b9082ff6d4d58)
[`900a95b`](https://github.com/xbpk3t/docs-alfred/commit/900a95ba4c641c70f5a504e0e2c70e24375123ea)
[`175376c`](https://github.com/xbpk3t/docs-alfred/commit/175376ccc2430ad26f44d7eeee235223e091c99a)
[`dafa25d`](https://github.com/xbpk3t/docs-alfred/commit/dafa25d613f93abfd1c032fc4de78c9c2aedd335)）

把散落在 `.claude/projects` JSONL 与 Alfred snippets 里的高频手写 prompt 收束为单一 skill
+ 子命令路由，沉淀进 dotfiles 的 Nix 配置。

- 路由与内容解耦：每个 `references/*.md` 写 `abbr` frontmatter，`gen-aliases.nu`
  扫描生成 `aliases.json`，随 Nix rebuild 同步，新增 prompt 只需加文件 + 一行 abbr
- 路由必经处埋点写 `zzz-stats.jsonl` 做频次统计，捕获率近 100%（不依赖 AI 事后记录）
- prompt 间交叉引用用 `[filepath](filepath)` 链接而非 abbr，避免 loop hell；`.xxx.md`
  约定为「不可路由但可被路径引用」
- prpt schema 内嵌并用 JSON Schema 校验依赖；`dafa25d` 移除误加的
  `.worktrees/skx-render` gitlink


### data-cli goods using 提取命令 [2026-08-12]

[`0fef932`](https://github.com/xbpk3t/docs-alfred/commit/0fef9326a26b2dd0ece7e8b6f4e5014dbd1ac992)
（关联 [`2946020`](https://github.com/xbpk3t/docs-alfred/commit/29460205ac52f373fe6233ade99207a9072af6e5)
[`b2c73a0`](https://github.com/xbpk3t/docs-alfred/commit/b2c73a08db0b10950ff5e1e54986daf2f2a020da)）

给 `goods` 实体补 `goods using` 命令，提取 `table[].isUsing == true` 的在用 goods，
作为 AI 做 minst/购买决策的数据基础（`topic.using` 已废弃）。

- 命令形态 `data-cli goods using`（entity-action），输出四层嵌套
  `tag→types→topics→items` 且保留 `tag: goods`，对齐 `dump gh` 便于未来并入 gh
- 真实数据 smoke test 跑出 3 个真实缺陷并修复：纯数字 `name` 被 goccy 类型推断
  成 int 而丢、YAML 1.1 真值 `on/yes/y` 被漏判、`url/source` 误塞 `extra`（RTL 不对称）
- 提取层用 `yaml.Node.String()` 保真而非类型化反推，贯穿「不静默丢数据」原则


### ccx export 文件名改用中文标题 [2026-08-12]

[`b064441`](https://github.com/xbpk3t/docs-alfred/commit/b0644416b8e436a27d83ebdde792ccda13626199)

`ccx session export` 原本用 `gosimple/slug` 把中文标题转成拼音 slug，改为直接用
`title` 做文件名（macOS/APFS 原生支持 Unicode 文件名）。

- 新增 `sanitizeFilename`：替换非法字符 `/\:*?"<>|`、去首尾点防 `.`/`..` 路径逃逸与
  `.foo` 隐藏文件、`strings.Fields` 折叠空白、去控制字符、50 字截断、空回退 `untitled`
- 删除 `EngTitle`/`trimEngTitle`/`slugTitle`/`titleComponentsFromTitle` 链路；ASCII
  标题同样保留原文（含空格），不再转 `-` slug
- 保留 `pkg/textutil.SlugFilename`（仍在 go.mod 依赖）以最小改动原则不碰无关包


### export 用会话名派生标题、移除 fallback 横跳 [2026-08-09]

[`ddc9047`](https://github.com/xbpk3t/docs-alfred/commit/ddc9047c8daff328cce56d99eaf41cba5c85186f)
（关联 [`07876a6`](https://github.com/xbpk3t/docs-alfred/commit/07876a665a499a05afe9817cac3dfdca3a3d0765)（移除 classify/title fallback）
[`db710b3`](https://github.com/xbpk3t/docs-alfred/commit/db710b320a3a4601767616ec5b1aac572aa0a34a)（落 wiki root）
[`cd6ac13`](https://github.com/xbpk3t/docs-alfred/commit/cd6ac13e962c96488e076ebed41ab1c4bf8ef8c5)（从 custom-title 事件解析名））

ccx export 在「AI 分类失败要不要 fallback、fallback 到哪」之间反复横跳。根因是失败
语义没分型（结构/语义/内容/无内容被压进一个函数一个出口）。

- 标题改为从会话名单源派生：cc 取最新 `ai-title`、codex 取 `threads.title`（免费、
  确定性、无 AI 依赖），AI 只负责 `topicPath`，消除「AI 挂掉整次 export 全毁」耦合
- 失败语义分型消除单出口横跳：`session name` 缺失则报错退出；`topic` 失败落 wiki root
- `SessionRef` 加 `Title` 后超 64B，三个函数从值传参改指针传参修 `hugeParam`


### ghcheck 校验 topic.kind [2026-07-24]

[`3a4fd2e`](https://github.com/xbpk3t/docs-alfred/commit/3a4fd2e942c17d80080e9f24790e0a524e8003db)

为 recall 训练体系补门禁：新增 `ghcheck` 包校验 `data/gh` 的 `topic.kind` 合法枚举
（mechanism/type/repo/tools/howto/temp 六值，`unset`/非法/缺失均 error）。

- 支撑 3w3h + mdscc 双切 schema：按 kind 限写骨架（只有 mechanism/type 必写），
  inventory 类（tools/howto/temp）禁止写，规避 YAML 顶层键笛卡尔积爆炸
- 草稿态（标注允许 `unset`）与门禁态（check 不允许 `unset`）分两阶段，不矛盾


### session export 增加 provenance 元数据 [2026-07-24]

[`cc9bda0`](https://github.com/xbpk3t/docs-alfred/commit/cc9bda061051840adbf028f7021cce45181273e5)

`ccx session export` 的 frontmatter 增加可追溯元数据，建立
`issue → agent loop → session → wiki` 可复现索引链路。

- 新增 `session`（必写）、`model`/`issue`（可选 omitempty）、`score`（默认 0 写死，
  不能 omitempty 否则静默丢失）四字段
- `model` 跨 agent 用不同 JSONPath：Claude Code 取 `assistant.message.model`、
  Codex 取 `turn_context.payload.model`，过滤偶发 `<synthetic>`
- 复现目标定为「Index + jump」（跳转索引）而非「时间机器」，故砍掉 cwd/git 等本机
  隐私字段；新字段只登记可选、不进 OKF required，老文件前向兼容


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
