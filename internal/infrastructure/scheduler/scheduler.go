package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
)

type UploadScheduler struct {
	workspaceRepo domain.WorkspaceRepository
	settingsRepo  domain.WorkspaceSettingsRepository
	rrCounters    map[uint64]uint64
	rrMu          sync.Mutex
}

func NewUploadScheduler(
	workspaceRepo domain.WorkspaceRepository,
	settingsRepo domain.WorkspaceSettingsRepository,
) *UploadScheduler {
	return &UploadScheduler{
		workspaceRepo: workspaceRepo,
		settingsRepo:  settingsRepo,
		rrCounters:    make(map[uint64]uint64),
	}
}

func (s *UploadScheduler) SelectNode(
	ctx context.Context,
	workspaceID uint64,
	upload *domain.UploadContext,
) (*domain.WorkspaceStorageNode, error) {
	if upload == nil || upload.SizeBytes <= 0 {
		return nil, usecase.NewAppError(usecase.ErrorCodeInvalidUploadRequest, http.StatusBadRequest, "invalid upload context or size", nil)
	}

	// 1. Fetch Workspace Settings (default FILL_FIRST if none)
	strategy := domain.UploadStrategyFillFirst
	if s.settingsRepo != nil {
		settings, err := s.settingsRepo.GetByWorkspaceID(ctx, workspaceID)
		if err == nil && settings != nil && settings.UploadStrategy != "" {
			strategy = settings.UploadStrategy
		}
	}

	// 2. Fetch Storage Nodes for Workspace
	nodes, err := s.workspaceRepo.GetActiveStorageNodes(ctx, workspaceID)
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodePrimaryAccountMissing, http.StatusInternalServerError, "failed to query workspace storage nodes", err)
	}

	// 3. Filter Eligible Storage Nodes
	// Eligibility Rules:
	// - Workspace membership: CONNECTED
	// - CloudAccount status: ACTIVE
	// - Quota sync status: SUCCESS
	// - Quota AvailableBytes >= upload.SizeBytes
	var eligible []*domain.WorkspaceStorageNode
	for _, node := range nodes {
		if node == nil || node.Account == nil || node.Quota == nil {
			continue
		}

		if node.Status != domain.WorkspaceConnectionStatusConnected {
			continue
		}

		if node.Account.Status != domain.AccountStatusActive {
			continue
		}

		if node.Quota.SyncStatus != domain.QuotaSyncStatusSuccess {
			continue
		}

		if node.Quota.AvailableBytes < upload.SizeBytes {
			continue
		}

		eligible = append(eligible, node)
	}

	if len(eligible) == 0 {
		return nil, usecase.NewAppError(
			usecase.ErrorCodeInsufficientStorage,
			http.StatusInsufficientStorage,
			fmt.Sprintf("workspace %d has no eligible storage node with at least %d bytes available capacity", workspaceID, upload.SizeBytes),
			nil,
		)
	}

	// 4. Apply Selection Strategy
	switch strategy {
	case domain.UploadStrategyFillFirst:
		return s.selectFillFirst(eligible)

	case domain.UploadStrategyLeastUsed:
		return s.selectLeastUsed(eligible)

	case domain.UploadStrategyMostAvailable:
		return s.selectMostAvailable(eligible)

	case domain.UploadStrategyRoundRobin:
		return s.selectRoundRobin(workspaceID, eligible)

	default:
		return s.selectFillFirst(eligible)
	}
}

// FILL_FIRST: Select the first node in deterministic order (sorted by Account ID)
func (s *UploadScheduler) selectFillFirst(eligible []*domain.WorkspaceStorageNode) (*domain.WorkspaceStorageNode, error) {
	sortNodesByID(eligible)
	return eligible[0], nil
}

// LEAST_USED: Select node with lowest percentage utilization (UsedBytes / TotalBytes)
func (s *UploadScheduler) selectLeastUsed(eligible []*domain.WorkspaceStorageNode) (*domain.WorkspaceStorageNode, error) {
	sortNodesByID(eligible)

	bestNode := eligible[0]
	bestRatio := calcUtilizationRatio(bestNode.Quota)

	for i := 1; i < len(eligible); i++ {
		ratio := calcUtilizationRatio(eligible[i].Quota)
		if ratio < bestRatio {
			bestRatio = ratio
			bestNode = eligible[i]
		}
	}

	return bestNode, nil
}

// MOST_AVAILABLE: Select node with highest available bytes
func (s *UploadScheduler) selectMostAvailable(eligible []*domain.WorkspaceStorageNode) (*domain.WorkspaceStorageNode, error) {
	sortNodesByID(eligible)

	bestNode := eligible[0]
	for i := 1; i < len(eligible); i++ {
		if eligible[i].Quota.AvailableBytes > bestNode.Quota.AvailableBytes {
			bestNode = eligible[i]
		}
	}

	return bestNode, nil
}

// ROUND_ROBIN: Rotate selection evenly across eligible nodes
func (s *UploadScheduler) selectRoundRobin(workspaceID uint64, eligible []*domain.WorkspaceStorageNode) (*domain.WorkspaceStorageNode, error) {
	sortNodesByID(eligible)

	s.rrMu.Lock()
	counter := s.rrCounters[workspaceID]
	idx := counter % uint64(len(eligible))
	s.rrCounters[workspaceID] = counter + 1
	s.rrMu.Unlock()

	return eligible[idx], nil
}

func sortNodesByID(nodes []*domain.WorkspaceStorageNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Account.ID < nodes[j].Account.ID
	})
}

func calcUtilizationRatio(q *domain.CloudQuota) float64 {
	if q == nil || q.TotalBytes <= 0 {
		return 1.0 // Treat uninitialized or 0 total capacity as 100% used
	}
	return float64(q.UsedBytes) / float64(q.TotalBytes)
}
