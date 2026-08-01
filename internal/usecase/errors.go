package usecase

import "fmt"

type ErrorCode string

const (
	ErrorCodeInvalidState                   ErrorCode = "invalid_state"
	ErrorCodeAuthorizationDenied            ErrorCode = "authorization_denied"
	ErrorCodeMissingCode                    ErrorCode = "missing_code"
	ErrorCodeOAuthExchangeFailed            ErrorCode = "oauth_exchange_failed"
	ErrorCodeOAuthProfileFailed             ErrorCode = "oauth_profile_failed"
	ErrorCodeGoogleUnavailable              ErrorCode = "google_unavailable"
	ErrorCodeInvalidAuthorization           ErrorCode = "invalid_authorization_code"
	ErrorCodeQuotaSyncFailed                ErrorCode = "quota_sync_failed"
	ErrorCodeQuotaUnavailable               ErrorCode = "quota_provider_unavailable"
	ErrorCodeQuotaAuthentication            ErrorCode = "quota_authentication_failed"
	ErrorCodeQuotaRateLimited               ErrorCode = "quota_rate_limited"
	ErrorCodeQuotaInvalidResponse           ErrorCode = "quota_invalid_response"
	ErrorCodeWorkspaceAlreadyExists         ErrorCode = "workspace_already_exists"
	ErrorCodeCloudAccountAlreadyAttached    ErrorCode = "cloud_account_already_attached"
	ErrorCodePrimaryAccountCannotDisconnect ErrorCode = "primary_account_cannot_disconnect"
	ErrorCodeWorkspaceNotFound              ErrorCode = "workspace_not_found"
	ErrorCodePrimaryAccountMissing          ErrorCode = "primary_account_missing"
	ErrorCodeStorageProviderUnavailable     ErrorCode = "storage_provider_unavailable"
	ErrorCodeStorageAuthenticationFailed    ErrorCode = "storage_authentication_failed"
	ErrorCodeFileUploadFailed               ErrorCode = "file_upload_failed"
	ErrorCodeFileTooLarge                   ErrorCode = "file_too_large"
	ErrorCodeInvalidUploadRequest           ErrorCode = "invalid_upload_request"
	ErrorCodeChecksumFailed                 ErrorCode = "checksum_failed"
	ErrorCodeDatabasePersistenceFailed      ErrorCode = "database_persistence_failed"
	ErrorCodeFileNotFound                   ErrorCode = "file_not_found"
	ErrorCodeFileDownloadFailed             ErrorCode = "file_download_failed"
	ErrorCodeFileDeleteFailed               ErrorCode = "file_delete_failed"
	ErrorCodeFileRenameFailed               ErrorCode = "file_rename_failed"
	ErrorCodeFileMoveFailed                 ErrorCode = "file_move_failed"
	ErrorCodeInsufficientStorage            ErrorCode = "insufficient_storage"
)

type AppError struct {
	Code       ErrorCode
	Message    string
	StatusCode int
	Err        error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewAppError(code ErrorCode, statusCode int, message string, err error) *AppError {
	return &AppError{Code: code, StatusCode: statusCode, Message: message, Err: err}
}
