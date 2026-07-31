package usecase

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cumulus/internal/domain"
	"cumulus/internal/infrastructure/encryption"
)

type WorkspaceService struct {
	workspaceRepo domain.WorkspaceRepository
	quotaService  *QuotaService
	encryptor     *encryption.TokenEncryptor
}

func NewWorkspaceService(workspaceRepo domain.WorkspaceRepository, quotaService *QuotaService, encryptor *encryption.TokenEncryptor) *WorkspaceService {
	return &WorkspaceService{
		workspaceRepo: workspaceRepo,
		quotaService:  quotaService,
		encryptor:     encryptor,
	}
}

func (s *WorkspaceService) ResolveLoginWorkspace(ctx context.Context, account *domain.CloudAccount) (*domain.Workspace, *domain.WorkspaceCloudAccount, *domain.CloudQuota, error) {
	if account == nil {
		return nil, nil, nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusBadRequest, "cloud account is nil", nil)
	}

	workspace, membership, err := s.workspaceRepo.FindByCloudAccountID(ctx, account.ID)
	if err != nil {
		return nil, nil, nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusInternalServerError, "failed to resolve workspace", err)
	}
	if workspace != nil && membership != nil && membership.Status == domain.WorkspaceConnectionStatusConnected {
		quota, quotaErr := s.quotaService.SyncAccountQuota(ctx, account)
		if quotaErr != nil {
			return workspace, membership, quota, quotaErr
		}
		return workspace, membership, quota, nil
	}

	if existingMem, memErr := s.workspaceRepo.FindMembership(ctx, account.ID); memErr == nil && existingMem != nil {
		if ws, wsErr := s.workspaceRepo.FindByID(ctx, existingMem.WorkspaceID); wsErr == nil && ws != nil {
			if recErr := s.workspaceRepo.ReconnectMembership(ctx, existingMem.ID, ws.ID, existingMem.Role); recErr == nil {
				if reconnectedMem, fetchErr := s.workspaceRepo.FindMembership(ctx, account.ID); fetchErr == nil && reconnectedMem != nil {
					quota, _ := s.quotaService.SyncAccountQuota(ctx, account)
					return ws, reconnectedMem, quota, nil
				}
			}
		}
	}

	workspace, membership, err = s.createWorkspaceAndAttachPrimary(ctx, account)
	if err != nil {
		return nil, nil, nil, err
	}

	quota, quotaErr := s.quotaService.SyncAccountQuota(ctx, account)
	if quotaErr != nil {
		return workspace, membership, quota, quotaErr
	}

	return workspace, membership, quota, nil
}

func (s *WorkspaceService) ConnectSecondaryAccount(ctx context.Context, workspaceID uint64, account *domain.CloudAccount) (*domain.WorkspaceCloudAccount, *domain.CloudQuota, error) {
	if account == nil {
		return nil, nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusBadRequest, "cloud account is nil", nil)
	}

	ws, err := s.workspaceRepo.FindByID(ctx, workspaceID)
	if err != nil {
		return nil, nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusInternalServerError, "failed to query workspace", err)
	}
	if ws == nil {
		return nil, nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusNotFound, "workspace not found", nil)
	}

	existingMembership, err := s.workspaceRepo.FindMembership(ctx, account.ID)
	if err != nil {
		return nil, nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusInternalServerError, "failed to query account membership", err)
	}

	if existingMembership != nil {
		// Case A: Already connected to current Workspace -> Idempotent success
		if existingMembership.Status == domain.WorkspaceConnectionStatusConnected && existingMembership.WorkspaceID == workspaceID {
			quota, _ := s.quotaService.SyncAccountQuota(ctx, account)
			return existingMembership, quota, nil
		}

		// Case B: Connected to another active Workspace -> HTTP 409 Conflict
		if existingMembership.Status == domain.WorkspaceConnectionStatusConnected && existingMembership.WorkspaceID != workspaceID {
			return nil, nil, NewAppError(ErrorCodeCloudAccountAlreadyAttached, http.StatusConflict, "cloud account already attached to another workspace", nil)
		}

		// Case C: Previously disconnected -> Reconnect
		if existingMembership.Status == domain.WorkspaceConnectionStatusDisconnected {
			targetRole := domain.WorkspaceRoleSecondary
			if err := s.workspaceRepo.ReconnectMembership(ctx, existingMembership.ID, workspaceID, targetRole); err != nil {
				return nil, nil, NewAppError(ErrorCodeCloudAccountAlreadyAttached, http.StatusInternalServerError, "failed to reconnect account membership", err)
			}
			reconnectedMembership, err := s.workspaceRepo.FindMembership(ctx, account.ID)
			if err != nil {
				return nil, nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusInternalServerError, "failed to fetch reconnected membership", err)
			}
			quota, _ := s.quotaService.SyncAccountQuota(ctx, account)
			return reconnectedMembership, quota, nil
		}
	}

	// Case D: Brand new account membership
	membership := &domain.WorkspaceCloudAccount{
		WorkspaceID:    workspaceID,
		CloudAccountID: account.ID,
		Role:           domain.WorkspaceRoleSecondary,
		Status:         domain.WorkspaceConnectionStatusConnected,
		ConnectedAt:    time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.workspaceRepo.AttachSecondary(ctx, membership); err != nil {
		return nil, nil, NewAppError(ErrorCodeCloudAccountAlreadyAttached, http.StatusConflict, "failed to attach secondary account", err)
	}

	quota, _ := s.quotaService.SyncAccountQuota(ctx, account)
	return membership, quota, nil
}

func (s *WorkspaceService) DisconnectAccount(ctx context.Context, workspaceID, cloudAccountID uint64) error {
	membership, err := s.workspaceRepo.FindMembership(ctx, cloudAccountID)
	if err != nil {
		return NewAppError(ErrorCodeWorkspaceNotFound, http.StatusInternalServerError, "failed to query account membership", err)
	}
	if membership == nil || membership.WorkspaceID != workspaceID || membership.Status != domain.WorkspaceConnectionStatusConnected {
		return NewAppError(ErrorCodeWorkspaceNotFound, http.StatusNotFound, "active membership not found for workspace", nil)
	}
	if membership.Role == domain.WorkspaceRolePrimary {
		return NewAppError(ErrorCodePrimaryAccountCannotDisconnect, http.StatusBadRequest, "primary account cannot be disconnected", nil)
	}
	if err := s.workspaceRepo.DisconnectAccount(ctx, workspaceID, cloudAccountID); err != nil {
		return NewAppError(ErrorCodeWorkspaceNotFound, http.StatusInternalServerError, "failed to disconnect account", err)
	}
	return nil
}

func (s *WorkspaceService) createWorkspaceAndAttachPrimary(ctx context.Context, account *domain.CloudAccount) (*domain.Workspace, *domain.WorkspaceCloudAccount, error) {
	workspace := &domain.Workspace{
		UUID:      strings.ReplaceAll(fmt.Sprintf("ws-%d-%d", account.ID, time.Now().UnixNano()), " ", ""),
		Name:      fmt.Sprintf("Workspace - %s", account.Email),
		Slug:      workspaceSlug(account.Email, account.ID),
		Status:    domain.WorkspaceStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	membership, err := s.workspaceRepo.CreateWorkspaceWithPrimary(ctx, workspace, account.ID)
	if err != nil {
		return nil, nil, NewAppError(ErrorCodeWorkspaceAlreadyExists, http.StatusConflict, "failed to create workspace with primary account", err)
	}

	return workspace, membership, nil
}

func workspaceSlug(email string, id uint64) string {
	base := strings.ToLower(strings.TrimSpace(email))
	base = strings.NewReplacer("@", "-", ".", "-").Replace(base)
	if base == "" {
		base = "workspace"
	}
	return fmt.Sprintf("%s-%d", base, id)
}
