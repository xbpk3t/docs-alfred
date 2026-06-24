package litter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockUploader is a hand-written mock implementing Uploader without import cycle.
type mockUploader struct {
	uploadFn func(ctx context.Context, filename, content string) (*Result, error)
	name     string
}

func (m *mockUploader) Upload(ctx context.Context, filename, content string) (*Result, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, filename, content)
	}

	return &Result{URL: "https://example.com/default"}, nil
}

func (m *mockUploader) Name() string { return m.name }

func TestFallback_Name(t *testing.T) {
	f := NewFallback()
	assert.Equal(t, "fallback", f.Name())
}

func TestFallback_FirstSuccess(t *testing.T) {
	called := 0
	up1 := &mockUploader{
		name: "first",
		uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
			called++

			return &Result{URL: "https://example.com/a", Driver: "first"}, nil
		},
	}
	up2 := &mockUploader{
		name: "second",
		uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
			t.Error("second uploader should not be called when first succeeds")

			return nil, nil
		},
	}

	f := NewFallback(up1, up2)
	result, err := f.Upload(context.Background(), "test.txt", "data")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/a", result.URL)
	assert.Equal(t, "first", result.Driver)
	assert.Equal(t, 1, called)
}

func TestFallback_SecondSucceeds(t *testing.T) {
	up1 := &mockUploader{
		name: "first",
		uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
			return nil, errors.New("first failed")
		},
	}
	up2 := &mockUploader{
		name: "second",
		uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
			return &Result{URL: "https://example.com/b", Driver: "second"}, nil
		},
	}

	f := NewFallback(up1, up2)
	result, err := f.Upload(context.Background(), "test.txt", "data")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/b", result.URL)
	assert.Equal(t, "second", result.Driver)
}

func TestFallback_AllFail(t *testing.T) {
	up1 := &mockUploader{
		name: "first",
		uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
			return nil, errors.New("first error")
		},
	}
	up2 := &mockUploader{
		name: "second",
		uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
			return nil, errors.New("second error")
		},
	}

	f := NewFallback(up1, up2)
	_, err := f.Upload(context.Background(), "test.txt", "data")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all upload drivers failed")
	assert.Contains(t, err.Error(), "first error")
	assert.Contains(t, err.Error(), "second error")
}

func TestFallback_EmptyUploaders(t *testing.T) {
	f := NewFallback()
	_, err := f.Upload(context.Background(), "test.txt", "data")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all upload drivers failed")
}

func TestFallback_SingleUploaderSucceeds(t *testing.T) {
	up := &mockUploader{
		name: "only",
		uploadFn: func(_ context.Context, filename, content string) (*Result, error) {
			assert.Equal(t, "doc.pdf", filename)
			assert.Equal(t, "binary-content", content)

			return &Result{URL: "https://example.com/doc", Driver: "only"}, nil
		},
	}

	f := NewFallback(up)
	result, err := f.Upload(context.Background(), "doc.pdf", "binary-content")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/doc", result.URL)
	assert.Equal(t, "only", result.Driver)
}

func TestFallback_SingleUploaderFails(t *testing.T) {
	up := &mockUploader{
		name: "only",
		uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
			return nil, errors.New("only failed")
		},
	}

	f := NewFallback(up)
	_, err := f.Upload(context.Background(), "test.txt", "data")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all upload drivers failed")
	assert.Contains(t, err.Error(), "only failed")
}

func TestFallback_ThreeUploadersFirstSucceeds(t *testing.T) {
	up1 := &mockUploader{name: "a", uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
		return &Result{URL: "https://a.com", Driver: "a"}, nil
	}}
	up2 := &mockUploader{name: "b", uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
		t.Error("b should not be called")

		return nil, nil
	}}
	up3 := &mockUploader{name: "c", uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
		t.Error("c should not be called")

		return nil, nil
	}}

	f := NewFallback(up1, up2, up3)
	result, err := f.Upload(context.Background(), "test.txt", "data")
	require.NoError(t, err)
	assert.Equal(t, "a", result.Driver)
}

func TestFallback_ThreeUploadersMiddleSucceeds(t *testing.T) {
	up1 := &mockUploader{name: "a", uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
		return nil, errors.New("a failed")
	}}
	up2 := &mockUploader{name: "b", uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
		return &Result{URL: "https://b.com", Driver: "b"}, nil
	}}
	up3 := &mockUploader{name: "c", uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
		t.Error("c should not be called")

		return nil, nil
	}}

	f := NewFallback(up1, up2, up3)
	result, err := f.Upload(context.Background(), "test.txt", "data")
	require.NoError(t, err)
	assert.Equal(t, "b", result.Driver)
}

func TestFallback_ThreeUploadersAllFail(t *testing.T) {
	up1 := &mockUploader{name: "a", uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
		return nil, errors.New("a error")
	}}
	up2 := &mockUploader{name: "b", uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
		return nil, errors.New("b error")
	}}
	up3 := &mockUploader{name: "c", uploadFn: func(_ context.Context, _, _ string) (*Result, error) {
		return nil, errors.New("c error")
	}}

	f := NewFallback(up1, up2, up3)
	_, err := f.Upload(context.Background(), "test.txt", "data")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all upload drivers failed")
	assert.Contains(t, err.Error(), "a error")
	assert.Contains(t, err.Error(), "b error")
	assert.Contains(t, err.Error(), "c error")
}

func TestFallback_DefaultUploadFn(t *testing.T) {
	// When uploadFn is nil, the default returns a successful result.
	up := &mockUploader{name: "default"}
	f := NewFallback(up)
	result, err := f.Upload(context.Background(), "f.txt", "c")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/default", result.URL)
}

func TestNewFromNames_SpecificDrivers(t *testing.T) {
	f := NewFromNames([]string{"litterbox", "zerox0", "fileio"}, "1h")
	require.Len(t, f.uploaders, 3)
	assert.Equal(t, "litterbox", f.uploaders[0].Name())
	assert.Equal(t, "zerox0", f.uploaders[1].Name())
	assert.Equal(t, "fileio", f.uploaders[2].Name())
}

func TestNewFromNames_UnknownNamesSkipped(t *testing.T) {
	f := NewFromNames([]string{"unknown1", "unknown2"}, "1h")
	// All unknown -> falls back to default chain
	require.Len(t, f.uploaders, 3)
	assert.Equal(t, "litterbox", f.uploaders[0].Name())
	assert.Equal(t, "zerox0", f.uploaders[1].Name())
	assert.Equal(t, "fileio", f.uploaders[2].Name())
}

func TestNewFromNames_EmptyNames(t *testing.T) {
	f := NewFromNames(nil, "24h")
	require.Len(t, f.uploaders, 3)
}

func TestNewFromNames_SingleDriver(t *testing.T) {
	f := NewFromNames([]string{"zerox0"}, "1h")
	require.Len(t, f.uploaders, 1)
	assert.Equal(t, "zerox0", f.uploaders[0].Name())
}

func TestNewFromNames_MixedKnownAndUnknown(t *testing.T) {
	f := NewFromNames([]string{"unknown", "fileio", "bad"}, "72h")
	require.Len(t, f.uploaders, 1)
	assert.Equal(t, "fileio", f.uploaders[0].Name())
}

func TestNewFromNames_AllDrivers(t *testing.T) {
	f := NewFromNames([]string{"litterbox", "zerox0", "fileio"}, "12h")
	require.Len(t, f.uploaders, 3)
	assert.Equal(t, "litterbox", f.uploaders[0].Name())
	assert.Equal(t, "zerox0", f.uploaders[1].Name())
	assert.Equal(t, "fileio", f.uploaders[2].Name())
}

func TestNewFromNames_DuplicateDrivers(t *testing.T) {
	f := NewFromNames([]string{"zerox0", "zerox0", "zerox0"}, "1h")
	require.Len(t, f.uploaders, 3)
	for _, u := range f.uploaders {
		assert.Equal(t, "zerox0", u.Name())
	}
}

func TestNewFallback_Variadic(t *testing.T) {
	up1 := &mockUploader{name: "a"}
	up2 := &mockUploader{name: "b"}
	f := NewFallback(up1, up2)
	assert.Len(t, f.uploaders, 2)
}
