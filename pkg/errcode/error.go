package errcode

import "errors"

// Error 定义错误码结构.
type Error struct {
	message string // 错误信息
	code    int    // 错误码
}

type wrappedError struct {
	err   *Error
	cause error
}

// NewError 创建新的错误码.
func NewError(code int, message string) *Error {
	return &Error{
		code:    code,
		message: message,
	}
}

// WithError 使用原始错误包装错误码.
func WithError(err *Error, originalErr error) error {
	if originalErr == nil {
		return err
	}

	return &wrappedError{err: err, cause: originalErr}
}

// // NewError create a error
// func NewError(code int, msg string) *Error {
// 	if _, ok := errorCodes[code]; ok {
// 		panic(fmt.Sprintf("code %d is exsit, please change one", code))
// 	}
// 	errorCodes[code] = struct{}{}
// 	return &Error{code: code, msg: msg}
// }

// Error 实现error接口.
func (e *Error) Error() string {
	return e.message
}

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}

	return e.code == t.code && e.message == t.message
}

// Code 获取错误码.
func (e *Error) Code() int {
	return e.code
}

// Message 获取错误信息.
func (e *Error) Message() string {
	return e.message
}

// WithMessage 使用新消息包装错误.
func (e *Error) WithMessage(message string) *Error {
	return NewError(e.code, message)
}

func (e *wrappedError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	if e.cause == nil {
		return e.err.Error()
	}

	return e.err.Error() + ": " + e.cause.Error()
}

func (e *wrappedError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.cause
}

func (e *wrappedError) Is(target error) bool {
	if e == nil || e.err == nil {
		return false
	}

	return errors.Is(e.err, target) || errors.Is(e.cause, target)
}

func (e *wrappedError) As(target any) bool {
	if e == nil {
		return false
	}

	if errors.As(e.err, target) {
		return true
	}

	return errors.As(e.cause, target)
}

func (e *wrappedError) Code() int {
	if e == nil || e.err == nil {
		return 0
	}

	return e.err.Code()
}

func (e *wrappedError) Message() string {
	if e == nil || e.err == nil {
		return ""
	}

	return e.err.Message()
}
