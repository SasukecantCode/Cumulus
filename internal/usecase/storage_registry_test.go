package usecase_test

import (
	"context"
	"io"
	"testing"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
)

type mockStorageProviderStub struct{}

func (m *mockStorageProviderStub) Upload(ctx context.Context, account *domain.CloudAccount, input domain.StorageUploadInput) (*domain.UploadedObject, error) {
	return &domain.UploadedObject{ProviderFileID: "dummy-id"}, nil
}
func (m *mockStorageProviderStub) Download(ctx context.Context, account *domain.CloudAccount, providerFileID string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockStorageProviderStub) Delete(ctx context.Context, account *domain.CloudAccount, providerFileID string) error {
	return nil
}
func (m *mockStorageProviderStub) Rename(ctx context.Context, account *domain.CloudAccount, providerFileID string, newName string) error {
	return nil
}
func (m *mockStorageProviderStub) Move(ctx context.Context, account *domain.CloudAccount, providerFileID string, targetFolderID string) error {
	return nil
}
func (m *mockStorageProviderStub) GetMetadata(ctx context.Context, account *domain.CloudAccount, providerFileID string) (*domain.StorageFileMetadata, error) {
	return nil, nil
}

func TestMemoryStorageProviderRegistry(t *testing.T) {
	registry := usecase.NewMemoryStorageProviderRegistry()

	_, err := registry.Get("google")
	if err == nil {
		t.Fatal("expected error when getting unregistered provider, got nil")
	}

	stub := &mockStorageProviderStub{}
	registry.Register("google", stub)

	provider, err := registry.Get("google")
	if err != nil {
		t.Fatalf("expected registered provider, got error: %v", err)
	}

	if provider != stub {
		t.Fatal("returned provider does not match registered instance")
	}
}

func (m *mockStorageProviderStub) List(ctx context.Context, account *domain.CloudAccount, input domain.StorageListInput) (*domain.StorageListPage, error) {
	return &domain.StorageListPage{}, nil
}
