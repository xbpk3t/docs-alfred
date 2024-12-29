package errcode

var (
	// 验证错误码 (16000-16999)
	ErrValidateInput = NewError(16000, "验证输入失败")
	ErrInvalidInput  = NewError(16001, "无效的输入")
)
