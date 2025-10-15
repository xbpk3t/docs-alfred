package errcode

const (
	ErrCodeYAMLMarshal = 12000 + iota
	ErrCodeYAMLUnmarshal
)

// YAML操作错误码 (12000-12999).
var (
	// ErrYAMLMarshal YAML序列化失败.
	ErrYAMLMarshal = NewError(ErrCodeYAMLMarshal, "YAML序列化失败")
	// ErrYAMLUnmarshal YAML反序列化失败.
	ErrYAMLUnmarshal = NewError(ErrCodeYAMLUnmarshal, "YAML反序列化失败")
)
