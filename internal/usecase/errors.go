package usecase

import "fmt"

type ErrorCode string

const (
	ErrorCodeInvalidState         ErrorCode = "invalid_state"
	ErrorCodeAuthorizationDenied  ErrorCode = "authorization_denied"
	ErrorCodeMissingCode          ErrorCode = "missing_code"
	ErrorCodeOAuthExchangeFailed  ErrorCode = "oauth_exchange_failed"
	ErrorCodeOAuthProfileFailed   ErrorCode = "oauth_profile_failed"
	ErrorCodeGoogleUnavailable    ErrorCode = "google_unavailable"
	ErrorCodeInvalidAuthorization ErrorCode = "invalid_authorization_code"
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
