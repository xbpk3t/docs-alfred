package errcode

const (
	ErrCodeParseConfig = 12000 + iota
)

var (
	// 解析错误码 (12000-12999)
	ErrParseConfig     = NewError(ErrCodeParseConfig, "解析配置失败")
	ErrParseWeekNumber = NewError(12001, "解析周数失败")
	ErrParseTemplate   = NewError(12002, "解析模板失败")
	ErrParseYAML       = NewError(12003, "解析YAML失败")
	ErrParseJSON       = NewError(12004, "解析JSON失败")
	ErrParseTime       = NewError(12005, "解析时间失败")
)
