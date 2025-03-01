package errcode

var (
	// 文件操作错误码 (11000-11999)
	ErrWorkDir      = NewError(11000, "获取工作目录失败")
	ErrReadFile     = NewError(11001, "读取文件失败")
	ErrWriteFile    = NewError(11002, "写入文件失败")
	ErrCreateDir    = NewError(11003, "创建目录失败")
	ErrListDir      = NewError(11004, "获取目录列表失败")
	ErrCreateFile   = NewError(11005, "创建文件失败")
	ErrFileNotFound = NewError(11006, "文件不存在")
	ErrFileProcess  = NewError(11007, "文件处理失败")
	ErrSaveLocal    = NewError(11008, "保存本地文件失败")
)
