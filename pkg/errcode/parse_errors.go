package errcode

const (
	ErrCodeParseConfig = 12000 + iota
)

// 解析错误码 (12000-12999).
var (
	// ErrParseConfig 解析配置失败.
	ErrParseConfig = NewError(ErrCodeParseConfig, "解析配置失败")
	// ErrParseWeekNumber 解析周数失败.
	ErrParseWeekNumber = NewError(12001, "解析周数失败")
	// ErrParseTemplate 解析模板失败.
	ErrParseTemplate = NewError(12002, "解析模板失败")
	// ErrParseYAML 解析YAML失败.
	ErrParseYAML = NewError(12003, "解析YAML失败")
	// ErrParseJSON 解析JSON失败.
	ErrParseJSON = NewError(12004, "解析JSON失败")
	// ErrParseTime 解析时间失败.
	ErrParseTime = NewError(12005, "解析时间失败")
)
