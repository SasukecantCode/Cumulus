package usecase

import (
	"context"
	"encoding/json"
	"log"

	"cumulus/internal/domain"
	"cumulus/internal/infrastructure/encryption"
)

type AccountService struct {
	cloudAccountRepo domain.CloudAccountRepository
	encryptor        *encryption.TokenEncryptor
}

func NewAccountService(cloudAccountRepo domain.CloudAccountRepository, encryptor *encryption.TokenEncryptor) *AccountService {
	return &AccountService{
		cloudAccountRepo: cloudAccountRepo,
		encryptor:        encryptor,
	}
}

// SaveGoogleAccount persists an authenticated Google account to the database
// If the same provider account already exists, it updates the existing record
// The refresh token is encrypted before persistence
func (s *AccountService) SaveGoogleAccount(ctx context.Context, authResult *AuthResult) (*domain.CloudAccount, error) {
	if authResult == nil {
		return nil, NewAppError(ErrorCodeOAuthExchangeFailed, 400, "auth result is nil", nil)
	}

	// Encrypt the refresh token before persistence
	encryptedToken, err := s.encryptor.Encrypt(authResult.RefreshToken)
	if err != nil {
		log.Printf("Error encrypting refresh token: %v", err)
		return nil, NewAppError(ErrorCodeOAuthExchangeFailed, 500, "failed to encrypt refresh token", err)
	}

	// Convert granted scopes to JSON
	scopesJSON, err := json.Marshal(authResult.GrantedScopes)
	if err != nil {
		log.Printf("Error marshaling scopes: %v", err)
		return nil, NewAppError(ErrorCodeOAuthExchangeFailed, 500, "failed to marshal scopes", err)
	}

	// Create or update the cloud account with encrypted token
	account := &domain.CloudAccount{
		ProviderType:      "google",
		ProviderAccountID: authResult.GoogleAccountID,
		Email:             authResult.Email,
		RefreshToken:      encryptedToken,
		GrantedScopes:     string(scopesJSON),
		Status:            domain.AccountStatusActive,
	}

	if err := s.cloudAccountRepo.CreateOrUpdate(ctx, account); err != nil {
		log.Printf("Error saving cloud account: %v", err)
		return nil, NewAppError(ErrorCodeOAuthExchangeFailed, 500, "failed to save account to database", err)
	}

	// Retrieve the saved account (to get the ID if it was just created)
	saved, err := s.cloudAccountRepo.FindByProviderAccount(ctx, "google", authResult.GoogleAccountID)
	if err != nil {
		log.Printf("Error retrieving saved account: %v", err)
		return nil, NewAppError(ErrorCodeOAuthExchangeFailed, 500, "failed to retrieve saved account", err)
	}

	if saved == nil {
		log.Printf("Cloud account not found after save: %s", authResult.GoogleAccountID)
		return nil, NewAppError(ErrorCodeOAuthExchangeFailed, 500, "cloud account not found after save", nil)
	}

	log.Printf("Successfully saved Google account: %d (provider_id=%s, email=%s)", saved.ID, saved.ProviderAccountID, saved.Email)
	return saved, nil
}
