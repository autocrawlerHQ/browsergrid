package local

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/autocrawlerHQ/browsergrid/internal/storage"
)

type Backend struct {
	basePath string
}

func New(cfg map[string]string) (storage.Backend, error) {
	path := cfg["path"]
	if path == "" {
		path = "./data"
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	return &Backend{basePath: path}, nil
}

func init() {
	storage.Register("local", func(cfg map[string]string) (storage.Backend, error) {
		return New(cfg)
	})
}

func (b *Backend) fullPath(key string) string {
	return filepath.Join(b.basePath, filepath.Clean(key))
}

func (b *Backend) Save(ctx context.Context, key string, r io.Reader) error {
	path := b.fullPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (b *Backend) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return os.Open(b.fullPath(key))
}

func (b *Backend) Delete(ctx context.Context, key string) error {
	return os.Remove(b.fullPath(key))
}
