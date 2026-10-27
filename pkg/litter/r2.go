package litter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

// R2Config configures the Cloudflare R2 uploader.
type R2Config struct {
	Endpoint        string // S3 API endpoint, e.g. https://<account>.r2.cloudflarestorage.com
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string // public base for direct links, e.g. https://dw.lucc.dev
	KeyPrefix       string // optional key prefix, e.g. "wiki/"
}

// R2 uploads objects to Cloudflare R2 via the S3 API and returns public
// direct links. Content-Type is set at PUT time from the filename extension
// (R2 has no lightweight metadata update, so it must be correct on write),
// and each upload is verified through the public URL before it is reported —
// a PUT can succeed while the public link is still broken (bucket/domain
// config lives outside the upload path).
type R2 struct {
	store         objectStore
	bucket        string
	publicBaseURL string
	keyPrefix     string
}

// objectStore is the subset of the minio client the driver uses; tests fake it.
type objectStore interface {
	Put(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error
	List(ctx context.Context, bucket, prefix string) ([]string, error)
	Get(ctx context.Context, bucket, key string) ([]byte, error)
}

type minioStore struct {
	client *minio.Client
}

func (m *minioStore) Put(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType})

	return err
}

func (m *minioStore) List(ctx context.Context, bucket, prefix string) ([]string, error) {
	var keys []string
	for obj := range m.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		keys = append(keys, obj.Key)
	}

	return keys, nil
}

func (m *minioStore) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	obj, err := m.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = obj.Close() }()

	return io.ReadAll(obj)
}

// NewR2 creates an R2 uploader from explicit configuration.
func NewR2(cfg *R2Config) (*R2, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" ||
		cfg.SecretAccessKey == "" || cfg.PublicBaseURL == "" {
		return nil, errors.New("r2: endpoint, bucket, credentials and publicBaseUrl are required")
	}

	host := strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "https://"), "http://")
	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: !strings.HasPrefix(cfg.Endpoint, "http://"),
	})
	if err != nil {
		return nil, fmt.Errorf("r2 client: %w", err)
	}

	return newR2WithStore(&minioStore{client: client}, cfg), nil
}

func newR2WithStore(store objectStore, cfg *R2Config) *R2 {
	return &R2{
		store:         store,
		bucket:        cfg.Bucket,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
		keyPrefix:     normalizeKeyPrefix(cfg.KeyPrefix),
	}
}

func (r *R2) Name() string { return driverR2 }

// Upload stores content under the given object name and returns its public
// direct link. The name is a path of [A-Za-z0-9._-] segments separated by "/"
// (e.g. "digest/wiki-2026-10-26/AI/agent/1a2b3c4d.md"); flat names still work.
func (r *R2) Upload(ctx context.Context, filename, content string) (*Result, error) {
	key, err := r.objectKey(filename)
	if err != nil {
		return nil, err
	}
	contentType := contentTypeForName(filename)
	if err := r.store.Put(ctx, r.bucket, key, strings.NewReader(content), int64(len(content)), contentType); err != nil {
		return nil, fmt.Errorf("r2 put %s: %w", key, err)
	}

	publicURL := r.publicBaseURL + "/" + key
	if err := verifyPublicObject(ctx, publicURL, contentType); err != nil {
		return nil, fmt.Errorf("r2 self-check %s: %w", publicURL, err)
	}

	return &Result{URL: publicURL, Driver: driverR2}, nil
}

// ListKeys lists object keys under prefix (within the configured key prefix).
// Returned keys are relative to the configured key prefix, so they can be
// passed straight back to GetObject.
func (r *R2) ListKeys(ctx context.Context, prefix string) ([]string, error) {
	keys, err := r.store.List(ctx, r.bucket, r.keyPrefix+prefix)
	if err != nil {
		return nil, fmt.Errorf("r2 list %s: %w", prefix, err)
	}
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		names = append(names, strings.TrimPrefix(key, r.keyPrefix))
	}

	return names, nil
}

// GetObject reads an object's content. A missing object reads as empty (nil,
// nil) so read-merge-write callers can append to a not-yet-existing object
// without special-casing; any other error is returned. The wiki digest uses it
// to read index files and merge JSONL logs.
func (r *R2) GetObject(ctx context.Context, key string) ([]byte, error) {
	data, err := r.store.Get(ctx, r.bucket, r.keyPrefix+key)
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, nil
		}

		return nil, fmt.Errorf("r2 get %s: %w", key, err)
	}

	return data, nil
}

// r2KeySegRe matches one key path segment: [A-Za-z0-9][A-Za-z0-9._-]*.
var r2KeySegRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func (r *R2) objectKey(filename string) (string, error) {
	if !validObjectKey(filename) {
		return "", fmt.Errorf("r2: invalid object name %q (want [A-Za-z0-9._-] path segments separated by /)", filename)
	}

	return r.keyPrefix + filename, nil
}

// validObjectKey accepts a "/"-separated path where every segment is a non-empty
// [A-Za-z0-9][A-Za-z0-9._-]* token (so ".." and empty segments are rejected).
func validObjectKey(name string) bool {
	if name == "" {
		return false
	}
	for _, seg := range strings.Split(name, "/") {
		if !r2KeySegRe.MatchString(seg) {
			return false
		}
	}

	return true
}

func normalizeKeyPrefix(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}

	return p + "/"
}

// contentTypeForName maps our artifact extensions to the Content-Type stored
// on the object. R2 serves exactly this value, so HTML renders and markdown
// shows as text instead of downloading.
func contentTypeForName(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".md":
		return "text/markdown; charset=utf-8"
	case ".json":
		return "application/json"
	case ".xml", ".opml":
		return "application/xml"
	default:
		return "application/octet-stream"
	}
}

// verifyClient is shared across self-checks so the digest write path (a HEAD
// after every upload) reuses one connection pool instead of rebuilding a client
// (and its TLS handshake) per call.
var verifyClient = httputil.NewRestyClient(httputil.DefaultClientTimeout, 2)

// verifyPublicObject checks the freshly uploaded object through its public URL:
// HTTP 200 and the expected media type. Any mismatch (private bucket, broken
// domain binding, wrong Content-Type) surfaces as an error so callers can
// decide how to degrade instead of publishing a dead link.
func verifyPublicObject(ctx context.Context, publicURL, wantContentType string) error {
	resp, err := verifyClient.R().
		SetContext(ctx).
		Head(publicURL)
	if err != nil {
		return fmt.Errorf("head: %w", err)
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("head: HTTP %d", resp.StatusCode())
	}
	gotMedia := mediaType(resp.Header().Get("Content-Type"))
	wantMedia := mediaType(wantContentType)
	if !strings.EqualFold(gotMedia, wantMedia) {
		return fmt.Errorf("content-type mismatch: want %s, got %s", wantMedia, gotMedia)
	}

	return nil
}

func mediaType(contentType string) string {
	media, _, _ := strings.Cut(contentType, ";")

	return strings.TrimSpace(media)
}
