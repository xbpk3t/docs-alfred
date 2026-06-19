# ccx

Claude Code session management CLI。当前支持 session chain walking 和 session export to wiki。

## 安装

```bash
go install github.com/xbpk3t/docs-alfred/ccx@main
```

依赖 `claude-extract`：

```bash
uv tool install claude-conversation-extractor
```

## 命令

### session chain

查看当前 session 的 chain 信息。

```bash
ccx session chain          # 默认输出
ccx session chain --json   # JSON 格式
ccx session chain --raw    # 原始输出
```

### session export

将当前 session 导出为 wiki 格式的 markdown 文件。

```bash
# 预览，不实际写入
ccx session export --dry-run --verbose

# 指定 wiki 根目录
ccx session export --wiki-root /path/to/wiki

# 指定输出目录（覆盖 wiki-root）
ccx session export --output-dir /tmp/output
```

**关键 flag：**

| Flag | 说明 |
|------|------|
| `--wiki-root` | wiki 根目录，支持绝对/相对路径 |
| `--output-dir` | 输出目录，覆盖 `--wiki-root` |
| `--dry-run` | 预览模式，不创建文件 |
| `--verbose` | 详细日志 |

## 输出格式

文件名：`YYYY-MM-DD-{title}.md`

Frontmatter：

```yaml
---
type: research
title: session-title
date: "2026-06-19"
source: claude-code
---
```

内容格式：
- `## User` / `## Claude` 作为消息标题
- 消息间用 `---` 分隔
- 无 emoji，无时间戳

## 消息过滤规则

- 只保留 `user` 和 `assistant` 角色
- 跳过空消息、`tool_use`、`thinking blocks`
- 系统消息（`<command-name>`、`<local-command-stdout>` 等）被过滤
- 内容从 `<command-args>` 标签中提取

## AI 功能

export 过程中有两个 AI 调用：

1. **标题生成** — 从 session 内容生成 ≤50 字符的语义标题
2. **内容分类** — 将内容分类到 wiki 的 topic path

## 已知问题

- **AI 分类 API 间歇性超时**（45 秒），超时时 fallback 到空路径（写入 wiki 根目录）
- **分类路径未验证** — 返回的 topic path 可能不存在于 wiki 目录中

## 改进方向

- AI 分类增加重试机制
- 验证分类路径是否存在于 wiki
- 缓存分类结果
- 更详细的性能指标日志

## 代码结构

```
ccx/
├── main.go              # 入口，cobra root command
├── cmd/
│   ├── session.go       # session 子命令
│   ├── session_export.go
│   ├── session_chain.go
│   └── config.go
└── internal/
    ├── export.go        # 导出逻辑
    └── chain.go         # chain walking 逻辑
```

设计要点：`cmd/` 处理 CLI 参数和输出，`internal/` 封装核心逻辑，错误处理带 fallback 机制。
