---
description: pwgen deterministic password generation, secret handling, and output compatibility rules
applyTo: "pwgen/**"
---

# pwgen Rules

## 职责

- `pwgen` 是确定性密码生成 CLI，入口在 `pwgen/cmd/root.go`。
- 生成逻辑放在 `pwgen/pkg`，Cobra command 只负责 config/flag、参数解析和输出。

## 密钥安全

- secret 只能来自 flag、配置或安全输入；不要写入日志、测试快照、错误信息或示例输出。
- 不提交真实 `ak.json`、个人 secret、真实站点密码材料。
- 测试使用固定 fake secret，避免看起来像真实凭证。

## 兼容性

- 保持默认 flag 和配置行为兼容：`--config`、`--output/-o`、`--secret/-s`、`--length/-l`、字符集 flags。
- 输出格式 `alfred`、`plain`、`raw`、`rofi` 可能被工作流消费，不随意改 shape。
- 密码生成必须确定性：同一 website、secret、length、字符集配置得到同一结果。

## 验证

- 改生成逻辑跑 `go test ./pwgen/pkg`。
- 改 CLI flag/config/output 跑 `go test ./pwgen/...`。
- 新增字符集或长度规则时，补边界测试和兼容性测试。
