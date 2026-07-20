package engine

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iamfurkann/osint-engine/internal/domain"
)

// DummyModule, motorun modülleri çalıştırıp çalıştıramadığını test etmek için
// yazılmış, sahte veriler üreten basit bir test eklentisidir.
type DummyModule struct{}

// NewDummyModule yeni bir sahte modül başlatır.
func NewDummyModule() *DummyModule {
	return &DummyModule{}
}

func (m *DummyModule) Name() string {
	return "dummy-module"
}

func (m *DummyModule) Run(ctx context.Context, target string) ([]*domain.Finding, error) {
	// Uzun süren bir network isteğini simüle etmek için kısa bir gecikme ekliyoruz.
	// Context iptal edilirse beklemeyi keser.
	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Hedef için sahte bir bulgu (E-posta) üretiyoruz
	finding := &domain.Finding{
		ID:        uuid.New().String(), // Benzersiz ID
		Type:      domain.TypeEmail,
		Value:     "admin@" + target,
		Context:   `{"confidence": 99}`,
		FoundBy:   m.Name(),
		CreatedAt: time.Now().UTC(),
	}

	return []*domain.Finding{finding}, nil
}
