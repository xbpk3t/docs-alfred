package errcode

const (
	ErrCodeValidateInput = 16000 + iota
	ErrCodeInvalidInput
)

// 验证错误码 (16000-16999).
var (
	// ErrValidateInput 验证输入失败.
	ErrValidateInput = NewError(ErrCodeValidateInput, "验证输入失败")
	// ErrInvalidInput 无效的输入.
	ErrInvalidInput = NewError(ErrCodeInvalidInput, "无效的输入")
)
