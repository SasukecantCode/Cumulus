package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cumulus/internal/config"
	"cumulus/internal/domain"
	"cumulus/internal/usecase"
	"golang.org/x/oauth2"
)

const userInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"

type OAuthClient struct {
	oauthConfig *oauth2.Config
	httpClient  *http.Client
}

func NewOAuthClient(cfg *config.Config) *OAuthClient {
	return &OAuthClient{
		oauthConfig: cfg.GoogleOAuthConfig(),
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *OAuthClient) GetAuthURL(state string) string {
	return c.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
}

func (c *OAuthClient) ExchangeCode(ctx context.Context, code string) (domain.OAuthTokens, error) {
	token, err := c.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return domain.OAuthTokens{}, classifyOAuthError(err)
	}

	scopes := parseScopes(token.Extra("scope"))
	return domain.OAuthTokens{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiryUnix:   token.Expiry.Unix(),
		Scopes:       scopes,
	}, nil
}

func (c *OAuthClient) GetUserProfile(ctx context.Context, accessToken string) (domain.OAuthUserProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return domain.OAuthUserProfile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.OAuthUserProfile{}, classifyOAuthError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return domain.OAuthUserProfile{}, classifyHTTPError(resp.StatusCode, string(body))
	}

	var payload struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return domain.OAuthUserProfile{}, err
	}

	return domain.OAuthUserProfile{
		GoogleAccountID: payload.ID,
		Email:           payload.Email,
	}, nil
}

func parseScopes(raw any) []string {
	text, _ := raw.(string)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	parts := strings.Fields(text)
	return parts
}

func classifyOAuthError(err error) error {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "invalid_grant") {
		return usecase.NewAppError(usecase.ErrorCodeInvalidAuthorization, http.StatusBadRequest, "invalid authorization code", err)
	}
	if strings.Contains(message, "context deadline exceeded") || strings.Contains(message, "timeout") {
		return usecase.NewAppError(usecase.ErrorCodeGoogleUnavailable, http.StatusServiceUnavailable, "google services unavailable", err)
	}
	return usecase.NewAppError(usecase.ErrorCodeOAuthExchangeFailed, http.StatusBadGateway, "oauth exchange failed", err)
}

func classifyHTTPError(statusCode int, body string) error {
	message := strings.TrimSpace(body)
	if message == "" {
		message = fmt.Sprintf("google api returned status %d", statusCode)
	}

	switch {
	case statusCode == http.StatusBadRequest:
		return usecase.NewAppError(usecase.ErrorCodeInvalidAuthorization, http.StatusBadRequest, "invalid authorization code", nil)
	case statusCode == http.StatusUnauthorized:
		return usecase.NewAppError(usecase.ErrorCodeInvalidAuthorization, http.StatusUnauthorized, "unauthorized access to google profile", nil)
	case statusCode >= 500:
		return usecase.NewAppError(usecase.ErrorCodeGoogleUnavailable, http.StatusServiceUnavailable, message, nil)
	default:
		return usecase.NewAppError(usecase.ErrorCodeOAuthProfileFailed, http.StatusBadGateway, message, nil)
	}
}
