package usecase

import (
	"context"

	"cumulus/internal/domain"
)

type StorageService struct {
	checksummer domain.Checksummer
	scheduler   domain.Scheduler
	drive       domain.DriveProvider
	files       domain.FileRepository
}

func NewStorageService(checksummer domain.Checksummer, scheduler domain.Scheduler, drive domain.DriveProvider, files domain.FileRepository) *StorageService {
	return &StorageService{
		checksummer: checksummer,
		scheduler:   scheduler,
		drive:       drive,
		files:       files,
	}
}

func (s *StorageService) Upload(_ context.Context, _ []byte, _ string, _ string, _ uint64) error {
	return nil
}
