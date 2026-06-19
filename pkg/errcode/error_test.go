package errcode

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithErrorReturnsBaseErrorWhenCauseNil(t *testing.T) {
	base := NewError(1001, "reading config file")

	err := WithError(base, nil)

	require.Same(t, base, err)
}

func TestWithErrorPreservesMessageAndCause(t *testing.T) {
	base := NewError(1001, "reading config file")
	cause := errors.New("permission denied")

	err := WithError(base, cause)

	require.EqualError(t, err, "reading config file: permission denied")
	require.ErrorIs(t, err, cause)
	require.ErrorIs(t, err, base)
}

func TestWithErrorSupportsErrorsAsForErrcode(t *testing.T) {
	base := NewError(1002, "validating config")
	err := WithError(base, errors.New("missing required field"))

	var codeErr *Error
	require.ErrorAs(t, err, &codeErr)
	require.Equal(t, base.Code(), codeErr.Code())
	require.Equal(t, base.Message(), codeErr.Message())
}
