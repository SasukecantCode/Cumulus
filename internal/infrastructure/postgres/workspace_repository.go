package postgres

import (
	"context"
	"errors"
	"time"

	"cumulus/internal/domain"
	"gorm.io/gorm"
)

type WorkspaceRepository struct {
	db *gorm.DB
}

func NewWorkspaceRepository(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) Create(ctx context.Context, workspace *domain.Workspace) error {
	if workspace == nil {
		return errors.New("workspace cannot be nil")
	}
	return r.db.WithContext(ctx).Create(workspace).Error
}

func (r *WorkspaceRepository) CreateWorkspaceWithPrimary(ctx context.Context, workspace *domain.Workspace, cloudAccountID uint64) (*domain.WorkspaceCloudAccount, error) {
	if workspace == nil {
		return nil, errors.New("workspace cannot be nil")
	}

	var membership domain.WorkspaceCloudAccount
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(workspace).Error; err != nil {
			return err
		}

		membership = domain.WorkspaceCloudAccount{
			WorkspaceID:    workspace.ID,
			CloudAccountID: cloudAccountID,
			Role:           domain.WorkspaceRolePrimary,
			Status:         domain.WorkspaceConnectionStatusConnected,
			ConnectedAt:    time.Now().UTC(),
		}

		return tx.Create(&membership).Error
	})
	if err != nil {
		return nil, err
	}
	return &membership, nil
}

func (r *WorkspaceRepository) FindByID(ctx context.Context, id uint64) (*domain.Workspace, error) {
	var workspace domain.Workspace
	result := r.db.WithContext(ctx).First(&workspace, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &workspace, nil
}

func (r *WorkspaceRepository) FindBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	var workspace domain.Workspace
	result := r.db.WithContext(ctx).Where("slug = ?", slug).First(&workspace)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &workspace, nil
}

func (r *WorkspaceRepository) FindByCloudAccountID(ctx context.Context, cloudAccountID uint64) (*domain.Workspace, *domain.WorkspaceCloudAccount, error) {
	var membership domain.WorkspaceCloudAccount
	result := r.db.WithContext(ctx).Where("cloud_account_id = ? AND status = ?", cloudAccountID, domain.WorkspaceConnectionStatusConnected).First(&membership)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}
	if result.Error != nil {
		return nil, nil, result.Error
	}
	workspace, err := r.FindByID(ctx, membership.WorkspaceID)
	if err != nil {
		return nil, nil, err
	}
	return workspace, &membership, nil
}

func (r *WorkspaceRepository) FindMembership(ctx context.Context, cloudAccountID uint64) (*domain.WorkspaceCloudAccount, error) {
	var membership domain.WorkspaceCloudAccount
	result := r.db.WithContext(ctx).Where("cloud_account_id = ?", cloudAccountID).First(&membership)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &membership, nil
}

func (r *WorkspaceRepository) AttachAccount(ctx context.Context, membership *domain.WorkspaceCloudAccount) error {
	if membership == nil {
		return errors.New("membership cannot be nil")
	}
	return r.db.WithContext(ctx).Create(membership).Error
}

func (r *WorkspaceRepository) AttachSecondary(ctx context.Context, membership *domain.WorkspaceCloudAccount) error {
	if membership == nil {
		return errors.New("membership cannot be nil")
	}
	membership.Role = domain.WorkspaceRoleSecondary
	membership.Status = domain.WorkspaceConnectionStatusConnected
	return r.db.WithContext(ctx).Create(membership).Error
}

func (r *WorkspaceRepository) ReconnectMembership(ctx context.Context, membershipID uint64, workspaceID uint64, role domain.WorkspaceRole) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&domain.WorkspaceCloudAccount{}).
		Where("id = ?", membershipID).
		Updates(map[string]any{
			"workspace_id":    workspaceID,
			"role":            role,
			"status":          domain.WorkspaceConnectionStatusConnected,
			"connected_at":    now,
			"disconnected_at": nil,
			"updated_at":      now,
		}).Error
}

func (r *WorkspaceRepository) DisconnectAccount(ctx context.Context, workspaceID, cloudAccountID uint64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&domain.WorkspaceCloudAccount{}).
		Where("workspace_id = ? AND cloud_account_id = ? AND role = ?", workspaceID, cloudAccountID, domain.WorkspaceRoleSecondary).
		Updates(map[string]any{
			"status":          domain.WorkspaceConnectionStatusDisconnected,
			"disconnected_at": now,
			"updated_at":      now,
		}).Error
}

func (r *WorkspaceRepository) GetActiveAccounts(ctx context.Context, workspaceID uint64) ([]*domain.CloudAccount, error) {
	var accounts []*domain.CloudAccount
	err := r.db.WithContext(ctx).
		Table("cloud_accounts").
		Select("cloud_accounts.*").
		Joins("JOIN workspace_cloud_accounts ON workspace_cloud_accounts.cloud_account_id = cloud_accounts.id").
		Where("workspace_cloud_accounts.workspace_id = ? AND workspace_cloud_accounts.status = ?", workspaceID, domain.WorkspaceConnectionStatusConnected).
		Order("workspace_cloud_accounts.role ASC, workspace_cloud_accounts.created_at ASC").
		Scan(&accounts).Error
	return accounts, err
}

type storageNodeRow struct {
	Role                domain.WorkspaceRole             `gorm:"column:wca_role"`
	ConnectionStatus    domain.WorkspaceConnectionStatus `gorm:"column:wca_status"`
	AccountID           uint64                           `gorm:"column:ca_id"`
	ProviderType        string                           `gorm:"column:ca_provider_type"`
	ProviderAccountID   string                           `gorm:"column:ca_provider_account_id"`
	Email               string                           `gorm:"column:ca_email"`
	RefreshToken        string                           `gorm:"column:ca_refresh_token"`
	GrantedScopes       string                           `gorm:"column:ca_granted_scopes"`
	AccountStatus       domain.AccountStatus             `gorm:"column:ca_status"`
	AccountCreatedAt    time.Time                        `gorm:"column:ca_created_at"`
	AccountUpdatedAt    time.Time                        `gorm:"column:ca_updated_at"`
	QuotaID             uint64                           `gorm:"column:cq_id"`
	QuotaCloudAccountID uint64                           `gorm:"column:cq_cloud_account_id"`
	QuotaProvider       string                           `gorm:"column:cq_provider"`
	TotalBytes          int64                            `gorm:"column:cq_total_bytes"`
	UsedBytes           int64                            `gorm:"column:cq_used_bytes"`
	AvailableBytes      int64                            `gorm:"column:cq_available_bytes"`
	LastSyncedAt        time.Time                        `gorm:"column:cq_last_synced_at"`
	QuotaSyncStatus     domain.QuotaSyncStatus           `gorm:"column:cq_sync_status"`
	SyncError           string                           `gorm:"column:cq_sync_error"`
	RawResponse         string                           `gorm:"column:cq_raw_response"`
	QuotaCreatedAt      time.Time                        `gorm:"column:cq_created_at"`
	QuotaUpdatedAt      time.Time                        `gorm:"column:cq_updated_at"`
}

func (r *WorkspaceRepository) GetActiveStorageNodes(ctx context.Context, workspaceID uint64) ([]*domain.WorkspaceStorageNode, error) {
	var rows []storageNodeRow
	err := r.db.WithContext(ctx).
		Table("workspace_cloud_accounts wca").
		Select(`
			wca.role AS wca_role,
			wca.status AS wca_status,
			ca.id AS ca_id,
			ca.provider_type AS ca_provider_type,
			ca.provider_account_id AS ca_provider_account_id,
			ca.email AS ca_email,
			ca.refresh_token AS ca_refresh_token,
			ca.granted_scopes AS ca_granted_scopes,
			ca.status AS ca_status,
			ca.created_at AS ca_created_at,
			ca.updated_at AS ca_updated_at,
			cq.id AS cq_id,
			cq.cloud_account_id AS cq_cloud_account_id,
			cq.provider AS cq_provider,
			cq.total_bytes AS cq_total_bytes,
			cq.used_bytes AS cq_used_bytes,
			cq.available_bytes AS cq_available_bytes,
			cq.last_synced_at AS cq_last_synced_at,
			cq.sync_status AS cq_sync_status,
			cq.sync_error AS cq_sync_error,
			cq.raw_response AS cq_raw_response,
			cq.created_at AS cq_created_at,
			cq.updated_at AS cq_updated_at
		`).
		Joins("JOIN cloud_accounts ca ON ca.id = wca.cloud_account_id").
		Joins("LEFT JOIN cloud_quotas cq ON cq.cloud_account_id = ca.id").
		Where("wca.workspace_id = ? AND wca.status = ? AND ca.status = ?",
			workspaceID,
			domain.WorkspaceConnectionStatusConnected,
			domain.AccountStatusActive,
		).
		Order("wca.role ASC, wca.created_at ASC").
		Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	nodes := make([]*domain.WorkspaceStorageNode, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, &domain.WorkspaceStorageNode{
			Role:   row.Role,
			Status: row.ConnectionStatus,
			Account: &domain.CloudAccount{
				ID:                row.AccountID,
				ProviderType:      row.ProviderType,
				ProviderAccountID: row.ProviderAccountID,
				Email:             row.Email,
				RefreshToken:      row.RefreshToken,
				GrantedScopes:     row.GrantedScopes,
				Status:            row.AccountStatus,
				CreatedAt:         row.AccountCreatedAt,
				UpdatedAt:         row.AccountUpdatedAt,
			},
			Quota: &domain.CloudQuota{
				ID:             row.QuotaID,
				CloudAccountID: row.QuotaCloudAccountID,
				Provider:       row.QuotaProvider,
				TotalBytes:     row.TotalBytes,
				UsedBytes:      row.UsedBytes,
				AvailableBytes: row.AvailableBytes,
				LastSyncedAt:   row.LastSyncedAt,
				SyncStatus:     row.QuotaSyncStatus,
				SyncError:      row.SyncError,
				RawResponse:    row.RawResponse,
				CreatedAt:      row.QuotaCreatedAt,
				UpdatedAt:      row.QuotaUpdatedAt,
			},
		})
	}

	return nodes, nil
}
