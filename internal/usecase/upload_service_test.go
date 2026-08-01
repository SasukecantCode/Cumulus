package usecase_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
)

type mockUploadWorkspaceRepo struct {
	workspace *domain.Workspace
	nodes     []*domain.WorkspaceStorageNode
}

func (m *mockUploadWorkspaceRepo) Create(ctx context.Context, workspace *domain.Workspace) error {
	return nil
}
func (m *mockUploadWorkspaceRepo) CreateWorkspaceWithPrimary(ctx context.Context, workspace *domain.Workspace, cloudAccountID uint64) (*domain.WorkspaceCloudAccount, error) {
	return nil, nil
}
func (m *mockUploadWorkspaceRepo) FindByID(ctx context.Context, id uint64) (*domain.Workspace, error) {
	if m.workspace != nil && m.workspace.ID == id {
		return m.workspace, nil
	}
	return nil, nil
}
func (m *mockUploadWorkspaceRepo) FindBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockUploadWorkspaceRepo) FindByCloudAccountID(ctx context.Context, cloudAccountID uint64) (*domain.Workspace, *domain.WorkspaceCloudAccount, error) {
	return nil, nil, nil
}
func (m *mockUploadWorkspaceRepo) FindMembership(ctx context.Context, cloudAccountID uint64) (*domain.WorkspaceCloudAccount, error) {
	return nil, nil
}
func (m *mockUploadWorkspaceRepo) AttachAccount(ctx context.Context, membership *domain.WorkspaceCloudAccount) error {
	return nil
}
func (m *mockUploadWorkspaceRepo) AttachSecondary(ctx context.Context, membership *domain.WorkspaceCloudAccount) error {
	return nil
}
func (m *mockUploadWorkspaceRepo) ReconnectMembership(ctx context.Context, membershipID uint64, workspaceID uint64, role domain.WorkspaceRole) error {
	return nil
}
func (m *mockUploadWorkspaceRepo) DisconnectAccount(ctx context.Context, workspaceID, cloudAccountID uint64) error {
	return nil
}
func (m *mockUploadWorkspaceRepo) GetActiveAccounts(ctx context.Context, workspaceID uint64) ([]*domain.CloudAccount, error) {
	return nil, nil
}
func (m *mockUploadWorkspaceRepo) GetActiveStorageNodes(ctx context.Context, workspaceID uint64) ([]*domain.WorkspaceStorageNode, error) {
	return m.nodes, nil
}

type mockFileRepo struct {
	files          map[uint64]*domain.File
	nextID         uint64
	failUpdateMeta bool
	failSoftDelete bool
}

func newMockFileRepo() *mockFileRepo {
	return &mockFileRepo{
		files:  make(map[uint64]*domain.File),
		nextID: 1,
	}
}

func (m *mockFileRepo) Create(ctx context.Context, file *domain.File) error {
	if file.ID == 0 {
		file.ID = m.nextID
		m.nextID++
	}
	m.files[file.ID] = file
	return nil
}

func (m *mockFileRepo) FindByID(ctx context.Context, id uint64) (*domain.File, error) {
	f, ok := m.files[id]
	if !ok {
		return nil, nil
	}
	return f, nil
}
func (m *mockFileRepo) FindByWorkspace(ctx context.Context, workspaceID uint64) ([]*domain.File, error) {
	var result []*domain.File
	for _, f := range m.files {
		if f.WorkspaceID == workspaceID {
			result = append(result, f)
		}
	}
	return result, nil
}
func (m *mockFileRepo) FindByProviderFileID(ctx context.Context, providerFileID string) (*domain.File, error) {
	for _, f := range m.files {
		if f.ProviderFileID == providerFileID {
			return f, nil
		}
	}
	return nil, nil
}
func (m *mockFileRepo) UpdateMetadata(ctx context.Context, file *domain.File) error {
	if m.failUpdateMeta {
		return errors.New("db write failed")
	}
	m.files[file.ID] = file
	return nil
}
func (m *mockFileRepo) UpdateStatus(ctx context.Context, id uint64, status domain.FileStatus) error {
	if f, ok := m.files[id]; ok {
		f.Status = status
	}
	return nil
}
func (m *mockFileRepo) ListWorkspaceFiles(ctx context.Context, filter domain.FileListFilter) ([]*domain.File, int64, error) {
	var result []*domain.File
	for _, f := range m.files {
		if f.WorkspaceID == filter.WorkspaceID && f.Status != domain.FileStatusDeleted {
			result = append(result, f)
		}
	}
	return result, int64(len(result)), nil
}
func (m *mockFileRepo) SoftDelete(ctx context.Context, id uint64) error {
	if m.failSoftDelete {
		return errors.New("db write failed")
	}
	if f, ok := m.files[id]; ok {
		f.Status = domain.FileStatusDeleted
	}
	return nil
}

func (m *mockFileRepo) Rename(ctx context.Context, id uint64, newFilename string) error {
	if f, ok := m.files[id]; ok {
		f.Filename = newFilename
	}
	return nil
}
func (m *mockFileRepo) Move(ctx context.Context, id uint64, targetParentID *uint64) error {
	if f, ok := m.files[id]; ok {
		f.ParentID = targetParentID
	}
	return nil
}
func (m *mockFileRepo) Delete(ctx context.Context, id uint64) error {
	delete(m.files, id)
	return nil
}

type mockFileChunkRepo struct {
	chunks     map[uint64][]*domain.FileChunk
	failCreate bool
}

func newMockFileChunkRepo() *mockFileChunkRepo {
	return &mockFileChunkRepo{
		chunks: make(map[uint64][]*domain.FileChunk),
	}
}

func (m *mockFileChunkRepo) Create(ctx context.Context, chunk *domain.FileChunk) error {
	if m.failCreate {
		return errors.New("db write failed")
	}
	m.chunks[chunk.FileID] = append(m.chunks[chunk.FileID], chunk)
	return nil
}

func (m *mockFileChunkRepo) CreateBatch(ctx context.Context, chunks []*domain.FileChunk) error {
	for _, c := range chunks {
		_ = m.Create(ctx, c)
	}
	return nil
}

func (m *mockFileChunkRepo) FindByFileID(ctx context.Context, fileID uint64) ([]*domain.FileChunk, error) {
	return m.chunks[fileID], nil
}

func (m *mockFileChunkRepo) DeleteByFileID(ctx context.Context, fileID uint64) error {
	delete(m.chunks, fileID)
	return nil
}

func (m *mockFileChunkRepo) UpdateStatus(ctx context.Context, id uint64, status domain.FileStatus) error {
	for _, clist := range m.chunks {
		for _, c := range clist {
			if c.ID == id {
				c.Status = status
			}
		}
	}
	return nil
}

type mockStorageProvider struct {
	uploadErr     error
	uploadFailOn  int
	deleteCalled  bool
	deletedFileID string
	deletedIDs    []string
	uploadedData  map[string][]byte
	uploadCount   int
}

func newMockStorageProvider() *mockStorageProvider {
	return &mockStorageProvider{
		uploadedData: make(map[string][]byte),
	}
}

func (m *mockStorageProvider) Upload(ctx context.Context, account *domain.CloudAccount, input domain.StorageUploadInput) (*domain.UploadedObject, error) {
	m.uploadCount++
	if m.uploadFailOn > 0 && m.uploadCount == m.uploadFailOn {
		return nil, errors.New("simulated chunk upload failure")
	}
	if m.uploadErr != nil {
		return nil, m.uploadErr
	}
	buf := &bytes.Buffer{}
	io.Copy(buf, input.Reader)
	fileID := fmt.Sprintf("provider-file-%d-%s", m.uploadCount, input.Filename)
	m.uploadedData[fileID] = buf.Bytes()
	return &domain.UploadedObject{
		ProviderFileID: fileID,
		Filename:       input.Filename,
		MimeType:       input.MimeType,
		SizeBytes:      int64(buf.Len()),
		CreatedTime:    time.Now().UTC(),
		ModifiedTime:   time.Now().UTC(),
		ETag:           "etag-123",
	}, nil
}
func (m *mockStorageProvider) Download(ctx context.Context, account *domain.CloudAccount, providerFileID string) (io.ReadCloser, error) {
	data, ok := m.uploadedData[providerFileID]
	if !ok {
		return nil, fmt.Errorf("file %s not found on provider", providerFileID)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (m *mockStorageProvider) Delete(ctx context.Context, account *domain.CloudAccount, providerFileID string) error {
	m.deleteCalled = true
	m.deletedFileID = providerFileID
	m.deletedIDs = append(m.deletedIDs, providerFileID)
	delete(m.uploadedData, providerFileID)
	return nil
}
func (m *mockStorageProvider) Rename(ctx context.Context, account *domain.CloudAccount, providerFileID string, newName string) error {
	return nil
}
func (m *mockStorageProvider) Move(ctx context.Context, account *domain.CloudAccount, providerFileID string, targetFolderID string) error {
	return nil
}
func (m *mockStorageProvider) GetMetadata(ctx context.Context, account *domain.CloudAccount, providerFileID string) (*domain.StorageFileMetadata, error) {
	return nil, nil
}

type mockScheduler struct {
	selectedNode *domain.WorkspaceStorageNode
	selectErr    error
}

func (m *mockScheduler) SelectNode(ctx context.Context, workspaceID uint64, upload *domain.UploadContext) (*domain.WorkspaceStorageNode, error) {
	if m.selectErr != nil {
		return nil, m.selectErr
	}
	return m.selectedNode, nil
}

// Test Case 1: Upload fits entirely on one account
func TestUploadFile_TestCase1_SingleAccountFits(t *testing.T) {
	ws := &domain.Workspace{ID: 10, Status: domain.WorkspaceStatusActive}
	acc := &domain.CloudAccount{ID: 20, ProviderType: "google", Status: domain.AccountStatusActive}
	node := &domain.WorkspaceStorageNode{
		Account: acc,
		Quota:   &domain.CloudQuota{AvailableBytes: 100 * 1024 * 1024, SyncStatus: domain.QuotaSyncStatusSuccess},
		Role:    domain.WorkspaceRolePrimary,
		Status:  domain.WorkspaceConnectionStatusConnected,
	}

	wsRepo := &mockUploadWorkspaceRepo{workspace: ws, nodes: []*domain.WorkspaceStorageNode{node}}
	fileRepo := newMockFileRepo()
	chunkRepo := newMockFileChunkRepo()
	prov := newMockStorageProvider()

	registry := usecase.NewMemoryStorageProviderRegistry()
	registry.Register("google", prov)

	sch := &mockScheduler{selectedNode: node}
	svc := usecase.NewUploadService(wsRepo, fileRepo, chunkRepo, registry, sch)

	payload := "Single account fits entirely!"
	expectedHash := sha256.Sum256([]byte(payload))
	expectedChecksum := hex.EncodeToString(expectedHash[:])

	file, err := svc.UploadFile(context.Background(), 10, "single.txt", int64(len(payload)), strings.NewReader(payload))
	if err != nil {
		t.Fatalf("expected successful single upload, got error: %v", err)
	}

	if file.Status != domain.FileStatusAvailable {
		t.Errorf("expected status AVAILABLE, got %s", file.Status)
	}
	if file.Checksum != expectedChecksum {
		t.Errorf("expected checksum %s, got %s", expectedChecksum, file.Checksum)
	}

	// Verify NO file_chunks records were created
	chunks, _ := chunkRepo.FindByFileID(context.Background(), file.ID)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunk records for single upload, got %d", len(chunks))
	}
}

// Test Case 2: Upload exceeds every single account but fits across multiple accounts
func TestUploadFile_TestCase2_MultiAccountChunked(t *testing.T) {
	ws := &domain.Workspace{ID: 10, Status: domain.WorkspaceStatusActive}
	accA := &domain.CloudAccount{ID: 1, ProviderType: "google", Status: domain.AccountStatusActive}
	accB := &domain.CloudAccount{ID: 2, ProviderType: "google", Status: domain.AccountStatusActive}

	nodeA := &domain.WorkspaceStorageNode{
		Account: accA,
		Quota:   &domain.CloudQuota{AvailableBytes: 40, SyncStatus: domain.QuotaSyncStatusSuccess},
		Role:    domain.WorkspaceRolePrimary,
		Status:  domain.WorkspaceConnectionStatusConnected,
	}
	nodeB := &domain.WorkspaceStorageNode{
		Account: accB,
		Quota:   &domain.CloudQuota{AvailableBytes: 60, SyncStatus: domain.QuotaSyncStatusSuccess},
		Role:    domain.WorkspaceRoleSecondary,
		Status:  domain.WorkspaceConnectionStatusConnected,
	}

	wsRepo := &mockUploadWorkspaceRepo{workspace: ws, nodes: []*domain.WorkspaceStorageNode{nodeA, nodeB}}
	fileRepo := newMockFileRepo()
	chunkRepo := newMockFileChunkRepo()
	prov := newMockStorageProvider()

	registry := usecase.NewMemoryStorageProviderRegistry()
	registry.Register("google", prov)

	// Scheduler rejects single upload because size = 80 > 40 (node A) and size = 80 > 60 (node B)
	sch := &mockScheduler{selectErr: usecase.NewAppError(usecase.ErrorCodeInsufficientStorage, http.StatusInsufficientStorage, "no single node fits file", nil)}
	svc := usecase.NewUploadService(wsRepo, fileRepo, chunkRepo, registry, sch)

	payload := "12345678901234567890123456789012345678901234567890123456789012345678901234567890" // 80 bytes
	file, err := svc.UploadFile(context.Background(), 10, "stacked.dat", 80, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("expected successful chunked upload, got error: %v", err)
	}

	if file.Status != domain.FileStatusAvailable {
		t.Errorf("expected file status AVAILABLE, got %s", file.Status)
	}

	// Verify file_chunks records created
	chunks, _ := chunkRepo.FindByFileID(context.Background(), file.ID)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunk records, got %d", len(chunks))
	}

	// Chunk 0: 40 bytes on Account 1
	if chunks[0].SizeBytes != 40 || chunks[0].CloudAccountID != 1 || chunks[0].ChunkIndex != 0 {
		t.Errorf("chunk 0 mismatch: %+v", chunks[0])
	}
	// Chunk 1: 40 bytes on Account 2
	if chunks[1].SizeBytes != 40 || chunks[1].CloudAccountID != 2 || chunks[1].ChunkIndex != 1 {
		t.Errorf("chunk 1 mismatch: %+v", chunks[1])
	}
}

// Test Case 3: Chunk upload failure (Rollback)
func TestUploadFile_TestCase3_ChunkUploadFailure_Rollback(t *testing.T) {
	ws := &domain.Workspace{ID: 10, Status: domain.WorkspaceStatusActive}
	accA := &domain.CloudAccount{ID: 1, ProviderType: "google", Status: domain.AccountStatusActive}
	accB := &domain.CloudAccount{ID: 2, ProviderType: "google", Status: domain.AccountStatusActive}

	nodeA := &domain.WorkspaceStorageNode{
		Account: accA,
		Quota:   &domain.CloudQuota{AvailableBytes: 40, SyncStatus: domain.QuotaSyncStatusSuccess},
		Role:    domain.WorkspaceRolePrimary,
		Status:  domain.WorkspaceConnectionStatusConnected,
	}
	nodeB := &domain.WorkspaceStorageNode{
		Account: accB,
		Quota:   &domain.CloudQuota{AvailableBytes: 60, SyncStatus: domain.QuotaSyncStatusSuccess},
		Role:    domain.WorkspaceRoleSecondary,
		Status:  domain.WorkspaceConnectionStatusConnected,
	}

	wsRepo := &mockUploadWorkspaceRepo{workspace: ws, nodes: []*domain.WorkspaceStorageNode{nodeA, nodeB}}
	fileRepo := newMockFileRepo()
	chunkRepo := newMockFileChunkRepo()
	prov := newMockStorageProvider()
	prov.uploadFailOn = 2 // Fail on Chunk 1 (2nd upload call)

	registry := usecase.NewMemoryStorageProviderRegistry()
	registry.Register("google", prov)

	sch := &mockScheduler{selectErr: usecase.NewAppError(usecase.ErrorCodeInsufficientStorage, http.StatusInsufficientStorage, "no single node fits file", nil)}
	svc := usecase.NewUploadService(wsRepo, fileRepo, chunkRepo, registry, sch)

	payload := "12345678901234567890123456789012345678901234567890123456789012345678901234567890" // 80 bytes
	_, err := svc.UploadFile(context.Background(), 10, "failed.dat", 80, strings.NewReader(payload))
	if err == nil {
		t.Fatalf("expected upload failure error, got nil")
	}

	// Verify Rollback: Chunk 0 was deleted from provider
	if !prov.deleteCalled {
		t.Errorf("expected remote provider Delete to be called during rollback")
	}

	// Verify metadata cleaned: 0 chunks in DB
	chunks, _ := chunkRepo.FindByFileID(context.Background(), 1)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunk records after rollback, got %d", len(chunks))
	}

	// Verify logical file status set to FAILED
	f, _ := fileRepo.FindByID(context.Background(), 1)
	if f != nil && f.Status != domain.FileStatusFailed {
		t.Errorf("expected file status FAILED, got %s", f.Status)
	}
}

func TestUploadFile_CompensatingRollback(t *testing.T) {
	ws := &domain.Workspace{ID: 10, Status: domain.WorkspaceStatusActive}
	acc := &domain.CloudAccount{ID: 20, ProviderType: "google", RefreshToken: "enc"}
	node := &domain.WorkspaceStorageNode{Account: acc, Quota: &domain.CloudQuota{AvailableBytes: 1000, SyncStatus: domain.QuotaSyncStatusSuccess}, Role: domain.WorkspaceRolePrimary, Status: domain.WorkspaceConnectionStatusConnected}

	wsRepo := &mockUploadWorkspaceRepo{workspace: ws, nodes: []*domain.WorkspaceStorageNode{node}}
	fileRepo := newMockFileRepo()
	fileRepo.failUpdateMeta = true

	prov := newMockStorageProvider()
	registry := usecase.NewMemoryStorageProviderRegistry()
	registry.Register("google", prov)

	svc := usecase.NewUploadService(wsRepo, fileRepo, nil, registry, nil)

	_, err := svc.UploadFile(context.Background(), 10, "test.txt", 12, strings.NewReader("hello world!"))
	if err == nil {
		t.Fatal("expected error when DB metadata update fails, got nil")
	}

	if !prov.deleteCalled {
		t.Errorf("expected compensating provider.Delete() to be called on DB update failure")
	}
}
