package errcode

const (
	ErrCodeMergeRss = 17000 + iota
	ErrCodeRssRead
	ErrCodeRssParse
	ErrCodeRssResendSKNotFound
)

var (
	// RSS错误码 (17000-17999)
	ErrMergeRss            = NewError(ErrCodeMergeRss, "合并RSS失败")
	ErrRssRead             = NewError(ErrCodeRssRead, "读取RSS失败")
	ErrRssParse            = NewError(ErrCodeRssParse, "解析RSS失败")
	ErrRssResendSKNotFound = NewError(ErrCodeRssResendSKNotFound, "resend token is required")
)
