package postgres

import (
	"context"
	"errors"

	"cumulus/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CloudAccountRepository struct {
	db *gorm.DB
}

func NewCloudAccountRepository(db *gorm.DB) *CloudAccountRepository {
	return &CloudAccountRepository{db: db}
}

// CreateOrUpdate creates a new cloud account or updates an existing one based on (provider_type, provider_account_id)
func (r *CloudAccountRepository) CreateOrUpdate(ctx context.Context, account *domain.CloudAccount) error {
	if account == nil {
		return errors.New("account cannot be nil")
	}

	// Upsert: using OnConflict for provider_type + provider_account_id uniqueness
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider_type"}, {Name: "provider_account_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"email", "refresh_token", "granted_scopes", "status", "updated_at"}),
		}).
		Create(account)

	return result.Error
}

// FindByProviderAccount retrieves a cloud account by provider type and provider account ID
func (r *CloudAccountRepository) FindByProviderAccount(ctx context.Context, providerType, providerAccountID string) (*domain.CloudAccount, error) {
	var account domain.CloudAccount
	result := r.db.WithContext(ctx).
		Where("provider_type = ? AND provider_account_id = ?", providerType, providerAccountID).
		First(&account)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil // Not found, return nil without error
	}
	if result.Error != nil {
		return nil, result.Error
	}

	return &account, nil
}

// FindByEmail retrieves all cloud accounts for a given email
func (r *CloudAccountRepository) FindByEmail(ctx context.Context, email string) ([]*domain.CloudAccount, error) {
	var accounts []*domain.CloudAccount
	result := r.db.WithContext(ctx).
		Where("email = ?", email).
		Find(&accounts)

	return accounts, result.Error
}

// Delete removes a cloud account by ID
func (r *CloudAccountRepository) Delete(ctx context.Context, id uint64) error {
	result := r.db.WithContext(ctx).Delete(&domain.CloudAccount{}, id)
	return result.Error
}
