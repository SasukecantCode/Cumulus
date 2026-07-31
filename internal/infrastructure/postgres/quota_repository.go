package postgres

import (
	"context"
	"errors"

	"cumulus/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type QuotaRepository struct {
	db *gorm.DB
}

func NewQuotaRepository(db *gorm.DB) *QuotaRepository {
	return &QuotaRepository{db: db}
}

func (r *QuotaRepository) Upsert(ctx context.Context, quota *domain.CloudQuota) error {
	if quota == nil {
		return errors.New("quota cannot be nil")
	}

	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "cloud_account_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"provider",
				"total_bytes",
				"used_bytes",
				"available_bytes",
				"last_synced_at",
				"sync_status",
				"sync_error",
				"raw_response",
				"updated_at",
			}),
		}).
		Create(quota)

	return result.Error
}

func (r *QuotaRepository) FindByCloudAccountID(ctx context.Context, cloudAccountID uint64) (*domain.CloudQuota, error) {
	var quota domain.CloudQuota
	result := r.db.WithContext(ctx).
		Where("cloud_account_id = ?", cloudAccountID).
		First(&quota)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}

	return &quota, nil
}
