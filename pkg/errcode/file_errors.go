package errcode

const (
	ErrCodeWorkDir = 11000 + iota
	ErrCodeReadFile
	ErrCodeWriteFile
	ErrCodeCreateDir
	ErrCodeListDir
	ErrCodeCreateFile
	ErrCodeFileNotFound
	ErrCodeFileProcess
	ErrCodeSaveLocal
	ErrCodeCloseFile
)

var (
	// 文件操作错误码 (11000-11999)
	ErrWorkDir      = NewError(ErrCodeWorkDir, "获取工作目录失败")
	ErrReadFile     = NewError(ErrCodeReadFile, "读取文件失败")
	ErrWriteFile    = NewError(ErrCodeWriteFile, "写入文件失败")
	ErrCreateDir    = NewError(ErrCodeCreateDir, "创建目录失败")
	ErrListDir      = NewError(ErrCodeListDir, "获取目录列表失败")
	ErrCreateFile   = NewError(ErrCodeCreateFile, "创建文件失败")
	ErrFileNotFound = NewError(ErrCodeFileNotFound, "文件不存在")
	ErrFileProcess  = NewError(ErrCodeFileProcess, "文件处理失败")
	ErrSaveLocal    = NewError(ErrCodeSaveLocal, "保存本地文件失败")
	ErrCloseFile    = NewError(ErrCodeCloseFile, "关闭文件失败")
)
