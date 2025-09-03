package errcode

// Error 定义错误码结构
type Error struct {
	message string // 错误信息
	code    int    // 错误码
}

// NewError 创建新的错误码
func NewError(code int, message string) *Error {
	return &Error{
		code:    code,
		message: message,
	}
}

// WithError 使用原始错误包装错误码
func WithError(err *Error, originalErr error) error {
	if originalErr == nil {
		return err
	}

	return NewError(err.Code(), err.Message()+": "+originalErr.Error())
}

// // NewError create a error
// func NewError(code int, msg string) *Error {
// 	if _, ok := errorCodes[code]; ok {
// 		panic(fmt.Sprintf("code %d is exsit, please change one", code))
// 	}
// 	errorCodes[code] = struct{}{}
// 	return &Error{code: code, msg: msg}
// }

// Error 实现error接口
func (e *Error) Error() string {
	return e.message
}

// Code 获取错误码
func (e *Error) Code() int {
	return e.code
}

// Message 获取错误信息
func (e *Error) Message() string {
	return e.message
}

// WithMessage 使用新消息包装错误
func (e *Error) WithMessage(message string) *Error {
	return NewError(e.code, message)
}
