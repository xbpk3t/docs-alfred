package errcode

const (
	ErrCodeReadConfig = 14000 + iota
	ErrCodeUnmarshalConfig
	ErrCodeValidateConfig
	ErrCodeMergeConfig
	ErrCodeSetTag
	ErrCodeInvalidConfig
)

var (
	// 配置错误码 (14000-14999)
	ErrReadConfig      = NewError(ErrCodeReadConfig, "reading config file")
	ErrUnmarshalConfig = NewError(ErrCodeUnmarshalConfig, "unmarshaling config")
	ErrValidateConfig  = NewError(ErrCodeValidateConfig, "validating config")
	ErrMergeConfig     = NewError(ErrCodeMergeConfig, "合并配置失败")
	ErrSetTag          = NewError(ErrCodeSetTag, "设置标签失败")
	ErrInvalidConfig   = NewError(ErrCodeInvalidConfig, "无效的配置")
)
