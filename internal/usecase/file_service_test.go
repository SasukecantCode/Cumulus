package usecase_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"testing"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
)

type mockFileServiceStorageProvider struct {
	downloadData map[string][]byte
	deleteCalls  []string
	downloadErr  error
	deleteErr    error
	renameErr    error
}

func newMockFileServiceStorageProvider() *mockFileServiceStorageProvider {
	return &mockFileServiceStorageProvider{
		downloadData: make(map[string][]byte),
	}
}

func (m *mockFileServiceStorageProvider) Upload(ctx context.Context, account *domain.CloudAccount, input domain.StorageUploadInput) (*domain.UploadedObject, error) {
	return nil, nil
}

func (m *mockFileServiceStorageProvider) Download(ctx context.Context, account *domain.CloudAccount, providerFileID string) (io.ReadCloser, error) {
	if m.downloadErr != nil {
		return nil, m.downloadErr
	}
	data, ok := m.downloadData[providerFileID]
	if !ok {
		return nil, fmt.Errorf("chunk file %s not found on provider", providerFileID)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockFileServiceStorageProvider) Delete(ctx context.Context, account *domain.CloudAccount, providerFileID string) error {
	m.deleteCalls = append(m.deleteCalls, providerFileID)
	delete(m.downloadData, providerFileID)
	return m.deleteErr
}

func (m *mockFileServiceStorageProvider) Rename(ctx context.Context, account *domain.CloudAccount, providerFileID string, newFilename string) error {
	return m.renameErr
}

func (m *mockFileServiceStorageProvider) Move(ctx context.Context, account *domain.CloudAccount, providerFileID string, targetFolderID string) error {
	return nil
}

func (m *mockFileServiceStorageProvider) GetMetadata(ctx context.Context, account *domain.CloudAccount, providerFileID string) (*domain.StorageFileMetadata, error) {
	return nil, nil
}

type mockCloudAccountRepo struct {
	accounts map[uint64]*domain.CloudAccount
}

func newMockCloudAccountRepo() *mockCloudAccountRepo {
	return &mockCloudAccountRepo{
		accounts: make(map[uint64]*domain.CloudAccount),
	}
}

func (m *mockCloudAccountRepo) CreateOrUpdate(ctx context.Context, account *domain.CloudAccount) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockCloudAccountRepo) FindByID(ctx context.Context, id uint64) (*domain.CloudAccount, error) {
	acc, ok := m.accounts[id]
	if !ok {
		return nil, nil
	}
	return acc, nil
}

func (m *mockCloudAccountRepo) FindByProviderAccount(ctx context.Context, providerType, providerAccountID string) (*domain.CloudAccount, error) {
	for _, acc := range m.accounts {
		if acc.ProviderType == providerType && acc.ProviderAccountID == providerAccountID {
			return acc, nil
		}
	}
	return nil, nil
}

func (m *mockCloudAccountRepo) FindByEmail(ctx context.Context, email string) ([]*domain.CloudAccount, error) {
	var result []*domain.CloudAccount
	for _, acc := range m.accounts {
		if acc.Email == email {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (m *mockCloudAccountRepo) Delete(ctx context.Context, id uint64) error {
	delete(m.accounts, id)
	return nil
}

type mockFileServiceWorkspaceRepo struct {
	workspaces map[uint64]*domain.Workspace
}

func newMockFileServiceWorkspaceRepo() *mockFileServiceWorkspaceRepo {
	return &mockFileServiceWorkspaceRepo{
		workspaces: make(map[uint64]*domain.Workspace),
	}
}

func (m *mockFileServiceWorkspaceRepo) Create(ctx context.Context, workspace *domain.Workspace) error {
	m.workspaces[workspace.ID] = workspace
	return nil
}
func (m *mockFileServiceWorkspaceRepo) CreateWorkspaceWithPrimary(ctx context.Context, workspace *domain.Workspace, cloudAccountID uint64) (*domain.WorkspaceCloudAccount, error) {
	return nil, nil
}
func (m *mockFileServiceWorkspaceRepo) FindByID(ctx context.Context, id uint64) (*domain.Workspace, error) {
	w, ok := m.workspaces[id]
	if !ok {
		return nil, nil
	}
	return w, nil
}
func (m *mockFileServiceWorkspaceRepo) FindBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockFileServiceWorkspaceRepo) FindByCloudAccountID(ctx context.Context, cloudAccountID uint64) (*domain.Workspace, *domain.WorkspaceCloudAccount, error) {
	return nil, nil, nil
}
func (m *mockFileServiceWorkspaceRepo) FindMembership(ctx context.Context, cloudAccountID uint64) (*domain.WorkspaceCloudAccount, error) {
	return nil, nil
}
func (m *mockFileServiceWorkspaceRepo) AttachAccount(ctx context.Context, membership *domain.WorkspaceCloudAccount) error {
	return nil
}
func (m *mockFileServiceWorkspaceRepo) AttachSecondary(ctx context.Context, membership *domain.WorkspaceCloudAccount) error {
	return nil
}
func (m *mockFileServiceWorkspaceRepo) ReconnectMembership(ctx context.Context, membershipID uint64, workspaceID uint64, role domain.WorkspaceRole) error {
	return nil
}
func (m *mockFileServiceWorkspaceRepo) DisconnectAccount(ctx context.Context, workspaceID, cloudAccountID uint64) error {
	return nil
}
func (m *mockFileServiceWorkspaceRepo) GetActiveAccounts(ctx context.Context, workspaceID uint64) ([]*domain.CloudAccount, error) {
	return nil, nil
}
func (m *mockFileServiceWorkspaceRepo) GetActiveStorageNodes(ctx context.Context, workspaceID uint64) ([]*domain.WorkspaceStorageNode, error) {
	return nil, nil
}

// Test Case 4: Download chunked file
func TestFileService_TestCase4_DownloadChunkedFile(t *testing.T) {
	wsRepo := newMockFileServiceWorkspaceRepo()
	fileRepo := newMockFileRepo()
	chunkRepo := newMockFileChunkRepo()
	accountRepo := newMockCloudAccountRepo()
	registry := usecase.NewMemoryStorageProviderRegistry()

	provider := newMockFileServiceStorageProvider()
	provider.downloadData["chunk-0-id"] = []byte("Hello, ")
	provider.downloadData["chunk-1-id"] = []byte("Virtual ")
	provider.downloadData["chunk-2-id"] = []byte("File Storage!")
	registry.Register("google", provider)

	wsRepo.workspaces[1] = &domain.Workspace{ID: 1, Status: domain.WorkspaceStatusActive}
	acc := &domain.CloudAccount{ID: 10, ProviderType: "google"}
	accountRepo.accounts[10] = acc

	fullContent := "Hello, Virtual File Storage!"
	expectedHash := sha256.Sum256([]byte(fullContent))
	expectedChecksum := hex.EncodeToString(expectedHash[:])

	file := &domain.File{
		ID:             100,
		WorkspaceID:    1,
		CloudAccountID: 10,
		Provider:       "google",
		Filename:       "stacked_doc.txt",
		Checksum:       expectedChecksum,
		SizeBytes:      int64(len(fullContent)),
		Status:         domain.FileStatusAvailable,
	}
	_ = fileRepo.Create(context.Background(), file)

	_ = chunkRepo.Create(context.Background(), &domain.FileChunk{
		FileID:         100,
		CloudAccountID: 10,
		Provider:       "google",
		ProviderFileID: "chunk-0-id",
		ChunkIndex:     0,
		OffsetBytes:    0,
		SizeBytes:      7,
		Status:         domain.FileStatusAvailable,
	})
	_ = chunkRepo.Create(context.Background(), &domain.FileChunk{
		FileID:         100,
		CloudAccountID: 10,
		Provider:       "google",
		ProviderFileID: "chunk-1-id",
		ChunkIndex:     1,
		OffsetBytes:    7,
		SizeBytes:      8,
		Status:         domain.FileStatusAvailable,
	})
	_ = chunkRepo.Create(context.Background(), &domain.FileChunk{
		FileID:         100,
		CloudAccountID: 10,
		Provider:       "google",
		ProviderFileID: "chunk-2-id",
		ChunkIndex:     2,
		OffsetBytes:    15,
		SizeBytes:      13,
		Status:         domain.FileStatusAvailable,
	})

	service := usecase.NewFileService(wsRepo, fileRepo, chunkRepo, accountRepo, registry)

	stream, fetchedFile, err := service.DownloadFile(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("failed reading download stream: %v", err)
	}

	if string(data) != fullContent {
		t.Errorf("expected '%s', got '%s'", fullContent, string(data))
	}

	actualHash := sha256.Sum256(data)
	actualChecksum := hex.EncodeToString(actualHash[:])
	if actualChecksum != fetchedFile.Checksum {
		t.Errorf("expected downloaded checksum to match %s, got %s", fetchedFile.Checksum, actualChecksum)
	}
}

// Test Case 5: Delete chunked file
func TestFileService_TestCase5_DeleteChunkedFile(t *testing.T) {
	wsRepo := newMockFileServiceWorkspaceRepo()
	fileRepo := newMockFileRepo()
	chunkRepo := newMockFileChunkRepo()
	accountRepo := newMockCloudAccountRepo()
	registry := usecase.NewMemoryStorageProviderRegistry()

	provider := newMockFileServiceStorageProvider()
	provider.downloadData["chunk-0-id"] = []byte("data0")
	provider.downloadData["chunk-1-id"] = []byte("data1")
	registry.Register("google", provider)

	wsRepo.workspaces[1] = &domain.Workspace{ID: 1, Status: domain.WorkspaceStatusActive}
	acc := &domain.CloudAccount{ID: 10, ProviderType: "google"}
	accountRepo.accounts[10] = acc

	file := &domain.File{
		ID:             200,
		WorkspaceID:    1,
		CloudAccountID: 10,
		Provider:       "google",
		Filename:       "delete_chunked.txt",
		Status:         domain.FileStatusAvailable,
	}
	_ = fileRepo.Create(context.Background(), file)

	_ = chunkRepo.Create(context.Background(), &domain.FileChunk{
		FileID:         200,
		CloudAccountID: 10,
		Provider:       "google",
		ProviderFileID: "chunk-0-id",
		ChunkIndex:     0,
	})
	_ = chunkRepo.Create(context.Background(), &domain.FileChunk{
		FileID:         200,
		CloudAccountID: 10,
		Provider:       "google",
		ProviderFileID: "chunk-1-id",
		ChunkIndex:     1,
	})

	service := usecase.NewFileService(wsRepo, fileRepo, chunkRepo, accountRepo, registry)

	err := service.DeleteFile(context.Background(), 1, 200)
	if err != nil {
		t.Fatalf("DeleteFile failed for chunked file: %v", err)
	}

	// Verify all provider chunks were deleted
	if len(provider.deleteCalls) != 2 {
		t.Errorf("expected 2 remote delete calls, got %d", len(provider.deleteCalls))
	}

	// Verify metadata cleaned
	chunks, _ := chunkRepo.FindByFileID(context.Background(), 200)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunk records in DB after deletion, got %d", len(chunks))
	}

	// Verify logical file is soft-deleted
	f, _ := fileRepo.FindByID(context.Background(), 200)
	if f.Status != domain.FileStatusDeleted {
		t.Errorf("expected logical file status DELETED, got %s", f.Status)
	}
}

func (m *mockFileServiceStorageProvider) List(ctx context.Context, account *domain.CloudAccount, input domain.StorageListInput) (*domain.StorageListPage, error) {
	return &domain.StorageListPage{}, nil
}
