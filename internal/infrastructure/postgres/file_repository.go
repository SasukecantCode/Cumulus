package postgres

import (
	"context"
	"errors"
	"time"

	"cumulus/internal/domain"
	"gorm.io/gorm"
)

type FileRepository struct {
	db *gorm.DB
}

func NewFileRepository(db *gorm.DB) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Create(ctx context.Context, file *domain.File) error {
	return r.db.WithContext(ctx).Create(file).Error
}

func (r *FileRepository) FindByID(ctx context.Context, id uint64) (*domain.File, error) {
	var file domain.File
	err := r.db.WithContext(ctx).First(&file, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &file, nil
}

func (r *FileRepository) FindByWorkspace(ctx context.Context, workspaceID uint64) ([]*domain.File, error) {
	var files []*domain.File
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("id DESC").Find(&files).Error
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (r *FileRepository) FindByProviderFileID(ctx context.Context, providerFileID string) (*domain.File, error) {
	var file domain.File
	err := r.db.WithContext(ctx).Where("provider_file_id = ?", providerFileID).First(&file).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &file, nil
}

func (r *FileRepository) UpdateMetadata(ctx context.Context, file *domain.File) error {
	return r.db.WithContext(ctx).Model(file).Updates(map[string]any{
		"provider_file_id":  file.ProviderFileID,
		"filename":          file.Filename,
		"original_filename": file.OriginalFilename,
		"mime_type":         file.MimeType,
		"extension":         file.Extension,
		"size_bytes":        file.SizeBytes,
		"checksum":          file.Checksum,
		"status":            file.Status,
		"updated_at":        file.UpdatedAt,
	}).Error
}

func (r *FileRepository) UpdateStatus(ctx context.Context, id uint64, status domain.FileStatus) error {
	return r.db.WithContext(ctx).Model(&domain.File{}).Where("id = ?", id).Update("status", status).Error
}

func (r *FileRepository) ListWorkspaceFiles(ctx context.Context, filter domain.FileListFilter) ([]*domain.File, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.File{}).Where("workspace_id = ?", filter.WorkspaceID)

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	} else {
		query = query.Where("status != ?", domain.FileStatusDeleted)
	}

	if filter.Filename != "" {
		query = query.Where("filename ILIKE ?", "%"+filter.Filename+"%")
	}

	if filter.MimeType != "" {
		query = query.Where("mime_type = ?", filter.MimeType)
	}

	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var files []*domain.File
	err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&files).Error
	if err != nil {
		return nil, 0, err
	}

	return files, totalCount, nil
}

func (r *FileRepository) SoftDelete(ctx context.Context, id uint64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&domain.File{}).Where("id = ?", id).Updates(map[string]any{
		"status":     domain.FileStatusDeleted,
		"deleted_at": now,
		"updated_at": now,
	}).Error
}

func (r *FileRepository) Rename(ctx context.Context, id uint64, newFilename string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&domain.File{}).Where("id = ?", id).Updates(map[string]any{
		"filename":   newFilename,
		"updated_at": now,
	}).Error
}

func (r *FileRepository) Move(ctx context.Context, id uint64, targetParentID *uint64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&domain.File{}).Where("id = ?", id).Updates(map[string]any{
		"parent_id":  targetParentID,
		"updated_at": now,
	}).Error
}

func (r *FileRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&domain.File{}, id).Error
}
