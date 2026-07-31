package usecase_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
)

type mockWorkspaceRepo struct {
	workspaces  map[uint64]*domain.Workspace
	memberships map[uint64]*domain.WorkspaceCloudAccount // key: cloudAccountID
	slugs       map[string]*domain.Workspace
	nextWSID    uint64
	nextMemID   uint64
}

func newMockWorkspaceRepo() *mockWorkspaceRepo {
	return &mockWorkspaceRepo{
		workspaces:  make(map[uint64]*domain.Workspace),
		memberships: make(map[uint64]*domain.WorkspaceCloudAccount),
		slugs:       make(map[string]*domain.Workspace),
		nextWSID:    1,
		nextMemID:   1,
	}
}

func (m *mockWorkspaceRepo) Create(ctx context.Context, ws *domain.Workspace) error {
	ws.ID = m.nextWSID
	m.nextWSID++
	m.workspaces[ws.ID] = ws
	m.slugs[ws.Slug] = ws
	return nil
}

func (m *mockWorkspaceRepo) CreateWorkspaceWithPrimary(ctx context.Context, ws *domain.Workspace, cloudAccountID uint64) (*domain.WorkspaceCloudAccount, error) {
	if err := m.Create(ctx, ws); err != nil {
		return nil, err
	}
	mem := &domain.WorkspaceCloudAccount{
		ID:             m.nextMemID,
		WorkspaceID:    ws.ID,
		CloudAccountID: cloudAccountID,
		Role:           domain.WorkspaceRolePrimary,
		Status:         domain.WorkspaceConnectionStatusConnected,
		ConnectedAt:    time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	m.nextMemID++
	m.memberships[cloudAccountID] = mem
	return mem, nil
}

func (m *mockWorkspaceRepo) FindByID(ctx context.Context, id uint64) (*domain.Workspace, error) {
	ws, exists := m.workspaces[id]
	if !exists {
		return nil, nil
	}
	return ws, nil
}

func (m *mockWorkspaceRepo) FindBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	ws, exists := m.slugs[slug]
	if !exists {
		return nil, nil
	}
	return ws, nil
}

func (m *mockWorkspaceRepo) FindByCloudAccountID(ctx context.Context, cloudAccountID uint64) (*domain.Workspace, *domain.WorkspaceCloudAccount, error) {
	mem, exists := m.memberships[cloudAccountID]
	if !exists || mem.Status != domain.WorkspaceConnectionStatusConnected {
		return nil, nil, nil
	}
	ws, exists := m.workspaces[mem.WorkspaceID]
	if !exists {
		return nil, nil, nil
	}
	return ws, mem, nil
}

func (m *mockWorkspaceRepo) FindMembership(ctx context.Context, cloudAccountID uint64) (*domain.WorkspaceCloudAccount, error) {
	mem, exists := m.memberships[cloudAccountID]
	if !exists {
		return nil, nil
	}
	return mem, nil
}

func (m *mockWorkspaceRepo) AttachAccount(ctx context.Context, membership *domain.WorkspaceCloudAccount) error {
	membership.ID = m.nextMemID
	m.nextMemID++
	m.memberships[membership.CloudAccountID] = membership
	return nil
}

func (m *mockWorkspaceRepo) AttachSecondary(ctx context.Context, membership *domain.WorkspaceCloudAccount) error {
	membership.Role = domain.WorkspaceRoleSecondary
	membership.Status = domain.WorkspaceConnectionStatusConnected
	return m.AttachAccount(ctx, membership)
}

func (m *mockWorkspaceRepo) ReconnectMembership(ctx context.Context, membershipID uint64, workspaceID uint64, role domain.WorkspaceRole) error {
	for _, mem := range m.memberships {
		if mem.ID == membershipID {
			mem.WorkspaceID = workspaceID
			mem.Role = role
			mem.Status = domain.WorkspaceConnectionStatusConnected
			mem.DisconnectedAt = nil
			mem.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return nil
}

func (m *mockWorkspaceRepo) DisconnectAccount(ctx context.Context, workspaceID, cloudAccountID uint64) error {
	mem, exists := m.memberships[cloudAccountID]
	if exists && mem.WorkspaceID == workspaceID {
		now := time.Now().UTC()
		mem.Status = domain.WorkspaceConnectionStatusDisconnected
		mem.DisconnectedAt = &now
		mem.UpdatedAt = now
	}
	return nil
}

func (m *mockWorkspaceRepo) GetActiveAccounts(ctx context.Context, workspaceID uint64) ([]*domain.CloudAccount, error) {
	return nil, nil
}

func (m *mockWorkspaceRepo) GetActiveStorageNodes(ctx context.Context, workspaceID uint64) ([]*domain.WorkspaceStorageNode, error) {
	return nil, nil
}

type mockQuotaRepo struct {
	quotas map[uint64]*domain.CloudQuota
}

func newMockQuotaRepo() *mockQuotaRepo {
	return &mockQuotaRepo{quotas: make(map[uint64]*domain.CloudQuota)}
}

func (mq *mockQuotaRepo) Upsert(ctx context.Context, quota *domain.CloudQuota) error {
	mq.quotas[quota.CloudAccountID] = quota
	return nil
}

func (mq *mockQuotaRepo) FindByCloudAccountID(ctx context.Context, cloudAccountID uint64) (*domain.CloudQuota, error) {
	return mq.quotas[cloudAccountID], nil
}

func TestMultiAccountWorkspaceFlow(t *testing.T) {
	repo := newMockWorkspaceRepo()
	quotaRepo := newMockQuotaRepo()
	registry := usecase.NewMemoryQuotaProviderRegistry()
	registry.Register("google", &mockQuotaProvider{})

	quotaService := usecase.NewQuotaService(quotaRepo, registry, nil)
	service := usecase.NewWorkspaceService(repo, quotaService, nil)
	ctx := context.Background()

	// 1. Initial Google A login -> Creates Workspace Alpha
	googleA := &domain.CloudAccount{ID: 1, Email: "googleA@example.com", ProviderType: "google", Status: domain.AccountStatusActive}
	wsAlpha, memA, _, err := service.ResolveLoginWorkspace(ctx, googleA)
	if err != nil {
		t.Fatalf("failed initial login for Google A: %v", err)
	}
	if wsAlpha.ID != 1 || memA.Role != domain.WorkspaceRolePrimary {
		t.Fatalf("expected Workspace Alpha (ID 1) with PRIMARY role, got ws %d, role %s", wsAlpha.ID, memA.Role)
	}

	// 2. Connect Google B as SECONDARY account to Workspace Alpha
	googleB := &domain.CloudAccount{ID: 2, Email: "googleB@example.com", ProviderType: "google", Status: domain.AccountStatusActive}
	memB, quotaB, err := service.ConnectSecondaryAccount(ctx, wsAlpha.ID, googleB)
	if err != nil {
		t.Fatalf("failed to connect Google B as secondary: %v", err)
	}
	if memB.WorkspaceID != wsAlpha.ID || memB.Role != domain.WorkspaceRoleSecondary || memB.Status != domain.WorkspaceConnectionStatusConnected {
		t.Fatalf("unexpected secondary membership for Google B: %+v", memB)
	}
	if quotaB == nil {
		t.Fatal("expected quota to sync immediately after attaching secondary account")
	}

	// 3. Login using Google B -> Must resolve Workspace Alpha, NO new workspace created
	wsResolved, memBResolved, _, err := service.ResolveLoginWorkspace(ctx, googleB)
	if err != nil {
		t.Fatalf("failed login using Google B: %v", err)
	}
	if wsResolved.ID != wsAlpha.ID {
		t.Fatalf("expected login with Google B to resolve Workspace Alpha (ID %d), got workspace ID %d", wsAlpha.ID, wsResolved.ID)
	}
	if memBResolved.Role != domain.WorkspaceRoleSecondary {
		t.Fatalf("expected role SECONDARY for Google B, got %s", memBResolved.Role)
	}

	// 4. Idempotent attachment check -> Re-attaching Google B to Workspace Alpha returns success
	memBRepeat, _, err := service.ConnectSecondaryAccount(ctx, wsAlpha.ID, googleB)
	if err != nil {
		t.Fatalf("idempotent attach failed: %v", err)
	}
	if memBRepeat.ID != memB.ID {
		t.Fatalf("expected same membership ID for repeat attach, got %d vs %d", memBRepeat.ID, memB.ID)
	}

	// 5. Cross-workspace attachment prevention -> Connecting Google B to Workspace Beta returns HTTP 409
	googleC := &domain.CloudAccount{ID: 3, Email: "googleC@example.com", ProviderType: "google", Status: domain.AccountStatusActive}
	wsBeta, _, _, err := service.ResolveLoginWorkspace(ctx, googleC)
	if err != nil {
		t.Fatalf("failed to create Workspace Beta: %v", err)
	}

	_, _, err = service.ConnectSecondaryAccount(ctx, wsBeta.ID, googleB)
	if err == nil {
		t.Fatal("expected 409 Conflict when attaching Google B to Workspace Beta, got nil error")
	}
	appErr, ok := err.(*usecase.AppError)
	if !ok || appErr.StatusCode != http.StatusConflict || appErr.Code != usecase.ErrorCodeCloudAccountAlreadyAttached {
		t.Fatalf("expected ErrorCodeCloudAccountAlreadyAttached (409), got: %v", err)
	}

	// 6. Disconnect Google B from Workspace Alpha
	if err := service.DisconnectAccount(ctx, wsAlpha.ID, googleB.ID); err != nil {
		t.Fatalf("failed to disconnect Google B: %v", err)
	}

	// 7. Verify Google B status is DISCONNECTED
	memBDisconnected, err := repo.FindMembership(ctx, googleB.ID)
	if err != nil || memBDisconnected.Status != domain.WorkspaceConnectionStatusDisconnected {
		t.Fatalf("expected Google B status to be DISCONNECTED, got %+v", memBDisconnected)
	}

	// 8. Reconnect Google B to Workspace Alpha -> Status becomes CONNECTED again
	memBReconnected, quotaBReconnected, err := service.ConnectSecondaryAccount(ctx, wsAlpha.ID, googleB)
	if err != nil {
		t.Fatalf("failed to reconnect Google B: %v", err)
	}
	if memBReconnected.Status != domain.WorkspaceConnectionStatusConnected || memBReconnected.WorkspaceID != wsAlpha.ID {
		t.Fatalf("unexpected reconnected status/workspace: %+v", memBReconnected)
	}
	if quotaBReconnected == nil {
		t.Fatal("expected quota to re-sync after reconnecting")
	}

	// 9. Primary Account disconnect rejection -> Disconnecting Google A (PRIMARY) returns error
	err = service.DisconnectAccount(ctx, wsAlpha.ID, googleA.ID)
	if err == nil {
		t.Fatal("expected error when disconnecting PRIMARY account, got nil")
	}
	primaryErr, ok := err.(*usecase.AppError)
	if !ok || primaryErr.Code != usecase.ErrorCodePrimaryAccountCannotDisconnect {
		t.Fatalf("expected ErrorCodePrimaryAccountCannotDisconnect, got %v", err)
	}
}
