package usecase_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"testing"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
)

func TestManual_VirtualFileStorage_EndToEnd(t *testing.T) {
	// 1. Setup workspace with 3 storage accounts, each with small capacity (30 bytes, 30 bytes, 40 bytes)
	ws := &domain.Workspace{ID: 500, Status: domain.WorkspaceStatusActive}
	accA := &domain.CloudAccount{ID: 101, ProviderType: "google", Status: domain.AccountStatusActive}
	accB := &domain.CloudAccount{ID: 102, ProviderType: "google", Status: domain.AccountStatusActive}
	accC := &domain.CloudAccount{ID: 103, ProviderType: "google", Status: domain.AccountStatusActive}

	nodeA := &domain.WorkspaceStorageNode{
		Account: accA,
		Quota:   &domain.CloudQuota{AvailableBytes: 30, SyncStatus: domain.QuotaSyncStatusSuccess},
		Role:    domain.WorkspaceRolePrimary,
		Status:  domain.WorkspaceConnectionStatusConnected,
	}
	nodeB := &domain.WorkspaceStorageNode{
		Account: accB,
		Quota:   &domain.CloudQuota{AvailableBytes: 30, SyncStatus: domain.QuotaSyncStatusSuccess},
		Role:    domain.WorkspaceRoleSecondary,
		Status:  domain.WorkspaceConnectionStatusConnected,
	}
	nodeC := &domain.WorkspaceStorageNode{
		Account: accC,
		Quota:   &domain.CloudQuota{AvailableBytes: 40, SyncStatus: domain.QuotaSyncStatusSuccess},
		Role:    domain.WorkspaceRoleSecondary,
		Status:  domain.WorkspaceConnectionStatusConnected,
	}

	wsRepo := &mockUploadWorkspaceRepo{
		workspace: ws,
		nodes:     []*domain.WorkspaceStorageNode{nodeA, nodeB, nodeC},
	}
	fileRepo := newMockFileRepo()
	chunkRepo := newMockFileChunkRepo()
	accountRepo := newMockCloudAccountRepo()
	accountRepo.accounts[101] = accA
	accountRepo.accounts[102] = accB
	accountRepo.accounts[103] = accC

	provider := newMockStorageProvider()
	registry := usecase.NewMemoryStorageProviderRegistry()
	registry.Register("google", provider)

	sch := &mockScheduler{selectErr: usecase.NewAppError(usecase.ErrorCodeInsufficientStorage, http.StatusInsufficientStorage, "no single node fits", nil)}

	uploadSvc := usecase.NewUploadService(wsRepo, fileRepo, chunkRepo, registry, sch)
	fileSvc := usecase.NewFileService(wsRepo, fileRepo, chunkRepo, accountRepo, registry)

	// 2. Prepare 75-byte original payload
	payloadStr := "Cumulus Virtual File Storage Manual Test: Stacking 75 bytes across 3 accounts!"
	payload := []byte(payloadStr)
	originalHash := sha256.Sum256(payload)
	originalChecksum := hex.EncodeToString(originalHash[:])

	// STEP 1: Manual Upload of Chunked File
	t.Log("Executing Manual Upload of Chunked File (75 bytes)...")
	file, err := uploadSvc.UploadFile(context.Background(), 500, "virtual_test.txt", int64(len(payload)), bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Manual Chunked Upload failed: %v", err)
	}

	if file.Status != domain.FileStatusAvailable {
		t.Fatalf("expected file status AVAILABLE, got %s", file.Status)
	}
	if file.Checksum != originalChecksum {
		t.Fatalf("expected file checksum %s, got %s", originalChecksum, file.Checksum)
	}

	// Verify chunk layout in chunkRepo: 30 bytes (Node A), 30 bytes (Node B), 15 bytes (Node C)
	chunks, err := chunkRepo.FindByFileID(context.Background(), file.ID)
	if err != nil || len(chunks) != 3 {
		t.Fatalf("expected 3 chunks in DB, got len=%d err=%v", len(chunks), err)
	}
	t.Logf("Manual Upload Succeeded! Created %d chunks across accounts: Chunk 0 (%d bytes), Chunk 1 (%d bytes), Chunk 2 (%d bytes)",
		len(chunks), chunks[0].SizeBytes, chunks[1].SizeBytes, chunks[2].SizeBytes)

	// STEP 2: Manual Download Reconstructs Original File
	t.Log("Executing Manual Download to Reconstruct Original File...")
	stream, downloadedMeta, err := fileSvc.DownloadFile(context.Background(), 500, file.ID)
	if err != nil {
		t.Fatalf("Manual Download failed: %v", err)
	}
	defer stream.Close()

	reconstructedBytes, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("reading download stream failed: %v", err)
	}

	if string(reconstructedBytes) != payloadStr {
		t.Fatalf("reconstructed content mismatch! expected '%s', got '%s'", payloadStr, string(reconstructedBytes))
	}

	reconstructedHash := sha256.Sum256(reconstructedBytes)
	reconstructedChecksum := hex.EncodeToString(reconstructedHash[:])
	if reconstructedChecksum != downloadedMeta.Checksum {
		t.Fatalf("reconstructed checksum %s does not match metadata %s", reconstructedChecksum, downloadedMeta.Checksum)
	}
	t.Log("Manual Download Succeeded! Original file reconstructed perfectly and checksum matched!")

	// STEP 3: Manual Deletion Removes Every Chunk Successfully
	t.Log("Executing Manual Deletion of Chunked File...")
	err = fileSvc.DeleteFile(context.Background(), 500, file.ID)
	if err != nil {
		t.Fatalf("Manual Deletion failed: %v", err)
	}

	remainingChunks, _ := chunkRepo.FindByFileID(context.Background(), file.ID)
	if len(remainingChunks) != 0 {
		t.Fatalf("expected 0 chunks remaining after deletion, got %d", len(remainingChunks))
	}

	deletedFile, _ := fileRepo.FindByID(context.Background(), file.ID)
	if deletedFile.Status != domain.FileStatusDeleted {
		t.Fatalf("expected file status DELETED, got %s", deletedFile.Status)
	}
	t.Log("Manual Deletion Succeeded! Every provider chunk deleted and metadata cleaned successfully!")
}
