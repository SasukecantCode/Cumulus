package usecase

import (
	"fmt"
	"sync"

	"cumulus/internal/domain"
)

type MemoryQuotaProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]domain.QuotaProvider
}

func NewMemoryQuotaProviderRegistry() *MemoryQuotaProviderRegistry {
	return &MemoryQuotaProviderRegistry{
		providers: make(map[string]domain.QuotaProvider),
	}
}

func (r *MemoryQuotaProviderRegistry) Register(providerType string, provider domain.QuotaProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[providerType] = provider
}

func (r *MemoryQuotaProviderRegistry) Get(providerType string) (domain.QuotaProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, exists := r.providers[providerType]
	if !exists {
		return nil, fmt.Errorf("no quota provider registered for provider type: %s", providerType)
	}
	return provider, nil
}
