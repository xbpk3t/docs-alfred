package errcode

const (
	ErrCodeMergeRss = 17000 + iota
	ErrCodeRssRead
	ErrCodeRssParse
)

// RSS错误码 (17000-17999).
var (
	// ErrMergeRss 合并RSS失败.
	ErrMergeRss = NewError(ErrCodeMergeRss, "合并RSS失败")
	// ErrRssRead 读取RSS失败.
	ErrRssRead = NewError(ErrCodeRssRead, "读取RSS失败")
	// ErrRssParse 解析RSS失败.
	ErrRssParse = NewError(ErrCodeRssParse, "解析RSS失败")
)
