package errcode

var (
	ErrR2NSendMailFailed       = NewError(19000, "发送邮件失败")
	ErrR2NRenderTemplateFailed = NewError(19001, "渲染模板失败")
	ErrR2NParseTemplateFailed  = NewError(19002, "解析模板失败")
	ErrR2NNoFeedInType         = NewError(19003, "no feed found for type")
	ErrR2NMergeFeedsError      = NewError(19004, "merge feeds error")
)
