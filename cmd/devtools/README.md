# devtools-alfred

开发者工具箱 Alfred Workflow（**链式**多级 Script Filter）。

## 交互

1. `dd` → 列出 tool（目前 `base64`）
2. 选中 tool → **进入下一级**（不再显示 `dd base64`）→ 只有 `encode` / `decode`
3. 选中 action → 输入内容 → 结果 ⏎ **复制**（不自动粘贴；transient，不进剪贴板历史）

状态用 **Arg and Vars** 传 `tool` / `action`，不靠 keyword 行堆路径。

## Alfred 链式约定（改 plist 必读）

```text
SF-TOOLS (keyword=dd)
  → Arg/Vars  tool={query}   # argument 置空，只写变量
    → SF-ACTIONS             # keyword 空；script 见下
      → Arg/Vars  action={query}
        → SF-RUN             # 收 input；出结果
          → Clipboard
```

**Script 正文必须用环境变量，不要用 `{var:…}` 当 CLI 参数：**

```bash
./devtools tools
./devtools actions "$tool"                 # OK — Alfred 导出 env
./devtools run "$tool" "$action" "$1"      # OK

# 错误（会得到字面量 {var:tool}）:
# ./devtools actions "{var:tool}"
```

UI 标题/副标题里的 `{var:tool}` 可以保留（Alfred 会展开）；**只有 script 字符串里的 CLI 参数**容易不展开。

| Script Filter | argumenttype | alfredfiltersresults |
|---------------|--------------|----------------------|
| SF-TOOLS      | Optional (1) | true（当前层 fuzzy） |
| SF-ACTIONS    | Optional (1) | true |
| SF-RUN        | Optional (1) | false（结果不参与 fuzzy） |

## CLI（与三级 SF 对应）

```bash
./devtools tools
./devtools actions base64
./devtools run base64 encode
./devtools run base64 encode hello
```

加工具：只在 `cmd/root.go` 的 `toolsIndex` 增加一项（`Type` + `Actions` + `Run`），列表与执行共用同一注册表。

## 开发

```bash
task devtools          # universal binary + .alfredworkflow
go test ./cmd/devtools/...
open .workflow/        # 或 open devtools.alfredworkflow
```
