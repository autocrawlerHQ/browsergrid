package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gocloud.dev/blob"
	"gocloud.dev/blob/fileblob"
	"gocloud.dev/gcerrors"
)

type LocalProvider struct {
	bucket   *blob.Bucket
	basePath string
}

func NewLocalProvider(settings map[string]interface{}) (*LocalProvider, error) {
	bucketURL, ok := settings["bucket_url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required setting: bucket_url")
	}

	u, err := url.Parse(bucketURL)
	if err != nil {
		return nil, fmt.Errorf("invalid bucket URL: %w", err)
	}

	basePath := u.Path
	if u.Host != "" {
		basePath = filepath.Join(u.Host, u.Path)
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create base directory: %w", err)
	}

	bucket, err := fileblob.OpenBucket(basePath, nil)
	if err != nil {
		return nil, fmt.Errorf("open bucket: %w", err)
	}

	return &LocalProvider{
		bucket:   bucket,
		basePath: basePath,
	}, nil
}

func (p *LocalProvider) GetType() string {
	return "local"
}

func (p *LocalProvider) Put(ctx context.Context, key string, reader io.Reader, opts ...PutOption) error {
	options := &PutOptions{
		ContentType: "application/octet-stream",
	}

	for _, opt := range opts {
		opt(options)
	}

	writeOpts := &blob.WriterOptions{
		ContentType: options.ContentType,
		Metadata:    options.Metadata,
	}

	if options.CacheControl != "" {
		writeOpts.CacheControl = options.CacheControl
	}

	writer, err := p.bucket.NewWriter(ctx, key, writeOpts)
	if err != nil {
		return fmt.Errorf("create writer: %w", err)
	}

	_, err = io.Copy(writer, reader)
	closeErr := writer.Close()

	if err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close writer: %w", closeErr)
	}

	return nil
}

func (p *LocalProvider) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	reader, err := p.bucket.NewReader(ctx, key, nil)
	if err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("open reader: %w", err)
	}

	return reader, nil
}

func (p *LocalProvider) Delete(ctx context.Context, key string) error {
	err := p.bucket.Delete(ctx, key)
	if err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil
		}
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}

func (p *LocalProvider) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := p.bucket.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("check existence: %w", err)
	}

	return exists, nil
}

func (p *LocalProvider) List(ctx context.Context, prefix string, opts ...ListOption) ([]*Object, error) {
	options := &ListOptions{
		MaxKeys: 1000,
	}

	for _, opt := range opts {
		opt(options)
	}

	listOpts := &blob.ListOptions{
		Prefix:    prefix,
		Delimiter: options.Delimiter,
	}

	var objects []*Object
	iter := p.bucket.List(listOpts)

	count := 0
	for {
		obj, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}

		if options.StartAfter != "" && obj.Key <= options.StartAfter {
			continue
		}

		etag := ""
		if obj.MD5 != nil {
			etag = fmt.Sprintf("%x", obj.MD5)
		}

		objects = append(objects, &Object{
			Key:          obj.Key,
			Size:         obj.Size,
			ContentType:  "",
			ETag:         etag,
			LastModified: obj.ModTime,
			Metadata:     nil,
		})

		count++
		if options.MaxKeys > 0 && count >= options.MaxKeys {
			break
		}
	}

	return objects, nil
}

func (p *LocalProvider) SignedURL(ctx context.Context, key string, opts ...SignedURLOption) (string, error) {
	// For local storage, return a file:// URL
	fullPath := filepath.Join(p.basePath, key)

	// Ensure the file exists
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("stat file: %w", err)
	}

	// Return file URL
	return "file://" + fullPath, nil
}

func (p *LocalProvider) HealthCheck(ctx context.Context) error {
	// Try to write and read a test file
	testKey := ".health-check-" + fmt.Sprintf("%d", time.Now().UnixNano())
	testData := []byte("health-check")

	if err := p.Put(ctx, testKey, strings.NewReader(string(testData))); err != nil {
		return fmt.Errorf("health check write failed: %w", err)
	}

	defer p.Delete(ctx, testKey)

	reader, err := p.Get(ctx, testKey)
	if err != nil {
		return fmt.Errorf("health check read failed: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("health check read data failed: %w", err)
	}

	if string(data) != string(testData) {
		return fmt.Errorf("health check data mismatch")
	}

	return nil
}

func (p *LocalProvider) Close() error {
	return p.bucket.Close()
}
