package usecase

import (
	"context"

	"cumulus/internal/domain"
)

type MetadataService struct {
	users    domain.UserRepository
	accounts domain.GoogleAccountRepository
	files    domain.FileRepository
	folders  domain.FolderRepository
	policies domain.StoragePolicyRepository
}

func NewMetadataService(
	users domain.UserRepository,
	accounts domain.GoogleAccountRepository,
	files domain.FileRepository,
	folders domain.FolderRepository,
	policies domain.StoragePolicyRepository,
) *MetadataService {
	return &MetadataService{
		users:    users,
		accounts: accounts,
		files:    files,
		folders:  folders,
		policies: policies,
	}
}

func (s *MetadataService) HealthCheck(_ context.Context) error {
	return nil
}
