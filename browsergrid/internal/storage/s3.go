package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
	"gocloud.dev/gcerrors"
)

type S3Provider struct {
	bucket     *blob.Bucket
	bucketName string
	region     string
	client     *s3.Client
}

func NewS3Provider(settings map[string]interface{}) (*S3Provider, error) {
	bucketURL, ok := settings["bucket_url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required setting: bucket_url")
	}

	// Parse s3://bucket-name?region=us-east-1
	parts := strings.SplitN(strings.TrimPrefix(bucketURL, "s3://"), "?", 2)
	bucketName := parts[0]

	region := "us-east-1"
	if len(parts) > 1 {
		params := parseQuery(parts[1])
		if r, ok := params["region"]; ok {
			region = r
		}
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	bucket, err := s3blob.OpenBucket(context.Background(), client, bucketName, nil)
	if err != nil {
		return nil, fmt.Errorf("open S3 bucket: %w", err)
	}

	return &S3Provider{
		bucket:     bucket,
		bucketName: bucketName,
		region:     region,
		client:     client,
	}, nil
}

func (p *S3Provider) GetType() string {
	return "s3"
}

func (p *S3Provider) Put(ctx context.Context, key string, reader io.Reader, opts ...PutOption) error {
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

func (p *S3Provider) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	reader, err := p.bucket.NewReader(ctx, key, nil)
	if err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("open reader: %w", err)
	}

	return reader, nil
}

func (p *S3Provider) Delete(ctx context.Context, key string) error {
	err := p.bucket.Delete(ctx, key)
	if err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil
		}
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}

func (p *S3Provider) Exists(ctx context.Context, key string) (bool, error) {
	return p.bucket.Exists(ctx, key)
}

func (p *S3Provider) List(ctx context.Context, prefix string, opts ...ListOption) ([]*Object, error) {
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

func (p *S3Provider) SignedURL(ctx context.Context, key string, opts ...SignedURLOption) (string, error) {
	options := &SignedURLOptions{
		Method:  "GET",
		Expires: 1 * time.Hour,
	}

	for _, opt := range opts {
		opt(options)
	}

	urlOpts := &blob.SignedURLOptions{
		Expiry: options.Expires,
		Method: options.Method,
	}

	if options.ContentType != "" {
		urlOpts.ContentType = options.ContentType
	}

	signedURL, err := p.bucket.SignedURL(ctx, key, urlOpts)
	if err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("generate signed URL: %w", err)
	}

	return signedURL, nil
}

func (p *S3Provider) HealthCheck(ctx context.Context) error {
	iter := p.bucket.List(&blob.ListOptions{
		Prefix: ".health-check",
	})

	_, err := iter.Next(ctx)
	if err != nil && err != io.EOF {
		return fmt.Errorf("S3 health check failed: %w", err)
	}

	return nil
}

func (p *S3Provider) Close() error {
	return p.bucket.Close()
}

func parseQuery(query string) map[string]string {
	params := make(map[string]string)
	for _, param := range strings.Split(query, "&") {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) == 2 {
			params[parts[0]] = parts[1]
		}
	}
	return params
}
