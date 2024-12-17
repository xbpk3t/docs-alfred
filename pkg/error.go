package pkg

import "errors"

var (
	ErrParseYamlError = errors.New("parse yaml file error")
	ErrConfigNotFound = errors.New("config not found")
)

type DocsAlfredError struct {
	Err  error
	Word string
}

type RepoMergeError struct {
	Err  error
	Word string
}

type PkgError struct {
	Err  error
	Word string
}

func (e *DocsAlfredError) Error() string {
	return e.Err.Error()
}
