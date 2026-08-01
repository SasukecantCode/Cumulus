package scheduler_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"cumulus/internal/domain"
	"cumulus/internal/infrastructure/scheduler"
	"cumulus/internal/usecase"
)

type mockWorkspaceRepo struct {
	nodes map[uint64][]*domain.WorkspaceStorageNode
}

func (m *mockWorkspaceRepo) Create(ctx context.Context, ws *domain.Workspace) error { return nil }
func (m *mockWorkspaceRepo) CreateWorkspaceWithPrimary(ctx context.Context, workspace *domain.Workspace, cloudAccountID uint64) (*domain.WorkspaceCloudAccount, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) FindByID(ctx context.Context, id uint64) (*domain.Workspace, error) {
	return &domain.Workspace{ID: id, Status: domain.WorkspaceStatusActive}, nil
}
func (m *mockWorkspaceRepo) FindBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) FindByCloudAccountID(ctx context.Context, cloudAccountID uint64) (*domain.Workspace, *domain.WorkspaceCloudAccount, error) {
	return nil, nil, nil
}
func (m *mockWorkspaceRepo) FindMembership(ctx context.Context, cloudAccountID uint64) (*domain.WorkspaceCloudAccount, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) AttachAccount(ctx context.Context, membership *domain.WorkspaceCloudAccount) error {
	return nil
}
func (m *mockWorkspaceRepo) AttachSecondary(ctx context.Context, membership *domain.WorkspaceCloudAccount) error {
	return nil
}
func (m *mockWorkspaceRepo) ReconnectMembership(ctx context.Context, membershipID uint64, workspaceID uint64, role domain.WorkspaceRole) error {
	return nil
}
func (m *mockWorkspaceRepo) DisconnectAccount(ctx context.Context, workspaceID, cloudAccountID uint64) error {
	return nil
}
func (m *mockWorkspaceRepo) GetActiveAccounts(ctx context.Context, workspaceID uint64) ([]*domain.CloudAccount, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) GetActiveStorageNodes(ctx context.Context, workspaceID uint64) ([]*domain.WorkspaceStorageNode, error) {
	return m.nodes[workspaceID], nil
}

type mockSettingsRepo struct {
	settings map[uint64]*domain.WorkspaceSettings
}

func (m *mockSettingsRepo) GetByWorkspaceID(ctx context.Context, workspaceID uint64) (*domain.WorkspaceSettings, error) {
	if s, ok := m.settings[workspaceID]; ok {
		return s, nil
	}
	return &domain.WorkspaceSettings{
		WorkspaceID:    workspaceID,
		UploadStrategy: domain.UploadStrategyFillFirst,
	}, nil
}

func (m *mockSettingsRepo) Save(ctx context.Context, settings *domain.WorkspaceSettings) error {
	m.settings[settings.WorkspaceID] = settings
	return nil
}

func createTestNode(id uint64, role domain.WorkspaceRole, total, used, avail int64) *domain.WorkspaceStorageNode {
	return &domain.WorkspaceStorageNode{
		Account: &domain.CloudAccount{
			ID:           id,
			ProviderType: "google",
			Status:       domain.AccountStatusActive,
		},
		Quota: &domain.CloudQuota{
			ID:             id,
			CloudAccountID: id,
			Provider:       "google",
			TotalBytes:     total,
			UsedBytes:      used,
			AvailableBytes: avail,
			SyncStatus:     domain.QuotaSyncStatusSuccess,
		},
		Role:   role,
		Status: domain.WorkspaceConnectionStatusConnected,
	}
}

func TestScheduler_FillFirst(t *testing.T) {
	wsRepo := &mockWorkspaceRepo{
		nodes: map[uint64][]*domain.WorkspaceStorageNode{
			1: {
				createTestNode(10, domain.WorkspaceRolePrimary, 1000, 200, 800),
				createTestNode(20, domain.WorkspaceRoleSecondary, 1000, 100, 900),
			},
		},
	}
	settingsRepo := &mockSettingsRepo{
		settings: map[uint64]*domain.WorkspaceSettings{
			1: {WorkspaceID: 1, UploadStrategy: domain.UploadStrategyFillFirst},
		},
	}

	sch := scheduler.NewUploadScheduler(wsRepo, settingsRepo)
	uploadCtx := &domain.UploadContext{Filename: "test.txt", SizeBytes: 100}

	selected, err := sch.SelectNode(context.Background(), 1, uploadCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if selected.Account.ID != 10 {
		t.Errorf("expected node ID 10 (fill first), got %d", selected.Account.ID)
	}
}

func TestScheduler_LeastUsed(t *testing.T) {
	wsRepo := &mockWorkspaceRepo{
		nodes: map[uint64][]*domain.WorkspaceStorageNode{
			1: {
				createTestNode(10, domain.WorkspaceRolePrimary, 1000, 800, 200),   // 80% used
				createTestNode(20, domain.WorkspaceRoleSecondary, 1000, 200, 800), // 20% used
				createTestNode(30, domain.WorkspaceRoleSecondary, 1000, 500, 500), // 50% used
			},
		},
	}
	settingsRepo := &mockSettingsRepo{
		settings: map[uint64]*domain.WorkspaceSettings{
			1: {WorkspaceID: 1, UploadStrategy: domain.UploadStrategyLeastUsed},
		},
	}

	sch := scheduler.NewUploadScheduler(wsRepo, settingsRepo)
	uploadCtx := &domain.UploadContext{Filename: "test.txt", SizeBytes: 100}

	selected, err := sch.SelectNode(context.Background(), 1, uploadCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if selected.Account.ID != 20 {
		t.Errorf("expected node ID 20 (least used - 20%%), got %d", selected.Account.ID)
	}
}

func TestScheduler_MostAvailable(t *testing.T) {
	wsRepo := &mockWorkspaceRepo{
		nodes: map[uint64][]*domain.WorkspaceStorageNode{
			1: {
				createTestNode(10, domain.WorkspaceRolePrimary, 1000, 800, 200),
				createTestNode(20, domain.WorkspaceRoleSecondary, 2000, 500, 1500), // 1500 available
				createTestNode(30, domain.WorkspaceRoleSecondary, 1000, 100, 900),  // 900 available
			},
		},
	}
	settingsRepo := &mockSettingsRepo{
		settings: map[uint64]*domain.WorkspaceSettings{
			1: {WorkspaceID: 1, UploadStrategy: domain.UploadStrategyMostAvailable},
		},
	}

	sch := scheduler.NewUploadScheduler(wsRepo, settingsRepo)
	uploadCtx := &domain.UploadContext{Filename: "test.txt", SizeBytes: 100}

	selected, err := sch.SelectNode(context.Background(), 1, uploadCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if selected.Account.ID != 20 {
		t.Errorf("expected node ID 20 (most available - 1500 bytes), got %d", selected.Account.ID)
	}
}

func TestScheduler_RoundRobin(t *testing.T) {
	wsRepo := &mockWorkspaceRepo{
		nodes: map[uint64][]*domain.WorkspaceStorageNode{
			1: {
				createTestNode(10, domain.WorkspaceRolePrimary, 1000, 100, 900),
				createTestNode(20, domain.WorkspaceRoleSecondary, 1000, 100, 900),
				createTestNode(30, domain.WorkspaceRoleSecondary, 1000, 100, 900),
			},
		},
	}
	settingsRepo := &mockSettingsRepo{
		settings: map[uint64]*domain.WorkspaceSettings{
			1: {WorkspaceID: 1, UploadStrategy: domain.UploadStrategyRoundRobin},
		},
	}

	sch := scheduler.NewUploadScheduler(wsRepo, settingsRepo)
	uploadCtx := &domain.UploadContext{Filename: "test.txt", SizeBytes: 100}

	expectedIDs := []uint64{10, 20, 30, 10}
	for i, expectedID := range expectedIDs {
		selected, err := sch.SelectNode(context.Background(), 1, uploadCtx)
		if err != nil {
			t.Fatalf("step %d error: %v", i, err)
		}
		if selected.Account.ID != expectedID {
			t.Errorf("step %d expected node ID %d, got %d", i, expectedID, selected.Account.ID)
		}
	}
}

func TestScheduler_InsufficientStorage_HTTP507(t *testing.T) {
	wsRepo := &mockWorkspaceRepo{
		nodes: map[uint64][]*domain.WorkspaceStorageNode{
			1: {
				createTestNode(10, domain.WorkspaceRolePrimary, 1000, 950, 50),
			},
		},
	}
	settingsRepo := &mockSettingsRepo{}

	sch := scheduler.NewUploadScheduler(wsRepo, settingsRepo)
	uploadCtx := &domain.UploadContext{Filename: "large.bin", SizeBytes: 100}

	_, err := sch.SelectNode(context.Background(), 1, uploadCtx)
	if err == nil {
		t.Fatalf("expected error for insufficient storage")
	}

	var appErr *usecase.AppError
	if errors.As(err, &appErr) {
		if appErr.StatusCode != http.StatusInsufficientStorage {
			t.Errorf("expected HTTP 507, got %d", appErr.StatusCode)
		}
		if appErr.Code != usecase.ErrorCodeInsufficientStorage {
			t.Errorf("expected code insufficient_storage, got %s", appErr.Code)
		}
	} else {
		t.Fatalf("expected AppError type, got %T", err)
	}
}

func TestScheduler_EligibilityRules(t *testing.T) {
	// Node 1: Disconnected
	n1 := createTestNode(10, domain.WorkspaceRolePrimary, 1000, 0, 1000)
	n1.Status = domain.WorkspaceConnectionStatusDisconnected

	// Node 2: Account Inactive
	n2 := createTestNode(20, domain.WorkspaceRoleSecondary, 1000, 0, 1000)
	n2.Account.Status = domain.AccountStatusExpired

	// Node 3: Quota Sync Failed
	n3 := createTestNode(30, domain.WorkspaceRoleSecondary, 1000, 0, 1000)
	n3.Quota.SyncStatus = domain.QuotaSyncStatusFailed

	// Node 4: Valid & Active
	n4 := createTestNode(40, domain.WorkspaceRoleSecondary, 1000, 0, 1000)

	wsRepo := &mockWorkspaceRepo{
		nodes: map[uint64][]*domain.WorkspaceStorageNode{
			1: {n1, n2, n3, n4},
		},
	}
	settingsRepo := &mockSettingsRepo{}

	sch := scheduler.NewUploadScheduler(wsRepo, settingsRepo)
	uploadCtx := &domain.UploadContext{Filename: "test.txt", SizeBytes: 100}

	selected, err := sch.SelectNode(context.Background(), 1, uploadCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if selected.Account.ID != 40 {
		t.Errorf("expected node ID 40 (only eligible node), got %d", selected.Account.ID)
	}
}
