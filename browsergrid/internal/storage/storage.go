package storage

import (
	"context"
	"fmt"
	"io"
)

type Backend interface {
	Save(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type Factory func(cfg map[string]string) (Backend, error)

var registry = make(map[string]Factory)

func Register(name string, f Factory) {
	if _, exists := registry[name]; exists {
		panic("storage backend already registered: " + name)
	}
	registry[name] = f
}

func New(name string, cfg map[string]string) (Backend, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown storage backend: %s", name)
	}
	return f(cfg)
}
