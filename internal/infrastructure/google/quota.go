package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cumulus/internal/config"
	"cumulus/internal/domain"
	"cumulus/internal/usecase"
	"golang.org/x/oauth2"
)

const googleDriveAboutURL = "https://www.googleapis.com/drive/v3/about?fields=storageQuota"

type GoogleDriveQuotaProvider struct {
	oauthConfig *oauth2.Config
	httpClient  *http.Client
}

func NewGoogleDriveQuotaProvider(cfg *config.Config) *GoogleDriveQuotaProvider {
	return &GoogleDriveQuotaProvider{
		oauthConfig: cfg.GoogleOAuthConfig(),
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *GoogleDriveQuotaProvider) GetQuota(ctx context.Context, refreshToken string) (domain.QuotaSnapshot, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return domain.QuotaSnapshot{}, usecase.NewAppError(usecase.ErrorCodeQuotaAuthentication, http.StatusUnauthorized, "missing refresh token", nil)
	}

	tokenSource := p.oauthConfig.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := tokenSource.Token()
	if err != nil {
		return domain.QuotaSnapshot{}, classifyQuotaAuthError(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleDriveAboutURL, nil)
	if err != nil {
		return domain.QuotaSnapshot{}, usecase.NewAppError(usecase.ErrorCodeQuotaUnavailable, http.StatusServiceUnavailable, "failed to create quota request", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return domain.QuotaSnapshot{}, classifyQuotaNetworkError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return domain.QuotaSnapshot{}, usecase.NewAppError(usecase.ErrorCodeQuotaUnavailable, http.StatusServiceUnavailable, "failed to read quota response", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return domain.QuotaSnapshot{}, classifyQuotaHTTPError(resp.StatusCode, string(body))
	}

	var payload struct {
		StorageQuota struct {
			Limit string `json:"limit"`
			Usage string `json:"usage"`
		} `json:"storageQuota"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return domain.QuotaSnapshot{}, usecase.NewAppError(usecase.ErrorCodeQuotaInvalidResponse, http.StatusBadGateway, "invalid quota response", err)
	}

	totalBytes, usedBytes, availableBytes, err := parseGoogleQuota(payload.StorageQuota.Limit, payload.StorageQuota.Usage)
	if err != nil {
		return domain.QuotaSnapshot{}, usecase.NewAppError(usecase.ErrorCodeQuotaInvalidResponse, http.StatusBadGateway, "invalid quota response", err)
	}

	return domain.QuotaSnapshot{
		TotalBytes:     totalBytes,
		UsedBytes:      usedBytes,
		AvailableBytes: availableBytes,
		RawResponse:    string(body),
	}, nil
}

func parseGoogleQuota(limitRaw, usageRaw string) (int64, int64, int64, error) {
	limitRaw = strings.TrimSpace(limitRaw)
	usageRaw = strings.TrimSpace(usageRaw)
	if limitRaw == "" || usageRaw == "" {
		return 0, 0, 0, fmt.Errorf("missing storage quota fields")
	}

	totalBytes, err := strconv.ParseInt(limitRaw, 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	usedBytes, err := strconv.ParseInt(usageRaw, 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	availableBytes := totalBytes - usedBytes
	if availableBytes < 0 {
		availableBytes = 0
	}

	return totalBytes, usedBytes, availableBytes, nil
}

func classifyQuotaNetworkError(err error) error {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "timeout") || strings.Contains(message, "context deadline exceeded") {
		return usecase.NewAppError(usecase.ErrorCodeQuotaUnavailable, http.StatusServiceUnavailable, "quota provider unavailable", err)
	}
	return usecase.NewAppError(usecase.ErrorCodeQuotaUnavailable, http.StatusServiceUnavailable, "quota provider unavailable", err)
}

func classifyQuotaAuthError(err error) error {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "invalid_grant") || strings.Contains(message, "unauthorized") {
		return usecase.NewAppError(usecase.ErrorCodeQuotaAuthentication, http.StatusUnauthorized, "quota authentication failed", err)
	}
	return usecase.NewAppError(usecase.ErrorCodeQuotaAuthentication, http.StatusUnauthorized, "quota authentication failed", err)
}

func classifyQuotaHTTPError(statusCode int, body string) error {
	message := strings.TrimSpace(body)
	if message == "" {
		message = fmt.Sprintf("google drive api returned status %d", statusCode)
	}

	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return usecase.NewAppError(usecase.ErrorCodeQuotaAuthentication, statusCode, message, nil)
	case statusCode == http.StatusTooManyRequests:
		return usecase.NewAppError(usecase.ErrorCodeQuotaRateLimited, statusCode, message, nil)
	case statusCode >= 500:
		return usecase.NewAppError(usecase.ErrorCodeQuotaUnavailable, http.StatusServiceUnavailable, message, nil)
	default:
		return usecase.NewAppError(usecase.ErrorCodeQuotaInvalidResponse, http.StatusBadGateway, message, nil)
	}
}
