package errcode

const (
	ErrCodeSendMail = 15000 + iota
	ErrCodeMergeFeeds
	ErrCodeInvalidMail
	ErrCodeMailConfig
)

var (
	// 邮件错误码 (15000-15999)
	ErrSendMail    = NewError(ErrCodeSendMail, "发送邮件失败")
	ErrMergeFeeds  = NewError(ErrCodeMergeFeeds, "合并Feed失败")
	ErrInvalidMail = NewError(ErrCodeInvalidMail, "无效的邮件地址")
	ErrMailConfig  = NewError(ErrCodeMailConfig, "邮件配置错误")
)
