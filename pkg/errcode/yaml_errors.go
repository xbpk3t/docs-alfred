package errcode

var (
	// YAML操作错误码 (12000-12999)
	ErrYAMLMarshal   = NewError(12000, "YAML序列化失败")
	ErrYAMLUnmarshal = NewError(12001, "YAML反序列化失败")
)
