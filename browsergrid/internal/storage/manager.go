package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
)

type Manager struct {
	providers map[string]Provider
	active    Provider
	config    Config
	mu        sync.RWMutex
}

type Config struct {
	Provider string                 `yaml:"provider" json:"provider"`
	Settings map[string]interface{} `yaml:"settings" json:"settings"`
}

func NewManager(config Config) (*Manager, error) {
	m := &Manager{
		providers: make(map[string]Provider),
		config:    config,
	}

	provider, err := m.createProvider(config.Provider, config.Settings)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	m.active = provider
	m.providers[config.Provider] = provider

	return m, nil
}

func (m *Manager) GetProvider() Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

func (m *Manager) ForNamespace(namespace string) Storage {
	return &namespacedStorage{
		storage:   m.GetProvider(),
		namespace: namespace,
	}
}

func (m *Manager) ForTenant(tenantID string) Storage {
	return m.ForNamespace(tenantID)
}

func (m *Manager) HealthCheck(ctx context.Context) error {
	return m.GetProvider().HealthCheck(ctx)
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, provider := range m.providers {
		if err := provider.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) createProvider(providerType string, settings map[string]interface{}) (Provider, error) {
	switch providerType {
	case "local":
		return NewLocalProvider(settings)
	case "s3":
		return NewS3Provider(settings)

	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}

type namespacedStorage struct {
	storage   Storage
	namespace string
}

func (n *namespacedStorage) Put(ctx context.Context, key string, reader io.Reader, opts ...PutOption) error {
	return n.storage.Put(ctx, n.prefixKey(key), reader, opts...)
}

func (n *namespacedStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return n.storage.Get(ctx, n.prefixKey(key))
}

func (n *namespacedStorage) Delete(ctx context.Context, key string) error {
	return n.storage.Delete(ctx, n.prefixKey(key))
}

func (n *namespacedStorage) Exists(ctx context.Context, key string) (bool, error) {
	return n.storage.Exists(ctx, n.prefixKey(key))
}

func (n *namespacedStorage) List(ctx context.Context, prefix string, opts ...ListOption) ([]*Object, error) {
	fullPrefix := n.prefixKey(prefix)
	objects, err := n.storage.List(ctx, fullPrefix, opts...)
	if err != nil {
		return nil, err
	}

	for _, obj := range objects {
		obj.Key = n.stripPrefix(obj.Key)
	}

	return objects, nil
}

func (n *namespacedStorage) SignedURL(ctx context.Context, key string, opts ...SignedURLOption) (string, error) {
	return n.storage.SignedURL(ctx, n.prefixKey(key), opts...)
}

func (n *namespacedStorage) prefixKey(key string) string {
	return path.Join(n.namespace, key)
}

func (n *namespacedStorage) stripPrefix(key string) string {
	prefix := n.namespace + "/"
	return strings.TrimPrefix(key, prefix)
}
