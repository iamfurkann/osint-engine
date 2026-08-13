package engine

import (
	"context"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

type DummyModule struct{}

func NewDummyModule() *DummyModule {
	return &DummyModule{}
}

// Manifest, DummyModule'ün kimlik kartını döndürür.
func (m *DummyModule) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "dummy-1",
		Name:        "dummy-module",
		Description: "A dummy module for testing the engine architecture",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Language:    "go",
		Inputs:      []string{"domain", "email"},
	}
}

func (m *DummyModule) Timeout() time.Duration {
	return 1 * time.Second
}

// Artık domain.Finding değil, eklentiye özel ham plugin.Result döndürüyoruz
func (m *DummyModule) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	res := plugin.Result{
		Type:    "email", // E-posta türü
		Value:   "admin@" + target,
		Context: `{"confidence": 99}`,
	}

	return []plugin.Result{res}, nil
}
