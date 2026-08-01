package usecase

import (
	"fmt"
	"net/http"
	"sort"

	"cumulus/internal/domain"
)

type ChunkSpec struct {
	ChunkIndex  int
	Node        *domain.WorkspaceStorageNode
	SizeBytes   int64
	OffsetBytes int64
}

type ChunkPlan struct {
	Chunks    []ChunkSpec
	TotalSize int64
}

type ChunkPlanner struct{}

func NewChunkPlanner() *ChunkPlanner {
	return &ChunkPlanner{}
}

func (p *ChunkPlanner) Plan(fileSize int64, nodes []*domain.WorkspaceStorageNode) (*ChunkPlan, error) {
	if fileSize <= 0 {
		return nil, NewAppError(ErrorCodeInvalidUploadRequest, http.StatusBadRequest, "invalid file size for chunking", nil)
	}

	var eligible []*domain.WorkspaceStorageNode
	var totalAvailable int64

	for _, n := range nodes {
		if n != nil && n.Account != nil && n.Quota != nil && n.Quota.AvailableBytes > 0 {
			eligible = append(eligible, n)
			totalAvailable += n.Quota.AvailableBytes
		}
	}

	if totalAvailable < fileSize {
		return nil, NewAppError(
			ErrorCodeInsufficientStorage,
			http.StatusInsufficientStorage,
			fmt.Sprintf("insufficient total available storage across nodes (%d bytes available, %d bytes required)", totalAvailable, fileSize),
			nil,
		)
	}

	// Sort eligible nodes deterministically by Account.ID
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].Account.ID < eligible[j].Account.ID
	})

	var chunks []ChunkSpec
	remaining := fileSize
	var offset int64

	for i, node := range eligible {
		if remaining <= 0 {
			break
		}
		allocated := node.Quota.AvailableBytes
		if allocated > remaining {
			allocated = remaining
		}

		chunks = append(chunks, ChunkSpec{
			ChunkIndex:  i,
			Node:        node,
			SizeBytes:   allocated,
			OffsetBytes: offset,
		})

		offset += allocated
		remaining -= allocated
	}

	if remaining > 0 {
		return nil, NewAppError(
			ErrorCodeInsufficientStorage,
			http.StatusInsufficientStorage,
			fmt.Sprintf("failed to allocate chunks for remaining %d bytes", remaining),
			nil,
		)
	}

	return &ChunkPlan{
		Chunks:    chunks,
		TotalSize: fileSize,
	}, nil
}
