package s3

import (
	"context"
	"fmt"
	"io"
	"path"

	"github.com/autocrawlerHQ/browsergrid/internal/storage"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Backend struct {
	client *s3.Client
	bucket string
	prefix string
}

func New(cfg map[string]string) (storage.Backend, error) {
	bucket := cfg["bucket"]
	region := cfg["region"]
	if bucket == "" {
		return nil, fmt.Errorf("s3: bucket is required")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg)
	return &Backend{client: client, bucket: bucket, prefix: cfg["prefix"]}, nil
}

func init() {
	storage.Register("s3", func(cfg map[string]string) (storage.Backend, error) { return New(cfg) })
}

func (b *Backend) key(k string) *string {
	if b.prefix != "" {
		k = path.Join(b.prefix, k)
	}
	return &k
}

func (b *Backend) Save(ctx context.Context, key string, r io.Reader) error {
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &b.bucket, Key: b.key(key), Body: r})
	return err
}

func (b *Backend) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := b.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &b.bucket, Key: b.key(key)})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (b *Backend) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &b.bucket, Key: b.key(key)})
	return err
}
