package usecase_test

import (
	"testing"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
)

func TestChunkPlanner_Plan_Success(t *testing.T) {
	nodeA := &domain.WorkspaceStorageNode{
		Account: &domain.CloudAccount{ID: 1, ProviderType: "google"},
		Quota:   &domain.CloudQuota{AvailableBytes: 4 * 1024 * 1024 * 1024},
	}
	nodeB := &domain.WorkspaceStorageNode{
		Account: &domain.CloudAccount{ID: 2, ProviderType: "google"},
		Quota:   &domain.CloudQuota{AvailableBytes: 3 * 1024 * 1024 * 1024},
	}
	nodeC := &domain.WorkspaceStorageNode{
		Account: &domain.CloudAccount{ID: 3, ProviderType: "google"},
		Quota:   &domain.CloudQuota{AvailableBytes: 8 * 1024 * 1024 * 1024},
	}

	planner := usecase.NewChunkPlanner()

	fileSize := int64(10 * 1024 * 1024 * 1024) // 10 GB
	plan, err := planner.Plan(fileSize, []*domain.WorkspaceStorageNode{nodeA, nodeB, nodeC})
	if err != nil {
		t.Fatalf("expected successful plan, got error: %v", err)
	}

	if plan.TotalSize != fileSize {
		t.Errorf("expected total size %d, got %d", fileSize, plan.TotalSize)
	}

	if len(plan.Chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(plan.Chunks))
	}

	// Chunk 0: Node A, 4GB, offset 0
	if plan.Chunks[0].Node.Account.ID != 1 || plan.Chunks[0].SizeBytes != 4*1024*1024*1024 || plan.Chunks[0].OffsetBytes != 0 {
		t.Errorf("chunk 0 mismatch: %+v", plan.Chunks[0])
	}

	// Chunk 1: Node B, 3GB, offset 4GB
	if plan.Chunks[1].Node.Account.ID != 2 || plan.Chunks[1].SizeBytes != 3*1024*1024*1024 || plan.Chunks[1].OffsetBytes != 4*1024*1024*1024 {
		t.Errorf("chunk 1 mismatch: %+v", plan.Chunks[1])
	}

	// Chunk 2: Node C, 3GB, offset 7GB
	if plan.Chunks[2].Node.Account.ID != 3 || plan.Chunks[2].SizeBytes != 3*1024*1024*1024 || plan.Chunks[2].OffsetBytes != 7*1024*1024*1024 {
		t.Errorf("chunk 2 mismatch: %+v", plan.Chunks[2])
	}
}

func TestChunkPlanner_Plan_InsufficientStorage(t *testing.T) {
	nodeA := &domain.WorkspaceStorageNode{
		Account: &domain.CloudAccount{ID: 1},
		Quota:   &domain.CloudQuota{AvailableBytes: 2 * 1024 * 1024},
	}
	nodeB := &domain.WorkspaceStorageNode{
		Account: &domain.CloudAccount{ID: 2},
		Quota:   &domain.CloudQuota{AvailableBytes: 3 * 1024 * 1024},
	}

	planner := usecase.NewChunkPlanner()
	_, err := planner.Plan(10*1024*1024, []*domain.WorkspaceStorageNode{nodeA, nodeB})
	if err == nil {
		t.Fatal("expected error for insufficient storage, got nil")
	}
}
