package postgres

import (
	"context"

	"cumulus/internal/domain"
	"gorm.io/gorm"
)

type FileChunkRepository struct {
	db *gorm.DB
}

func NewFileChunkRepository(db *gorm.DB) *FileChunkRepository {
	return &FileChunkRepository{db: db}
}

func (r *FileChunkRepository) Create(ctx context.Context, chunk *domain.FileChunk) error {
	return r.db.WithContext(ctx).Create(chunk).Error
}

func (r *FileChunkRepository) CreateBatch(ctx context.Context, chunks []*domain.FileChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&chunks).Error
}

func (r *FileChunkRepository) FindByFileID(ctx context.Context, fileID uint64) ([]*domain.FileChunk, error) {
	var chunks []*domain.FileChunk
	err := r.db.WithContext(ctx).
		Where("file_id = ?", fileID).
		Order("chunk_index ASC").
		Find(&chunks).Error
	if err != nil {
		return nil, err
	}
	return chunks, nil
}

func (r *FileChunkRepository) DeleteByFileID(ctx context.Context, fileID uint64) error {
	return r.db.WithContext(ctx).
		Where("file_id = ?", fileID).
		Delete(&domain.FileChunk{}).Error
}

func (r *FileChunkRepository) UpdateStatus(ctx context.Context, id uint64, status domain.FileStatus) error {
	return r.db.WithContext(ctx).
		Model(&domain.FileChunk{}).
		Where("id = ?", id).
		Update("status", status).Error
}
