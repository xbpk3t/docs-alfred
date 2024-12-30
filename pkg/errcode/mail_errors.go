package errcode

var (
	// 邮件错误码 (15000-15999)
	ErrSendMail    = NewError(15000, "发送邮件失败")
	ErrMergeFeeds  = NewError(15001, "合并Feed失败")
	ErrInvalidMail = NewError(15002, "无效的邮件地址")
	ErrMailConfig  = NewError(15003, "邮件配置错误")
)
