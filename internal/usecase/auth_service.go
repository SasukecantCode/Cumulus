package usecase

import (
	"context"
	"net/http"
	"strings"

	"cumulus/internal/domain"
	"cumulus/internal/utils"
)

type AuthService struct {
	oauthProvider domain.OAuthProvider
	stateManager  utils.StateManager
}

type AuthResult struct {
	Email                string
	GoogleAccountID      string
	GrantedScopes        []string
	RefreshTokenReceived bool
}

func NewAuthService(oauthProvider domain.OAuthProvider, stateManager utils.StateManager) *AuthService {
	return &AuthService{
		oauthProvider: oauthProvider,
		stateManager:  stateManager,
	}
}

func (s *AuthService) StartGoogleLogin(_ context.Context) (string, error) {
	state, err := s.stateManager.GenerateState()
	if err != nil {
		return "", NewAppError(ErrorCodeOAuthExchangeFailed, http.StatusInternalServerError, "failed to generate OAuth state", err)
	}

	return s.oauthProvider.GetAuthURL(state), nil
}

func (s *AuthService) HandleGoogleCallback(ctx context.Context, code, state string) (*AuthResult, error) {
	if strings.TrimSpace(code) == "" {
		return nil, NewAppError(ErrorCodeMissingCode, http.StatusBadRequest, "missing authorization code", nil)
	}

	if err := s.stateManager.ValidateState(state); err != nil {
		return nil, NewAppError(ErrorCodeInvalidState, http.StatusBadRequest, "invalid or expired state", err)
	}

	tokens, err := s.oauthProvider.ExchangeCode(ctx, code)
	if err != nil {
		return nil, mapOAuthError(err)
	}

	profile, err := s.oauthProvider.GetUserProfile(ctx, tokens.AccessToken)
	if err != nil {
		return nil, NewAppError(ErrorCodeOAuthProfileFailed, http.StatusBadGateway, "failed to fetch Google profile", err)
	}

	return &AuthResult{
		Email:                profile.Email,
		GoogleAccountID:      profile.GoogleAccountID,
		GrantedScopes:        tokens.Scopes,
		RefreshTokenReceived: strings.TrimSpace(tokens.RefreshToken) != "",
	}, nil
}

func mapOAuthError(err error) error {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}

	if strings.Contains(strings.ToLower(err.Error()), "invalid_grant") {
		return NewAppError(ErrorCodeInvalidAuthorization, http.StatusBadRequest, "invalid authorization code", err)
	}

	return NewAppError(ErrorCodeOAuthExchangeFailed, http.StatusBadGateway, "failed to exchange authorization code", err)
}
