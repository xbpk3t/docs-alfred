# devtools-alfred

开发者工具箱 Alfred Workflow（**单 Script Filter** + query 状态机）。

## 交互

1. `dd` → 列出 tool（目前 `base64`）
2. 选中 tool → **同一 SF 内** query 扩成 `base64 `（`valid:false` + `autocomplete`）→ 列表只有 `encode` / `decode`
3. 选中 action → query 成 `base64 encode ` → 输入内容
4. 结果 ⏎ **复制**（不自动粘贴；transient，不进剪贴板历史）

状态全在 **query 字符串**，不靠多级 Arg/Vars / 多 SF。

## Alfred 约定（改 plist 必读）

```text
SF-MAIN (keyword=dd)  script: ./devtools "$1"
  → Clipboard (autopaste=false, transient=true)
```

| 配置 | 值 | 原因 |
|------|-----|------|
| argumenttype | Optional (1) | 空 query 也能列出 tools |
| withspace | true | `dd` 后空格进 SF |
| alfredfiltersresults | **false** | 分层 filter 在 Go 里；避免整段 query 误滤 |
| queuedelaymode | Immediate (1) | 避免 Automatic+custom=3 的秒级排队 |

中间层 item：**`valid: false` + `autocomplete: "base64 encode "`**（回车扩 query，不离开 SF）。
最终层：**`valid: true` + `arg: <结果>`** → Clipboard。

## CLI

```bash
./devtools                  # 列 tools
./devtools base64           # 列 actions
./devtools base64 encode    # 提示输入
./devtools base64 encode hello
./devtools "base64 encode hello world"
```

加工具：只在 `cmd/root.go` 的 `toolsIndex` 增加一项（`Type` + `Actions` + `Run`）。

## 开发

```bash
task devtools          # universal binary + .alfredworkflow
go test ./cmd/devtools/...
open cmd/devtools/devtools.alfredworkflow   # 或 open .workflow/
```
