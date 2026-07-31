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

type QuotaService struct {
	quotaRepo domain.QuotaRepository
	provider  domain.QuotaProvider
	encryptor *encryption.TokenEncryptor
}

func NewQuotaService(quotaRepo domain.QuotaRepository, provider domain.QuotaProvider, encryptor *encryption.TokenEncryptor) *QuotaService {
	return &QuotaService{
		quotaRepo: quotaRepo,
		provider:  provider,
		encryptor: encryptor,
	}
}

func (s *QuotaService) SyncAccountQuota(ctx context.Context, account *domain.CloudAccount) (*domain.CloudQuota, error) {
	if account == nil {
		return nil, NewAppError(ErrorCodeQuotaSyncFailed, http.StatusBadRequest, "cloud account is nil", nil)
	}

	refreshToken, err := s.encryptor.Decrypt(account.RefreshToken)
	if err != nil {
		return s.persistFailure(ctx, account, "", NewAppError(ErrorCodeQuotaAuthentication, http.StatusUnauthorized, "failed to decrypt refresh token", err))
	}

	snapshot, err := s.provider.GetQuota(ctx, refreshToken)
	if err != nil {
		return s.persistFailure(ctx, account, "", err)
	}

	quota := &domain.CloudQuota{
		CloudAccountID: account.ID,
		Provider:       account.ProviderType,
		TotalBytes:     snapshot.TotalBytes,
		UsedBytes:      snapshot.UsedBytes,
		AvailableBytes: snapshot.AvailableBytes,
		LastSyncedAt:   time.Now().UTC(),
		SyncStatus:     domain.QuotaSyncStatusSuccess,
		SyncError:      "",
		RawResponse:    snapshot.RawResponse,
	}

	if err := s.quotaRepo.Upsert(ctx, quota); err != nil {
		return nil, NewAppError(ErrorCodeQuotaSyncFailed, http.StatusInternalServerError, "failed to persist quota", err)
	}

	saved, err := s.quotaRepo.FindByCloudAccountID(ctx, account.ID)
	if err != nil {
		return nil, NewAppError(ErrorCodeQuotaSyncFailed, http.StatusInternalServerError, "failed to load saved quota", err)
	}

	return saved, nil
}

func (s *QuotaService) persistFailure(ctx context.Context, account *domain.CloudAccount, rawResponse string, syncErr error) (*domain.CloudQuota, error) {
	if account == nil {
		return nil, NewAppError(ErrorCodeQuotaSyncFailed, http.StatusBadRequest, "cloud account is nil", nil)
	}

	var existing *domain.CloudQuota
	if saved, err := s.quotaRepo.FindByCloudAccountID(ctx, account.ID); err == nil {
		existing = saved
	}

	quota := &domain.CloudQuota{
		CloudAccountID: account.ID,
		Provider:       account.ProviderType,
		LastSyncedAt:   time.Now().UTC(),
		SyncStatus:     domain.QuotaSyncStatusFailed,
		SyncError:      quotaErrorMessage(syncErr),
		RawResponse:    rawResponse,
	}

	if existing != nil {
		quota.TotalBytes = existing.TotalBytes
		quota.UsedBytes = existing.UsedBytes
		quota.AvailableBytes = existing.AvailableBytes
		if quota.RawResponse == "" {
			quota.RawResponse = existing.RawResponse
		}
	}

	if err := s.quotaRepo.Upsert(ctx, quota); err != nil {
		return nil, NewAppError(ErrorCodeQuotaSyncFailed, http.StatusInternalServerError, "failed to persist quota sync failure", err)
	}

	saved, err := s.quotaRepo.FindByCloudAccountID(ctx, account.ID)
	if err != nil {
		return nil, NewAppError(ErrorCodeQuotaSyncFailed, http.StatusInternalServerError, "failed to load saved quota", err)
	}

	return saved, syncErr
}

func quotaErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	if appErr, ok := err.(*AppError); ok {
		return appErr.Message
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = fmt.Sprintf("%T", err)
	}
	return message
}
