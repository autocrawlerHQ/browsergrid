package provider

import (
	"context"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

type Provisioner interface {
	Start(ctx context.Context, sess *sessions.Session) (wsURL, liveURL string, err error)

	Stop(ctx context.Context, sess *sessions.Session) error

	HealthCheck(ctx context.Context, sess *sessions.Session) error

	GetMetrics(ctx context.Context, sess *sessions.Session) (*sessions.SessionMetrics, error)

	GetType() workpool.ProviderType
}

type Factory struct {
	providers map[workpool.ProviderType]Provisioner
}

func NewFactory() *Factory {
	return &Factory{
		providers: make(map[workpool.ProviderType]Provisioner),
	}
}

func (f *Factory) Register(providerType workpool.ProviderType, p Provisioner) {
	f.providers[providerType] = p
}

func (f *Factory) Get(providerType workpool.ProviderType) (Provisioner, bool) {
	p, ok := f.providers[providerType]
	return p, ok
}

func (f *Factory) GetRegisteredTypes() []workpool.ProviderType {
	types := make([]workpool.ProviderType, 0, len(f.providers))
	for t := range f.providers {
		types = append(types, t)
	}
	return types
}

var DefaultFactory = NewFactory()

func FromString(providerType string) (Provisioner, bool) {
	return DefaultFactory.Get(workpool.ProviderType(providerType))
}

func Register(providerType workpool.ProviderType, p Provisioner) {
	DefaultFactory.Register(providerType, p)
}
