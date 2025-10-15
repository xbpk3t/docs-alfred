package errcode

const (
	ErrCodeReadConfig = 14000 + iota
	ErrCodeUnmarshalConfig
	ErrCodeValidateConfig
	ErrCodeMergeConfig
	ErrCodeSetTag
	ErrCodeInvalidConfig
)

// 配置错误码 (14000-14999).
var (
	// ErrReadConfig 读取配置文件失败.
	ErrReadConfig = NewError(ErrCodeReadConfig, "reading config file")
	// ErrUnmarshalConfig 解析配置失败.
	ErrUnmarshalConfig = NewError(ErrCodeUnmarshalConfig, "unmarshaling config")
	// ErrValidateConfig 验证配置失败.
	ErrValidateConfig = NewError(ErrCodeValidateConfig, "validating config")
	// ErrMergeConfig 合并配置失败.
	ErrMergeConfig = NewError(ErrCodeMergeConfig, "合并配置失败")
	// ErrSetTag 设置标签失败.
	ErrSetTag = NewError(ErrCodeSetTag, "设置标签失败")
	// ErrInvalidConfig 无效的配置.
	ErrInvalidConfig = NewError(ErrCodeInvalidConfig, "无效的配置")
)
