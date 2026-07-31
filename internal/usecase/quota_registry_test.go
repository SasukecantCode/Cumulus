package usecase_test

import (
	"context"
	"testing"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
)

type mockQuotaProvider struct{}

func (m *mockQuotaProvider) GetQuota(ctx context.Context, refreshToken string) (domain.QuotaSnapshot, error) {
	return domain.QuotaSnapshot{TotalBytes: 100, UsedBytes: 20, AvailableBytes: 80}, nil
}

func TestMemoryQuotaProviderRegistry(t *testing.T) {
	registry := usecase.NewMemoryQuotaProviderRegistry()

	_, err := registry.Get("google")
	if err == nil {
		t.Fatal("expected error when getting unregistered provider, got nil")
	}

	mock := &mockQuotaProvider{}
	registry.Register("google", mock)

	provider, err := registry.Get("google")
	if err != nil {
		t.Fatalf("expected registered provider, got error: %v", err)
	}

	if provider != mock {
		t.Fatal("returned provider does not match registered instance")
	}
}
