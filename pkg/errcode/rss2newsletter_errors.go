package errcode

const (
	ErrCodeR2NSendMailFailed = 19000 + iota
	ErrCodeR2NRenderTemplateFailed
	ErrCodeR2NParseTemplateFailed
	ErrCodeR2NNoFeedInType
	ErrCodeR2NMergeFeedsError
)

var (
	ErrR2NSendMailFailed       = NewError(ErrCodeR2NSendMailFailed, "发送邮件失败")
	ErrR2NRenderTemplateFailed = NewError(ErrCodeR2NRenderTemplateFailed, "渲染模板失败")
	ErrR2NParseTemplateFailed  = NewError(ErrCodeR2NParseTemplateFailed, "解析模板失败")
	ErrR2NNoFeedInType         = NewError(ErrCodeR2NNoFeedInType, "no feed found for type")
	ErrR2NMergeFeedsError      = NewError(ErrCodeR2NMergeFeedsError, "merge feeds error")
)
