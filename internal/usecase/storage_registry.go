package usecase

import (
	"fmt"
	"sync"

	"cumulus/internal/domain"
)

type MemoryStorageProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]domain.StorageProvider
}

func NewMemoryStorageProviderRegistry() *MemoryStorageProviderRegistry {
	return &MemoryStorageProviderRegistry{
		providers: make(map[string]domain.StorageProvider),
	}
}

func (r *MemoryStorageProviderRegistry) Register(providerType string, provider domain.StorageProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[providerType] = provider
}

func (r *MemoryStorageProviderRegistry) Get(providerType string) (domain.StorageProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, exists := r.providers[providerType]
	if !exists {
		return nil, fmt.Errorf("no storage provider registered for provider type: %s", providerType)
	}
	return provider, nil
}
