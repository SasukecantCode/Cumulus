package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"cumulus/internal/domain"
	"cumulus/internal/infrastructure/postgres"
)

func TestWorkspaceRepositoryStorageNodesAndTransactions(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/cumulus?sslmode=disable"
	}

	db, err := postgres.Connect(dbURL)
	if err != nil {
		t.Skipf("Skipping DB integration test: PostgreSQL not available (%v)", err)
	}

	if err := postgres.Migrate(db); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	repo := postgres.NewWorkspaceRepository(db)
	accountRepo := postgres.NewCloudAccountRepository(db)
	quotaRepo := postgres.NewQuotaRepository(db)
	ctx := context.Background()

	now := time.Now().UnixNano()
	acc := &domain.CloudAccount{
		ProviderType:      "google",
		ProviderAccountID: fmt.Sprintf("test-storage-node-account-%d", now),
		Email:             fmt.Sprintf("test.storagenode.%d@example.com", now),
		RefreshToken:      "test-refresh-token",
		GrantedScopes:     "[]",
		Status:            domain.AccountStatusActive,
	}
	if err := accountRepo.CreateOrUpdate(ctx, acc); err != nil {
		t.Fatalf("Failed to create test cloud account: %v", err)
	}

	// Create test CloudQuota with sync_status = success
	quota := &domain.CloudQuota{
		CloudAccountID: acc.ID,
		Provider:       "google",
		TotalBytes:     100000,
		UsedBytes:      20000,
		AvailableBytes: 80000,
		LastSyncedAt:   time.Now().UTC(),
		SyncStatus:     domain.QuotaSyncStatusSuccess,
	}
	if err := quotaRepo.Upsert(ctx, quota); err != nil {
		t.Fatalf("Failed to upsert cloud quota: %v", err)
	}

	// Test 1: CreateWorkspaceWithPrimary transaction
	ws := &domain.Workspace{
		UUID:      fmt.Sprintf("test-ws-uuid-%d", now),
		Name:      "Test Storage Node Workspace",
		Slug:      fmt.Sprintf("test-storage-node-slug-%d", now),
		Status:    domain.WorkspaceStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	membership, err := repo.CreateWorkspaceWithPrimary(ctx, ws, acc.ID)
	if err != nil {
		t.Fatalf("CreateWorkspaceWithPrimary failed: %v", err)
	}
	if membership.WorkspaceID != ws.ID || membership.Role != domain.WorkspaceRolePrimary {
		t.Fatalf("unexpected membership values: %+v", membership)
	}

	// Test 2: GetActiveStorageNodes eager loading and filtering
	nodes, err := repo.GetActiveStorageNodes(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetActiveStorageNodes failed: %v", err)
	}

	if len(nodes) == 0 {
		t.Fatalf("expected at least 1 storage node, got 0")
	}

	node := nodes[0]
	if node.Account == nil || node.Account.ID != acc.ID {
		t.Fatalf("expected account ID %d, got %+v", acc.ID, node.Account)
	}
	if node.Quota == nil || node.Quota.AvailableBytes != 80000 {
		t.Fatalf("expected quota available bytes 80000, got %+v", node.Quota)
	}
	if node.Role != domain.WorkspaceRolePrimary || node.Status != domain.WorkspaceConnectionStatusConnected {
		t.Fatalf("unexpected node role/status: %s / %s", node.Role, node.Status)
	}

	// Test 3: Secondary Account Attachment, Disconnect & Reconnect in DB
	accSecondary := &domain.CloudAccount{
		ProviderType:      "google",
		ProviderAccountID: fmt.Sprintf("test-secondary-account-%d", now),
		Email:             fmt.Sprintf("test.secondary.%d@example.com", now),
		RefreshToken:      "test-refresh-token-2",
		GrantedScopes:     "[]",
		Status:            domain.AccountStatusActive,
	}
	if err := accountRepo.CreateOrUpdate(ctx, accSecondary); err != nil {
		t.Fatalf("Failed to create secondary test account: %v", err)
	}

	secMem := &domain.WorkspaceCloudAccount{
		WorkspaceID:    ws.ID,
		CloudAccountID: accSecondary.ID,
		Role:           domain.WorkspaceRoleSecondary,
		Status:         domain.WorkspaceConnectionStatusConnected,
		ConnectedAt:    time.Now().UTC(),
	}
	if err := repo.AttachSecondary(ctx, secMem); err != nil {
		t.Fatalf("AttachSecondary failed: %v", err)
	}

	foundMem, err := repo.FindMembership(ctx, accSecondary.ID)
	if err != nil || foundMem == nil || foundMem.Role != domain.WorkspaceRoleSecondary {
		t.Fatalf("FindMembership failed or returned unexpected data: %+v", foundMem)
	}

	// Disconnect secondary account
	if err := repo.DisconnectAccount(ctx, ws.ID, accSecondary.ID); err != nil {
		t.Fatalf("DisconnectAccount failed: %v", err)
	}

	foundMemAfterDisconnect, _ := repo.FindMembership(ctx, accSecondary.ID)
	if foundMemAfterDisconnect.Status != domain.WorkspaceConnectionStatusDisconnected {
		t.Fatalf("expected DISCONNECTED status after disconnect, got %s", foundMemAfterDisconnect.Status)
	}

	// Reconnect secondary account
	if err := repo.ReconnectMembership(ctx, foundMemAfterDisconnect.ID, ws.ID, domain.WorkspaceRoleSecondary); err != nil {
		t.Fatalf("ReconnectMembership failed: %v", err)
	}

	foundMemAfterReconnect, _ := repo.FindMembership(ctx, accSecondary.ID)
	if foundMemAfterReconnect.Status != domain.WorkspaceConnectionStatusConnected {
		t.Fatalf("expected CONNECTED status after reconnect, got %s", foundMemAfterReconnect.Status)
	}
}
