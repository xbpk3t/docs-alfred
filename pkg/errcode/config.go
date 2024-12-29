package errcode

var (
	// 配置错误码 (14000-14999)
	ErrReadConfig      = NewError(14000, "reading config file")
	ErrUnmarshalConfig = NewError(14001, "unmarshaling config")
	ErrValidateConfig  = NewError(14002, "validating config")
	ErrMergeConfig     = NewError(14003, "合并配置失败")
	ErrSetTag          = NewError(14004, "设置标签失败")
	ErrInvalidConfig   = NewError(14005, "无效的配置")
)
