package usecase_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
)

/* ---------------- fakes ---------------- */

type syncFileRepo struct {
	rows      []*domain.File
	nextID    uint64
	updated   []*domain.File
	retired   []uint64
	createErr error
}

func newSyncFileRepo(rows ...*domain.File) *syncFileRepo {
	return &syncFileRepo{rows: rows, nextID: 100}
}

func (r *syncFileRepo) ListForSync(_ context.Context, _ domain.FileSyncQuery) ([]*domain.File, error) {
	return r.rows, nil
}

func (r *syncFileRepo) CreateBatch(_ context.Context, files []*domain.File) error {
	if r.createErr != nil {
		return r.createErr
	}
	for _, f := range files {
		r.nextID++
		f.ID = r.nextID
		r.rows = append(r.rows, f)
	}
	return nil
}

func (r *syncFileRepo) MarkMissingFromProvider(_ context.Context, ids []uint64) error {
	r.retired = append(r.retired, ids...)
	return nil
}

func (r *syncFileRepo) UpdateMetadata(_ context.Context, file *domain.File) error {
	r.updated = append(r.updated, file)
	return nil
}

func (r *syncFileRepo) Create(context.Context, *domain.File) error { return nil }
func (r *syncFileRepo) FindByID(context.Context, uint64) (*domain.File, error) {
	return nil, nil
}
func (r *syncFileRepo) FindByWorkspace(context.Context, uint64) ([]*domain.File, error) {
	return nil, nil
}
func (r *syncFileRepo) ListWorkspaceFiles(context.Context, domain.FileListFilter) ([]*domain.File, int64, error) {
	return nil, 0, nil
}
func (r *syncFileRepo) FindByProviderFileID(context.Context, string) (*domain.File, error) {
	return nil, nil
}
func (r *syncFileRepo) UpdateStatus(context.Context, uint64, domain.FileStatus) error { return nil }
func (r *syncFileRepo) SoftDelete(context.Context, uint64) error                      { return nil }
func (r *syncFileRepo) Rename(context.Context, uint64, string) error                  { return nil }
func (r *syncFileRepo) Move(context.Context, uint64, *uint64) error                   { return nil }
func (r *syncFileRepo) Delete(context.Context, uint64) error                          { return nil }

type syncWorkspaceRepo struct {
	nodes []*domain.WorkspaceStorageNode
}

func (r *syncWorkspaceRepo) FindByID(_ context.Context, id uint64) (*domain.Workspace, error) {
	return &domain.Workspace{ID: id, Status: domain.WorkspaceStatusActive}, nil
}
func (r *syncWorkspaceRepo) GetActiveStorageNodes(context.Context, uint64) ([]*domain.WorkspaceStorageNode, error) {
	return r.nodes, nil
}
func (r *syncWorkspaceRepo) Create(context.Context, *domain.Workspace) error { return nil }
func (r *syncWorkspaceRepo) CreateWorkspaceWithPrimary(context.Context, *domain.Workspace, uint64) (*domain.WorkspaceCloudAccount, error) {
	return nil, nil
}
func (r *syncWorkspaceRepo) FindBySlug(context.Context, string) (*domain.Workspace, error) {
	return nil, nil
}
func (r *syncWorkspaceRepo) FindByCloudAccountID(context.Context, uint64) (*domain.Workspace, *domain.WorkspaceCloudAccount, error) {
	return nil, nil, nil
}
func (r *syncWorkspaceRepo) FindMembership(context.Context, uint64) (*domain.WorkspaceCloudAccount, error) {
	return nil, nil
}
func (r *syncWorkspaceRepo) AttachAccount(context.Context, *domain.WorkspaceCloudAccount) error {
	return nil
}
func (r *syncWorkspaceRepo) AttachSecondary(context.Context, *domain.WorkspaceCloudAccount) error {
	return nil
}
func (r *syncWorkspaceRepo) ReconnectMembership(context.Context, uint64, uint64, domain.WorkspaceRole) error {
	return nil
}
func (r *syncWorkspaceRepo) DisconnectAccount(context.Context, uint64, uint64) error { return nil }
func (r *syncWorkspaceRepo) GetActiveAccounts(context.Context, uint64) ([]*domain.CloudAccount, error) {
	return nil, nil
}

type syncProvider struct {
	pages   [][]domain.StorageFileMetadata
	listErr error
	calls   int
}

func (p *syncProvider) List(_ context.Context, _ *domain.CloudAccount, input domain.StorageListInput) (*domain.StorageListPage, error) {
	if p.listErr != nil {
		return nil, p.listErr
	}
	idx := 0
	if input.PageToken != "" {
		idx = int(input.PageToken[0] - '0')
	}
	p.calls++
	if idx >= len(p.pages) {
		return &domain.StorageListPage{}, nil
	}
	next := ""
	if idx+1 < len(p.pages) {
		next = string(rune('0' + idx + 1))
	}
	return &domain.StorageListPage{Files: p.pages[idx], NextPageToken: next}, nil
}

func (p *syncProvider) Upload(context.Context, *domain.CloudAccount, domain.StorageUploadInput) (*domain.UploadedObject, error) {
	return nil, nil
}
func (p *syncProvider) Download(context.Context, *domain.CloudAccount, string) (io.ReadCloser, error) {
	return nil, nil
}
func (p *syncProvider) Delete(context.Context, *domain.CloudAccount, string) error         { return nil }
func (p *syncProvider) Rename(context.Context, *domain.CloudAccount, string, string) error { return nil }
func (p *syncProvider) Move(context.Context, *domain.CloudAccount, string, string) error   { return nil }
func (p *syncProvider) GetMetadata(context.Context, *domain.CloudAccount, string) (*domain.StorageFileMetadata, error) {
	return nil, nil
}

func newSyncHarness(t *testing.T, rows []*domain.File, provider *syncProvider) (*usecase.DriveSyncService, *syncFileRepo) {
	t.Helper()

	account := &domain.CloudAccount{ID: 7, ProviderType: "google", Email: "a@example.com"}
	wsRepo := &syncWorkspaceRepo{nodes: []*domain.WorkspaceStorageNode{{Account: account}}}
	fileRepo := newSyncFileRepo(rows...)

	registry := usecase.NewMemoryStorageProviderRegistry()
	registry.Register("google", provider)

	return usecase.NewDriveSyncService(wsRepo, fileRepo, nil, registry), fileRepo
}

func remote(id, name string, size int64) domain.StorageFileMetadata {
	return domain.StorageFileMetadata{
		ProviderFileID: id,
		Filename:       name,
		MimeType:       "application/pdf",
		SizeBytes:      size,
		CreatedTime:    time.Now().UTC(),
	}
}

/* ---------------- tests ---------------- */

func TestSyncImportsPreExistingDriveFiles(t *testing.T) {
	provider := &syncProvider{pages: [][]domain.StorageFileMetadata{{
		remote("drive-1", "old-tax-return.pdf", 2048),
		remote("drive-2", "holiday.jpg", 4096),
	}}}
	svc, repo := newSyncHarness(t, nil, provider)

	result, err := svc.SyncWorkspace(context.Background(), 1)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if result.Imported != 2 {
		t.Fatalf("expected 2 imported, got %d", result.Imported)
	}
	for _, row := range repo.rows {
		if row.Origin != domain.FileOriginDrive {
			t.Errorf("%s: expected origin DRIVE, got %q", row.Filename, row.Origin)
		}
		if row.Status != domain.FileStatusAvailable {
			t.Errorf("%s: expected AVAILABLE, got %q", row.Filename, row.Status)
		}
	}
}

func TestSyncDoesNotDuplicateKnownFiles(t *testing.T) {
	existing := &domain.File{
		ID: 1, WorkspaceID: 1, CloudAccountID: 7,
		ProviderFileID: "drive-1", Filename: "report.pdf", SizeBytes: 2048,
		MimeType: "application/pdf",
		Status:   domain.FileStatusAvailable, Origin: domain.FileOriginDrive,
	}
	provider := &syncProvider{pages: [][]domain.StorageFileMetadata{{remote("drive-1", "report.pdf", 2048)}}}
	svc, repo := newSyncHarness(t, []*domain.File{existing}, provider)

	result, err := svc.SyncWorkspace(context.Background(), 1)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if result.Imported != 0 || result.Updated != 0 {
		t.Fatalf("unchanged file should be a no-op, got imported=%d updated=%d", result.Imported, result.Updated)
	}
	if len(repo.rows) != 1 {
		t.Fatalf("expected still 1 row, got %d", len(repo.rows))
	}
}

func TestSyncAdoptsProviderRename(t *testing.T) {
	existing := &domain.File{
		ID: 1, WorkspaceID: 1, CloudAccountID: 7,
		ProviderFileID: "drive-1", Filename: "old.pdf", SizeBytes: 10,
		MimeType: "application/pdf",
		Status:   domain.FileStatusAvailable, Origin: domain.FileOriginDrive,
	}
	provider := &syncProvider{pages: [][]domain.StorageFileMetadata{{remote("drive-1", "renamed.pdf", 20)}}}
	svc, repo := newSyncHarness(t, []*domain.File{existing}, provider)

	if _, err := svc.SyncWorkspace(context.Background(), 1); err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("expected 1 metadata update, got %d", len(repo.updated))
	}
	if repo.updated[0].Filename != "renamed.pdf" || repo.updated[0].SizeBytes != 20 {
		t.Fatalf("provider values did not win: %+v", repo.updated[0])
	}
}

// A chunked Cumulus upload's parent row has no provider object of its own, so
// it is absent from every listing. It must never be retired by the sync.
func TestSyncNeverRetiresCumulusUploads(t *testing.T) {
	chunked := &domain.File{
		ID: 1, WorkspaceID: 1, CloudAccountID: 7,
		ProviderFileID: "", Filename: "big.iso",
		Status:         domain.FileStatusAvailable, Origin: domain.FileOriginCumulus,
	}
	single := &domain.File{
		ID: 2, WorkspaceID: 1, CloudAccountID: 7,
		ProviderFileID: "cumulus-1", Filename: "note.txt",
		Status:         domain.FileStatusAvailable, Origin: domain.FileOriginCumulus,
	}
	// The provider lists neither of them.
	provider := &syncProvider{pages: [][]domain.StorageFileMetadata{{remote("drive-9", "other.pdf", 1)}}}
	svc, repo := newSyncHarness(t, []*domain.File{chunked, single}, provider)

	result, err := svc.SyncWorkspace(context.Background(), 1)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if result.Removed != 0 {
		t.Fatalf("expected no removals, got %d", result.Removed)
	}
	if len(repo.retired) != 0 {
		t.Fatalf("cumulus rows must never be retired, got %v", repo.retired)
	}
}

func TestSyncRetiresImportedFilesDeletedInDrive(t *testing.T) {
	gone := &domain.File{
		ID: 5, WorkspaceID: 1, CloudAccountID: 7,
		ProviderFileID: "drive-gone", Filename: "deleted-in-drive.pdf",
		Status:         domain.FileStatusAvailable, Origin: domain.FileOriginDrive,
	}
	provider := &syncProvider{pages: [][]domain.StorageFileMetadata{{remote("drive-1", "kept.pdf", 1)}}}
	svc, repo := newSyncHarness(t, []*domain.File{gone}, provider)

	result, err := svc.SyncWorkspace(context.Background(), 1)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if result.Removed != 1 {
		t.Fatalf("expected 1 removal, got %d", result.Removed)
	}
	if len(repo.retired) != 1 || repo.retired[0] != 5 {
		t.Fatalf("expected row 5 retired, got %v", repo.retired)
	}
}

// A file deleted in Cumulus but still in Drive comes back, rather than being
// inserted a second time under a new ID.
func TestSyncRevivesLocallyDeletedFileStillInDrive(t *testing.T) {
	deleted := &domain.File{
		ID: 3, WorkspaceID: 1, CloudAccountID: 7,
		ProviderFileID: "drive-1", Filename: "resurrect.pdf", SizeBytes: 5,
		MimeType: "application/pdf",
		Status:   domain.FileStatusDeleted, Origin: domain.FileOriginDrive,
	}
	provider := &syncProvider{pages: [][]domain.StorageFileMetadata{{remote("drive-1", "resurrect.pdf", 5)}}}
	svc, repo := newSyncHarness(t, []*domain.File{deleted}, provider)

	result, err := svc.SyncWorkspace(context.Background(), 1)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if result.Imported != 0 {
		t.Fatalf("expected revival, not a fresh import; imported=%d", result.Imported)
	}
	if len(repo.updated) != 1 || repo.updated[0].Status != domain.FileStatusAvailable {
		t.Fatalf("expected row revived to AVAILABLE, got %+v", repo.updated)
	}
}

func TestSyncWalksEveryPage(t *testing.T) {
	provider := &syncProvider{pages: [][]domain.StorageFileMetadata{
		{remote("a", "a.pdf", 1)},
		{remote("b", "b.pdf", 1)},
		{remote("c", "c.pdf", 1)},
	}}
	svc, _ := newSyncHarness(t, nil, provider)

	result, err := svc.SyncWorkspace(context.Background(), 1)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if result.Imported != 3 {
		t.Fatalf("expected all 3 pages imported, got %d", result.Imported)
	}
}

// One bad account reports its own error and leaves the workspace usable.
func TestSyncReportsAccountFailureWithoutAborting(t *testing.T) {
	provider := &syncProvider{listErr: errors.New("token expired")}
	svc, _ := newSyncHarness(t, nil, provider)

	result, err := svc.SyncWorkspace(context.Background(), 1)
	if err != nil {
		t.Fatalf("workspace sync should not fail outright: %v", err)
	}
	if len(result.Accounts) != 1 || result.Accounts[0].Error == "" {
		t.Fatalf("expected the account to carry its own error, got %+v", result.Accounts)
	}
}
