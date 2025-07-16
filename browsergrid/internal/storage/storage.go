package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFound     = errors.New("storage: object not found")
	ErrInvalidKey   = errors.New("storage: invalid key")
	ErrAccessDenied = errors.New("storage: access denied")
	ErrNotSupported = errors.New("storage: operation not supported")
)

type Storage interface {
	Put(ctx context.Context, key string, reader io.Reader, opts ...PutOption) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	List(ctx context.Context, prefix string, opts ...ListOption) ([]*Object, error)
	SignedURL(ctx context.Context, key string, opts ...SignedURLOption) (string, error)
}

type Provider interface {
	Storage
	GetType() string
	HealthCheck(ctx context.Context) error
	Close() error
}

type Object struct {
	Key          string            `json:"key"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"content_type"`
	ETag         string            `json:"etag"`
	LastModified time.Time         `json:"last_modified"`
	Metadata     map[string]string `json:"metadata"`
}

type PutOptions struct {
	ContentType  string
	Metadata     map[string]string
	CacheControl string
}

type PutOption func(*PutOptions)

func WithContentType(contentType string) PutOption {
	return func(o *PutOptions) {
		o.ContentType = contentType
	}
}

func WithMetadata(metadata map[string]string) PutOption {
	return func(o *PutOptions) {
		o.Metadata = metadata
	}
}

func WithCacheControl(cacheControl string) PutOption {
	return func(o *PutOptions) {
		o.CacheControl = cacheControl
	}
}

type ListOptions struct {
	MaxKeys    int    // 1000
	Delimiter  string // /
	StartAfter string // key
}

type ListOption func(*ListOptions)

func WithMaxKeys(max int) ListOption {
	return func(o *ListOptions) {
		o.MaxKeys = max
	}
}

func WithDelimiter(delimiter string) ListOption {
	return func(o *ListOptions) {
		o.Delimiter = delimiter
	}
}

func WithStartAfter(key string) ListOption {
	return func(o *ListOptions) {
		o.StartAfter = key
	}
}

type SignedURLOptions struct {
	Method      string        // GET, PUT, POST, DELETE
	Expires     time.Duration // 1 hour, 1 day, etc.
	ContentType string        // application/json, application/octet-stream, etc.
}

type SignedURLOption func(*SignedURLOptions)

func WithMethod(method string) SignedURLOption {
	return func(o *SignedURLOptions) {
		o.Method = method
	}
}

func WithExpires(expires time.Duration) SignedURLOption {
	return func(o *SignedURLOptions) {
		o.Expires = expires
	}
}

func WithSignedContentType(contentType string) SignedURLOption {
	return func(o *SignedURLOptions) {
		o.ContentType = contentType
	}
}
