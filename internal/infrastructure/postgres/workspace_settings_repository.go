package postgres

import (
	"context"
	"errors"
	"time"

	"cumulus/internal/domain"
	"gorm.io/gorm"
)

type WorkspaceSettingsRepository struct {
	db *gorm.DB
}

func NewWorkspaceSettingsRepository(db *gorm.DB) *WorkspaceSettingsRepository {
	return &WorkspaceSettingsRepository{db: db}
}

func (r *WorkspaceSettingsRepository) GetByWorkspaceID(ctx context.Context, workspaceID uint64) (*domain.WorkspaceSettings, error) {
	var settings domain.WorkspaceSettings
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return default settings if none exist
			return &domain.WorkspaceSettings{
				WorkspaceID:    workspaceID,
				UploadStrategy: domain.UploadStrategyFillFirst,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		}
		return nil, err
	}
	return &settings, nil
}

func (r *WorkspaceSettingsRepository) Save(ctx context.Context, settings *domain.WorkspaceSettings) error {
	if settings == nil {
		return errors.New("settings cannot be nil")
	}
	settings.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(ctx).Save(settings).Error
}
