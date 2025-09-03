package errcode

const (
	ErrCodeValidateInput = 16000 + iota
	ErrCodeInvalidInput
)

var (
	// 验证错误码 (16000-16999)
	ErrValidateInput = NewError(ErrCodeValidateInput, "验证输入失败")
	ErrInvalidInput  = NewError(ErrCodeInvalidInput, "无效的输入")
)
