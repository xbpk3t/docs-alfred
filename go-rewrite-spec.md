# SPEC: Go CLI 重构迁移（docs-alfred -> docs）

> 作者: luck
>
> 状态: 草案 / 待实现
>
> 关联: LUC-65
>
> 重要说明: 本文完整替代上一版 `go-rewrite-spec.md`。不要把本文当作旧草案的增量 patch；旧草案中的“行为完全等价迁移”“保留旧 `module:action` 命令”“独立 dgh binary”等前提均已废弃。

---

## 1. Executive Summary

本 spec 目标是把 `docs` 项目中 `packages/cli/*` 的 TypeScript CLI 退役，改为消费独立 Go 仓库 `/Users/luck/Desktop/docs-alfred` 提供的命令行工具。

这不是 TypeScript 到 Go 的逐字等价迁移，而是一次**破坏性 CLI 重构迁移**：

- 旧命令形态如 `docs-cli data:check gh`、`docs-cli gh:find`、`rss2nl trns:check` 不保留 alias。
- 新命令使用 Cobra 风格的空格分隔子命令。
- `docs` repo 最终删除 `packages/cli/*`。
- `docs` repo 本地 Taskfile 只检查 Go binaries 是否存在于 `PATH`，不负责安装。
- GitHub Actions 在 CI 中通过 `go install ...@main` 安装所需 Go binaries。
- `docs` repo 只移除 CLI 相关 Node/pnpm 依赖；web/blog/deploy 仍可继续使用 Node/pnpm。

最终 `docs-alfred` 提供这些 binaries：

| Binary | 来源仓库 | 职责 |
|--------|----------|------|
| `docs-cli` | `docs-alfred` | 数据渲染、数据校验、docs-images/dotfiles/blog 检查、GitHub repo 查询 |
| `rss2nl` | `docs-alfred` | RSS newsletter、转写、source discovery、wiki inbox 处理 |
| `pwgen` | `docs-alfred` | 密码生成器 |

独立 `dgh` binary 删除，其功能合入 `docs-cli gh search/sync`。

---

## 2. Repository Boundary

### 2.1 `docs-alfred` repo

`/Users/luck/Desktop/docs-alfred` 是 Go CLI 的实现仓库。它负责：

- 实现和维护 `docs-cli`、`rss2nl`、`pwgen`。
- 把现有 `dgh` 搜索/同步逻辑合入 `docs-cli gh search/sync`。
- 移除独立 `dgh` command/binary。
- 把 `rss2newsletter` 命令调整为 `rss2nl`，使 `go install github.com/xbpk3t/docs-alfred/rss2nl@main` 产物名为 `rss2nl`。
- 维护 Go tests、README、Taskfile 和可安装命令目录。

`docs-alfred` 不需要为 `docs` repo 提供 vendored 代码，也不需要把 Go 源码复制进 `docs` repo。

### 2.2 `docs` repo

`/Users/luck/Desktop/docs` 是工具消费仓库。它负责：

- 直接调用 `PATH` 中的 `docs-cli`、`rss2nl`、`pwgen`。
- 在 Taskfile 中通过 `command -v` 做 precondition；缺 binary 时给出明确错误。
- 在 CI workflow 中通过 `go install ...@main` 安装 Go binaries。
- 删除 `packages/cli/*` 和 CLI-only workspace 依赖。
- 更新 Taskfile、pre-commit、GitHub Actions、README/AGENTS/APM instructions 中的旧 CLI 命令引用。

`docs` repo 本地 Taskfile 不自动安装 CLI。原因：两边调用都走 Taskfile，本地缺工具时直接失败即可，避免 Taskfile 暗中改变开发环境。

---

## 3. Final CLI Contract

### 3.1 `docs-cli data`

数据层命令使用 `docs-cli data <domain> <action>`。这里 `data` 是模块，`domain` 是数据域，`action` 是动作。

#### `docs-cli data render`

渲染 YAML 数据产物。

```bash
docs-cli data render --config .github/workspace/docs.yml
docs-cli data render --config .github/workspace/docs.yml --extract topics --out /tmp/gh.json
```

参数：

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `-c, --config <path>` | 否 | `docs.yml` | render 配置文件 |
| `--extract <type>` | 否 | 无 | 仅支持 `topics` |
| `--out <path>` | `--extract` 时必填 | 无 | extract 输出路径 |

行为要求：

- 保持现有 render 配置格式兼容。
- 支持 `cmd: dgh/goods/task` 等现有处理器。
- `--extract topics` 从 `data/rendered/gh.json` 提取 `{tag,type,topics}` backbone。
- 输出文件格式和现有 TypeScript 版保持结构兼容，但不要求 stdout 字符串逐字一致。

#### `docs-cli data <domain> check`

校验数据域。

```bash
docs-cli data books check --path data/books --scope books
docs-cli data music check --path data/music --scope music
docs-cli data diary check --path data/diary --scope diary
docs-cli data gh check
docs-cli data goods check
docs-cli data task check
docs-cli data ntl check
```

支持域：

| Domain | 默认 path | 默认 scope | 旧命令来源 |
|--------|-----------|------------|------------|
| `books` | `data/books` | `books` | `data:check books` |
| `movie` | `data/books` | `movie` | `data:check movie` |
| `tv` | `data/books` | `tv` | `data:check tv` |
| `music` | `data/music` | `music` | `data:check music` |
| `diary` | `data/diary` | `diary` | `data:check diary` |
| `gh` | `data/gh` | auto | old `data:check gh` + old `gh:check` |
| `goods` | `data/goods` | auto | `data:check goods` |
| `task` | `data` | auto | `data:check task` |
| `ntl` | `data/.archive/z/ntl` | `ntl` | `data:check ntl` |

参数：

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--path <path>` | 否 | domain 决定 | 覆盖数据目录 |
| `--scope <scope>` | 否 | domain 决定 | structured data 校验 scope |

`data gh check` 是合并校验，必须覆盖两条旧链路：

1. 旧 `data:check gh` 的 metadata / image-dir 期望校验。
2. 旧 `gh:check` 的 YAML walker、entry、record 校验。

`data gh check` 至少检查：

- `data/gh/**/*.yml|yaml` 可解析。
- YAML multi-document 正确处理。
- 根节点为数组。
- section `type` 与文件名一致。
- section `record` 存在且为数组。
- repo/using entry 的 `url` 存在且是合法 URL。
- repo/using entry 若有 `record`，必须是数组。
- repo topics 中的 `record[].date` 是 `YYYY-MM-DD`。
- repo topics 中的 `record[].des` 存在且非空。
- `meta.slug`、`meta.hasPic`、nested `sub`、repo topics 派生出的 docs-images 期望路径能被校验。

#### `docs-cli data <domain> duplicate`

检测重复数据。

```bash
docs-cli data books duplicate --path data/books
docs-cli data music duplicate --path data/music
docs-cli data gh duplicate
```

支持域：`books`、`music`、`gh`。

参数：

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--path <path>` | 否 | domain 决定 | 覆盖数据目录 |

`gh` 去重按 URL 作为唯一 identity。

#### `docs-cli data gh find`

搜索本地 `data/gh` 条目，保留旧 `gh:find` 的本地文件定位能力。

```bash
docs-cli data gh find kubernetes
docs-cli data gh find --query kubernetes
docs-cli data gh find --url https://github.com/org/repo
```

参数：

| 参数 | 必填 | 说明 |
|------|------|------|
| `[query]` | 与 `--query`/`--url` 至少一个 | 搜索关键词 |
| `-q, --query <query>` | 同上 | 搜索关键词 |
| `--url <url>` | 否 | 按 URL 搜索 |
| `--limit <n>` | 否 | 输出上限，默认 20 |

搜索范围：repo URL、repo name、`des`、`zk`、topic name。输出必须包含 `data/gh` 文件位置，方便人工修改数据。

#### `docs-cli data gh append-record`

给本地 `data/gh` 条目追加 record。

```bash
docs-cli data gh append-record --url https://github.com/org/repo --date 2026-06-03 --des "note"
docs-cli data gh append-record --file data/gh/infra/devops.yml --url https://github.com/org/repo --date 2026-06-03 --des "note"
```

参数：

| 参数 | 必填 | 说明 |
|------|------|------|
| `--url <url>` | 是 | 目标 repo URL |
| `--date <date>` | 是 | `YYYY-MM-DD` |
| `--des <text>` | 是 | record 描述 |
| `--file <path>` | 否 | 直接指定目标 YAML 文件 |
| `--topic <name>` | 否 | 指定 topic 名；不指定则从 URL 最后路径段推断 |

行为要求：

- 未指定 `--file` 时遍历 `data/gh` 找唯一 URL 匹配。
- 多个匹配时报错；无匹配时报错。
- 优先追加到匹配 topic 的 `record`；否则追加到 section level `record`。
- 使用 `yq` v4（mikefarah/yq）做 YAML mutation，不手写 YAML writer。
- 写入前检查 date 格式。
- 写入后重新解析 YAML，确认文件合法。
- 写入后输出 `git diff --stat <file>` 摘要。

### 3.2 `docs-cli gh`

`docs-cli gh` 只负责远程/渲染后的 GitHub repo 查询，不负责本地 `data/gh` 文件编辑。本地数据操作放在 `docs-cli data gh ...`。

#### `docs-cli gh search`

合入原 Go `dgh` 和 TS `gh-alfred` 的能力。

```bash
docs-cli gh search
docs-cli gh search kubernetes
docs-cli gh search kubernetes --output plain
docs-cli gh search kubernetes --output alfred
```

参数：

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `[query]` | 否 | 空字符串 | 查询关键词；为空时返回全部 |
| `-o, --output <fmt>` | 否 | `plain` | `alfred`、`plain`、`raw`、`rofi` |
| `--url <url>` | 否 | `https://docs.lucc.dev/gh.yml` | 远程 gh.yml URL |
| `--cache <path>` | 否 | `/tmp/docs-cli-gh.yml` | 本地缓存路径 |
| `--max-age <duration>` | 否 | `24h` | 缓存 TTL |
| `--docs-url <url>` | 否 | `https://docs.lucc.dev` | docs URL base，用于 Alfred/plain 输出 doc link |

缓存策略：

- 默认远程 URL: `https://docs.lucc.dev/gh.yml`。
- 默认缓存路径: `/tmp/docs-cli-gh.yml`。
- 默认 TTL: 24h。
- `search` 发现缓存不存在或超过 TTL 时自动 fetch 并刷新缓存。
- fetch 失败但缓存存在时，允许使用过期缓存并输出 warning。
- fetch 失败且缓存不存在时，命令失败。

输出格式：

- `plain`: 人类可读输出，包含 repo URL、desc、type/tag、doc link。
- `alfred`: Alfred Script Filter JSON。
- `raw`: 原始结构化 JSON。
- `rofi`: rofi 可消费文本输出。

#### `docs-cli gh sync`

强制刷新远程 gh.yml 缓存。

```bash
docs-cli gh sync
docs-cli gh sync --url https://docs.lucc.dev/gh.yml --cache /tmp/docs-cli-gh.yml
```

参数：

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--url <url>` | 否 | `https://docs.lucc.dev/gh.yml` | 远程 gh.yml URL |
| `--cache <path>` | 否 | `/tmp/docs-cli-gh.yml` | 缓存写入路径 |

### 3.3 `docs-cli images/dotfiles/blog`

这些命令不属于 data domain，使用各自 scope。

```bash
docs-cli images check --data-dir data/gh --images-dir docs-images --skip-missing --skip-extra-files
docs-cli dotfiles sync-plan --dotfiles dotfiles --json
docs-cli dotfiles check --dotfiles dotfiles
docs-cli blog check --data-dir data/gh --blog-dir blog
```

`images check` 必须精确复刻现有行为：

- 从 `data/gh` 读取期望 docs-images 路径。
- 支持 `meta.slug` 作为目录名。
- 支持 `meta.hasPic` 判断是否期望有图。
- 支持 nested topic `sub`。
- 支持 repo-level topics 派生路径。
- `--skip-missing` 不因 missing expected 失败。
- `--skip-extra-files` 忽略 extra files。
- `--apply` 保留当前副作用：处理 duplicate files、隐藏 extra dirs、移动 extra files 到 `.temp`。

### 3.4 `rss2nl`

`rss2nl` 必须显式子命令。无子命令时显示 help/warning，exit non-zero 或 zero 均可，但不得发送 newsletter。

#### `rss2nl send`

```bash
rss2nl send --config .github/workspace/rss2nl.yml
rss2nl send --config .github/workspace/rss2nl.yml --check
```

参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-c, --config <path>` | `rss2nl.yml` | 配置文件 |
| `--check` | false | 只检查 feed 健康度，不发邮件 |
| `--trns-out <dir>` | `.cache/rss2nl/trns` | 转写缓存目录 |

#### `rss2nl trns`

```bash
rss2nl trns podcast --config .github/workspace/rss2nl.yml --limit 1
rss2nl trns check podcast --config .github/workspace/rss2nl.yml --limit 1 --strict
```

参数保持现有 TS 行为：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `source` | `podcast` | 当前仅支持 `podcast` |
| `--out <dir>` | `.cache/rss2nl/trns` | 缓存输出目录 |
| `--limit <n>` | config 决定 | 每 feed 条目上限 |
| `--asr` | config 决定 | 启用 ASR fallback |
| `--language <lang>` | config 决定 | ASR 语言 |
| `--publish` / `--no-publish` | config 决定 | 临时上传开关 |
| `--refresh` | false | 忽略已有缓存 |
| `--strict` | false | 有失败时 non-zero |

#### `rss2nl hunt`

```bash
rss2nl hunt --config .github/workspace/rss2nl.yml --state .github/workspace/feeds-hunt-state.json --new-only
```

必须保留 Exa/Tavily provider、report md/html/json、state、blocked-domain、new-only、dry-run、send-mail 等现有行为。

#### `rss2nl wiki`

```bash
rss2nl wiki https://example.com/article --config .github/workspace/rss2nl.yml
rss2nl wiki --inbox --config .github/workspace/rss2nl.yml
```

行为要求：

- 移除 Notion integration 和 `--from-notion`。
- 保留本地 `--inbox`，读取 wiki root 下的 `inbox.md`，处理成功后按现有安全规则 flush。
- `rss2nl wiki` 无 URL 且未传 `--inbox` 时输出 warning，exit 0，不写入文件。
- AI topic/type 分类逻辑保留。
- 写入路径必须做 path traversal 防护。
- pending/failed_recorded/written 的安全 flush 语义保持。

### 3.5 `pwgen`

`pwgen` 保留 Go 现有功能。TS 无对应实现。

---

## 4. Migration Matrix

### 4.1 `docs-cli`

| 旧命令 | 新命令 | 说明 |
|--------|--------|------|
| `docs-cli data:render` | `docs-cli data render` | 破坏性改名 |
| `docs-cli data:render --extract topics` | `docs-cli data render --extract topics` | 行为保留 |
| `docs-cli data:check books` | `docs-cli data books check` | domain-first |
| `docs-cli data:check music` | `docs-cli data music check` | domain-first |
| `docs-cli data:check diary` | `docs-cli data diary check` | domain-first |
| `docs-cli data:check goods` | `docs-cli data goods check` | domain-first |
| `docs-cli data:check task` | `docs-cli data task check` | domain-first |
| `docs-cli data:check ntl` | `docs-cli data ntl check` | domain-first |
| `docs-cli data:check gh` | `docs-cli data gh check` | 与旧 `gh:check` 合并 |
| `docs-cli gh:check` | `docs-cli data gh check` | 合并进本地 data/gh 校验 |
| `docs-cli data:duplicate books` | `docs-cli data books duplicate` | domain-first |
| `docs-cli data:duplicate music` | `docs-cli data music duplicate` | domain-first |
| `docs-cli data:duplicate gh` | `docs-cli data gh duplicate` | domain-first |
| `docs-cli gh:find` | `docs-cli data gh find` | 本地 data/gh 搜索 |
| `docs-cli gh:append-record` | `docs-cli data gh append-record` | 本地 data/gh 写入 |
| `docs-cli images:check` | `docs-cli images check` | space command |
| `docs-cli dotfiles:sync-plan` | `docs-cli dotfiles sync-plan` | space command |
| `docs-cli dotfiles:check` | `docs-cli dotfiles check` | space command |
| `docs-cli blog:check` | `docs-cli blog check` | space command |
| `dgh [query]` | `docs-cli gh search [query]` | 独立 dgh 删除 |
| `dgh sync` | `docs-cli gh sync` | 独立 dgh 删除 |

旧命令 alias 不保留。

### 4.2 `rss2nl`

| 旧命令 | 新命令 | 说明 |
|--------|--------|------|
| `rss2nl --config ...` | `rss2nl send --config ...` | root 不再默认 send |
| `rss2nl --check --config ...` | `rss2nl send --check --config ...` | 显式 send |
| `rss2nl send` | `rss2nl send` | 保留 |
| `rss2nl trns podcast` | `rss2nl trns podcast` | 保留 |
| `rss2nl trns:check podcast` | `rss2nl trns check podcast` | space command |
| `rss2nl hunt` | `rss2nl hunt` | 保留 |
| `rss2nl wiki --inbox` | `rss2nl wiki --inbox` | 保留 |
| `rss2nl wiki --from-notion` | 删除 | Notion 移除 |

---

## 5. Required Changes in `docs-alfred`

### 5.1 Command layout and install names

`go install` 默认 binary 名来自 command 目录名。目标是让 README 中的安装命令直接产出目标 binary：

```bash
go install github.com/xbpk3t/docs-alfred/docs-cli@main
go install github.com/xbpk3t/docs-alfred/rss2nl@main
go install github.com/xbpk3t/docs-alfred/pwgen@main
```

因此 `docs-alfred` 需要调整 command layout，使以上路径存在且 main package 产物名正确。

要求：

- `docs-cli` command 包含原 `docs/` 数据 render 能力、待迁移的 TS docs-cli 能力、原 `dgh` 的 search/sync 能力。
- `rss2newsletter` command 调整为 `rss2nl` command。
- `dgh` 独立 command 删除；可保留内部共享 package，但不再产出 binary。
- root README 更新为新的 `go install` 命令。
- root Taskfile 修正当前旧路径问题，例如不应继续引用不存在的 `alfred/dgh`。

### 5.2 Shared libraries

建议将共享逻辑拆到稳定 package：

- YAML multi-doc walker。
- structured data validation。
- gh image-dir expectation collector。
- data render pipeline。
- shell command wrapper (`git`, `yq`)。
- gh remote cache/search formatter。
- rss config/parser/render/transcript/hunt/wiki pipeline。
- AI client。

不要为了“抽象统一”把所有模块塞进一个巨大 package；按命令域和复用点拆分即可。

### 5.3 YAML handling

要求：

- 读取 `data/gh` 时必须支持 YAML multi-document。
- 写入 `data/gh` record 时必须调用 `yq` v4，不用 Go YAML writer 直接改文件。
- `yq --version` 必须验证是 mikefarah/yq v4，而不是 Python `yq`。
- 外部命令必须使用 argv 数组执行，不拼 shell string。

### 5.4 AI client

Go AI client 必须处理 DeepSeek 非标准字段 `reasoning_content`。

响应解析优先级：

1. `choices[0].message.content`
2. `choices[0].message.reasoning_content`
3. 两者都为空时报 unavailable/error

配置来源保持现有语义：

- `OPENAI_API_KEY`
- `OPENAI_BASE_URL`
- `LLM_AxonHub`
- rss2nl config 中的 model/baseUrl

不实现 streaming、tool use、structured object generation，除非后续另开 spec。

---

## 6. Required Changes in `docs`

### 6.1 Taskfile

更新所有直接调用旧 CLI 的 Taskfile：

- `.github/taskfile/Taskfile.data.yml`
- `.github/taskfile/Taskfile.docs-images.yml`
- `.github/taskfile/Taskfile.feeds.yml`
- `.github/taskfile/Taskfile.wiki.yml`

要求：

- `DOCS_CLI` 从 `pnpm exec docs-cli` 改为 `docs-cli`。
- `rss2nl` 从 `pnpm --filter @docs/cli-rss2nl cli -- ...` 改为直接 `rss2nl ...`。
- 面向用户的入口 task 保留 `desc` 和 `summary`。
- 在相关 tasks 的 `preconditions` 中加入 `command -v docs-cli` 或 `command -v rss2nl`。
- precondition 只检查，不安装。

示例：

```yaml
preconditions:
  - sh: command -v docs-cli >/dev/null
    msg: "Missing docs-cli. Install it from github.com/xbpk3t/docs-alfred/docs-cli@main and ensure it is on PATH."
```

### 6.2 GitHub Actions

CI 需要安装 Go binaries。按用户决策，直接使用 `@main`。

需要更新的 workflow 至少包括：

- `.github/workflows/linters.yml`
- `.github/workflows/feeds.yml`

示例：

```yaml
- uses: actions/setup-go@v5

- name: Install Go CLIs
  run: |
    go install github.com/xbpk3t/docs-alfred/docs-cli@main
    go install github.com/xbpk3t/docs-alfred/rss2nl@main
```

feeds workflow 中当前用 Node 解析 JSON candidate count 的小脚本可以保留，也可以改为 Go/`jq`。本 spec 不要求彻底去 Node，因为范围是 CLI-only 去 Node。

### 6.3 Pre-commit

`.pre-commit-config.yaml` 中 `books-data-check` 仍可调用 `task data:check`，但该 task 内部必须已改为 Go `docs-cli`。

CI pre-commit job 必须在运行 pre-commit 前安装 `docs-cli`，否则 hook 会失败。

### 6.4 Package cleanup

完成 Go CLI 迁移并验证后：

- 删除 `packages/cli/docs-cli`。
- 删除 `packages/cli/rss2nl`。
- 删除 `packages/cli/gh-alfred`。
- 从 `pnpm-workspace.yaml` 移除 `packages/cli/*`。
- 从 root `package.json` 移除 CLI-only scripts/deps，例如 `@docs/docs-cli` 和 `scripts.docs-cli`。
- 保留 web/blog/deploy 仍需要的 Node/pnpm 配置。
- 更新 `.github/instructions/docs-cli.instructions.md`、`.github/instructions/data.instructions.md`、AGENTS/APM instructions 中的旧 TS CLI 描述。

---

## 7. Validation Plan

### 7.1 `docs-alfred` validation

必须新增或更新 Go tests：

| Area | Required tests |
|------|----------------|
| YAML walker | multi-doc、parse-error、not-array、section、repo、using |
| `data gh check` | 合并旧 metadata 校验和旧 entry/record 校验 |
| structured data | books/movie/tv/music/diary/ntl scope 校验 |
| duplicate | books/music/gh duplicate 检测 |
| data render | `docs.yml` render、`--extract topics` |
| append-record | section-level append、topic-level append、invalid date、missing URL、multiple URL |
| images check | `meta.slug`、`meta.hasPic`、nested `sub`、repo topics、`--apply` |
| gh search/sync | cache miss、cache hit、expired cache、fetch failure with stale cache、output formats |
| rss2nl root | 无子命令不 send |
| rss2nl wiki | `--inbox`、无 URL warning exit 0、path traversal 防护 |
| AI client | content、reasoning_content、empty response |

Minimum commands:

```bash
cd /Users/luck/Desktop/docs-alfred
go test ./...
go install github.com/xbpk3t/docs-alfred/docs-cli@main
go install github.com/xbpk3t/docs-alfred/rss2nl@main
go install github.com/xbpk3t/docs-alfred/pwgen@main
```

### 7.2 `docs` validation

After installing Go binaries on PATH:

```bash
cd /Users/luck/Desktop/docs
task data
task y2m:check
task docs-images:ci-check
task feeds:trns -- --limit 1
```

For wiki inbox workflow, run the existing wiki inbox task after updating its command to `rss2nl wiki --inbox`.

For mutating command fixture validation, use temporary test data rather than real `data/gh` unless the user explicitly wants to append a real record.

### 7.3 CI validation

Expected CI behavior:

- `linters.yml` installs `docs-cli` before pre-commit.
- `feeds.yml` installs `rss2nl` before hunt/send.
- No CI workflow invokes `pnpm --filter @docs/cli-rss2nl`.
- No CI workflow invokes `pnpm exec docs-cli`.
- CLI package deletion does not break web/blog/deploy flows.

---

## 8. Non-goals

- 不把 Go 源码放进 `docs` repo。
- 不让 `docs` Taskfile 自动安装 local Go binaries。
- 不保留旧 `module:action` command aliases。
- 不保留独立 `dgh` binary。
- 不移除 web/blog/deploy 所需 Node/pnpm。
- 不迁移 Docusaurus/blog 架构。
- 不改 `data/` schema。
- 不改 `docs-images/` 目录规范。
- 不实现 AI streaming/tool use。
- 不保留 Notion integration。

---

## 9. Implementation Order

推荐顺序如下，避免中间状态不可验证：

1. 在 `docs-alfred` 中建立目标 command layout：`docs-cli`、`rss2nl`、`pwgen`。
2. 把 `dgh` search/sync 迁入 `docs-cli gh search/sync`，删除独立 `dgh` binary。
3. 实现 `docs-cli data render` 和 `data <domain> check/duplicate`。
4. 实现 `docs-cli data gh find/append-record/check`。
5. 实现 `docs-cli images/dotfiles/blog` commands。
6. 重构 `rss2newsletter` 为 `rss2nl`，完成 `send/trns/trns check/hunt/wiki`。
7. 更新 `docs-alfred` README、Taskfile、tests。
8. 在 `docs` repo Taskfile 中替换 CLI 调用并加入 preconditions。
9. 更新 GitHub Actions，使用 `go install ...@main`。
10. 跑 `docs` validation。
11. 删除 `packages/cli/*` 和 CLI-only pnpm workspace 配置。
12. 更新 repo instructions/docs，移除旧 TS CLI 描述。

---

## 10. Acceptance Criteria

迁移完成必须满足：

- `docs-alfred` 可通过 `go install github.com/xbpk3t/docs-alfred/docs-cli@main` 安装 `docs-cli`。
- `docs-alfred` 可通过 `go install github.com/xbpk3t/docs-alfred/rss2nl@main` 安装 `rss2nl`。
- `docs-cli data gh check` 覆盖旧 `data:check gh` 和旧 `gh:check` 的校验语义。
- `docs-cli gh search --output alfred` 可替代旧 gh-alfred/dgh 搜索输出。
- `rss2nl send` 替代旧 root send 行为；root 无子命令不发送。
- `rss2nl wiki --inbox` 保留本地 inbox workflow。
- `rss2nl wiki` 无 URL warning + exit 0。
- `docs` repo 中不再有 `packages/cli/*`。
- `docs` repo Taskfile 不再调用 `pnpm exec docs-cli`。
- `docs` repo Taskfile 不再调用 `pnpm --filter @docs/cli-rss2nl`。
- `task y2m:check` 通过，或所有失败均为迁移前已知数据问题且被记录。
- GitHub Actions 能在 CI 中安装 Go CLIs 并运行相关 workflows。
