package errcode

var (
	// RSS错误码 (17000-17999)
	ErrMergeRss            = NewError(17000, "合并RSS失败")
	ErrRssRead             = NewError(17001, "读取RSS失败")
	ErrRssParse            = NewError(17002, "解析RSS失败")
	ErrRssResendSKNotFound = NewError(17003, "resend token is required")
)
