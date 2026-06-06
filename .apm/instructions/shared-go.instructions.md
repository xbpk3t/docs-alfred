---
description: Shared Go package quality, dependency, network, and security rules
applyTo: "pkg/**"
---

# Shared Go Package Rules

## 包边界

- `pkg/*` 提供可复用基础能力，不能依赖具体 CLI 的 command package。
- 新共享 API 要保持小而具体；不要为了未来可能复用提前做宽泛抽象。

## 错误与日志

- library/package 返回 error，不调用 `os.Exit`、`log.Fatal`。
- 业务错误要 wrap 上下文，例如 `fmt.Errorf("read config %s: %w", path, err)`。
- 日志使用 `log/slog`；除 CLI 用户输出外，不在 package 中随意 `fmt.Println`。
- 不吞掉网络、AI、邮件、文件写入失败；允许降级时也要保留原因。
- 不打印 token、cookie、邮箱认证信息、完整敏感 payload。

## 现有依赖优先

- YAML 使用现有 `goccy/go-yaml`、`koanf`、`pkg/parser` 或已有配置 loader。
- RSS 使用 `mmcdole/gofeed` 和 `pkg/rss`。
- HTML/DOM 使用 `goquery`、readability 或已有 fetch/extract helper。
- Markdown/HTML 渲染使用 `goldmark`、`html/template` 和已有模板。
- HTTP 使用 `pkg/httputil` 或项目既有 client 风格。
- 并发使用已有 bounded pool、`context`、`errgroup`/`conc` 模式。
- 不手写 YAML/RSS/HTML/Markdown/URL parser，除非先说明现有工具无法覆盖。

## 网络与并发

- 外部请求必须有 timeout 或 caller-provided context。
- 不开 unbounded goroutine；并发量必须有明确上限或来自配置。
- retry 使用已有 retry/client 能力，不写散落的 sleep loop。
- 测试默认使用 fake server、fixture、mock，不依赖真实第三方服务。

## 文件与模板

- 写敏感或个人产物时使用私有权限，例如 `0600` 或 `fileutil.FilePermPrivate`。
- 文件写入前明确目标路径，避免 path traversal；优先复用 `fileutil`。
- 默认依赖 `html/template` escaping。
- 只有已知安全 HTML 才能使用 `template.HTML`，并在代码旁保留简短安全说明。

## 输出兼容

- `pkg/wf` 的 `alfred`、`plain`、`raw`、`rofi` 输出可能被 Alfred/Rofi/脚本消费。
- 不随意改字段名、排序、分隔符、JSON shape 或默认输出格式。
