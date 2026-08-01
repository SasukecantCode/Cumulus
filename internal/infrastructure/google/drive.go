package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"cumulus/internal/config"
	"cumulus/internal/domain"
	"cumulus/internal/infrastructure/encryption"
	"cumulus/internal/usecase"
	"golang.org/x/oauth2"
)

const (
	googleDriveUploadURL = "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart&fields=id,name,mimeType,size,createdTime,modifiedTime,headRevisionId"
	googleDriveFilesURL  = "https://www.googleapis.com/drive/v3/files"

	driveListMaxPageSize = 1000
)

type GoogleDriveStorageProvider struct {
	oauthConfig *oauth2.Config
	httpClient  *http.Client
	encryptor   *encryption.TokenEncryptor
}

func NewGoogleDriveStorageProvider(cfg *config.Config, encryptor *encryption.TokenEncryptor) *GoogleDriveStorageProvider {
	return &GoogleDriveStorageProvider{
		oauthConfig: cfg.GoogleOAuthConfig(),
		httpClient:  &http.Client{Timeout: 60 * time.Second},
		encryptor:   encryptor,
	}
}

func (p *GoogleDriveStorageProvider) Upload(ctx context.Context, account *domain.CloudAccount, input domain.StorageUploadInput) (*domain.UploadedObject, error) {
	if account == nil {
		return nil, usecase.NewAppError(usecase.ErrorCodePrimaryAccountMissing, http.StatusBadRequest, "cloud account is nil", nil)
	}

	if strings.TrimSpace(account.RefreshToken) == "" {
		return nil, usecase.NewAppError(usecase.ErrorCodeStorageAuthenticationFailed, http.StatusUnauthorized, "account missing refresh token", nil)
	}

	refreshToken, err := p.encryptor.Decrypt(account.RefreshToken)
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeStorageAuthenticationFailed, http.StatusUnauthorized, "failed to decrypt refresh token", err)
	}

	tokenSource := p.oauthConfig.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := tokenSource.Token()
	if err != nil {
		return nil, classifyGoogleStorageAuthError(err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Metadata Part
	metaHeader := make(textproto.MIMEHeader)
	metaHeader.Set("Content-Type", "application/json; charset=UTF-8")
	metaPart, err := writer.CreatePart(metaHeader)
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to create multipart metadata header", err)
	}
	if err := json.NewEncoder(metaPart).Encode(map[string]string{"name": input.Filename}); err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to encode metadata JSON", err)
	}

	// Media Part
	mediaHeader := make(textproto.MIMEHeader)
	mimeType := input.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	mediaHeader.Set("Content-Type", mimeType)
	mediaPart, err := writer.CreatePart(mediaHeader)
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to create multipart media header", err)
	}

	if input.Reader != nil {
		if _, err := io.Copy(mediaPart, input.Reader); err != nil {
			return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to copy file body to multipart writer", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to close multipart writer", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleDriveUploadURL, body)
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to create upload HTTP request", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/related; boundary=%s", writer.Boundary()))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, classifyGoogleStorageNetworkError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeStorageProviderUnavailable, http.StatusServiceUnavailable, "failed to read upload response", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, classifyGoogleStorageHTTPError(resp.StatusCode, string(respBody))
	}

	var driveResp struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		MimeType       string `json:"mimeType"`
		Size           string `json:"size"`
		CreatedTime    string `json:"createdTime"`
		ModifiedTime   string `json:"modifiedTime"`
		HeadRevisionID string `json:"headRevisionId"`
	}

	if err := json.Unmarshal(respBody, &driveResp); err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusBadGateway, "invalid upload response from Google Drive", err)
	}

	sizeBytes := input.SizeBytes
	if parsedSize, err := strconv.ParseInt(driveResp.Size, 10, 64); err == nil && parsedSize > 0 {
		sizeBytes = parsedSize
	}

	createdTime := time.Now().UTC()
	if parsedCreated, err := time.Parse(time.RFC3339, driveResp.CreatedTime); err == nil {
		createdTime = parsedCreated
	}

	modifiedTime := createdTime
	if parsedModified, err := time.Parse(time.RFC3339, driveResp.ModifiedTime); err == nil {
		modifiedTime = parsedModified
	}

	return &domain.UploadedObject{
		ProviderFileID: driveResp.ID,
		Filename:       driveResp.Name,
		MimeType:       driveResp.MimeType,
		SizeBytes:      sizeBytes,
		CreatedTime:    createdTime,
		ModifiedTime:   modifiedTime,
		ETag:           driveResp.HeadRevisionID,
		RawResponse:    string(respBody),
	}, nil
}

func (p *GoogleDriveStorageProvider) Delete(ctx context.Context, account *domain.CloudAccount, providerFileID string) error {
	if account == nil || strings.TrimSpace(providerFileID) == "" {
		return nil
	}

	refreshToken, err := p.encryptor.Decrypt(account.RefreshToken)
	if err != nil {
		return usecase.NewAppError(usecase.ErrorCodeStorageAuthenticationFailed, http.StatusUnauthorized, "failed to decrypt refresh token", err)
	}

	tokenSource := p.oauthConfig.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := tokenSource.Token()
	if err != nil {
		return classifyGoogleStorageAuthError(err)
	}

	url := fmt.Sprintf("%s/%s", googleDriveFilesURL, providerFileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to create delete HTTP request", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return classifyGoogleStorageNetworkError(err)
	}
	defer resp.Body.Close()

	// Treat HTTP 404 (Not Found) as idempotent success. Deleting an already-deleted object returns nil.
	if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return classifyGoogleStorageHTTPError(resp.StatusCode, string(body))
	}

	return nil

}

func (p *GoogleDriveStorageProvider) Download(ctx context.Context, account *domain.CloudAccount, providerFileID string) (io.ReadCloser, error) {
	if account == nil || strings.TrimSpace(providerFileID) == "" {
		return nil, usecase.NewAppError(usecase.ErrorCodeInvalidUploadRequest, http.StatusBadRequest, "invalid download request parameters", nil)
	}

	token, err := p.accessToken(ctx, account)
	if err != nil {
		return nil, err
	}

	// An imported Google Doc holds no bytes of its own, so alt=media fails on
	// it — natives have to go through /export instead.
	downloadURL := fmt.Sprintf("%s/%s?alt=media", googleDriveFilesURL, providerFileID)
	if meta, metaErr := p.GetMetadata(ctx, account, providerFileID); metaErr == nil && meta != nil {
		if exportMIME, isNative := googleNativeExportMIME(meta.MimeType); isNative {
			downloadURL = fmt.Sprintf("%s/%s/export?mimeType=%s", googleDriveFilesURL, providerFileID, url.QueryEscape(exportMIME))
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to create download HTTP request", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, classifyGoogleStorageNetworkError(err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return nil, classifyGoogleStorageHTTPError(resp.StatusCode, string(body))
	}

	return resp.Body, nil
}

func (p *GoogleDriveStorageProvider) Rename(ctx context.Context, account *domain.CloudAccount, providerFileID string, newName string) error {
	if account == nil || strings.TrimSpace(providerFileID) == "" || strings.TrimSpace(newName) == "" {
		return usecase.NewAppError(usecase.ErrorCodeInvalidUploadRequest, http.StatusBadRequest, "invalid rename parameters", nil)
	}

	refreshToken, err := p.encryptor.Decrypt(account.RefreshToken)
	if err != nil {
		return usecase.NewAppError(usecase.ErrorCodeStorageAuthenticationFailed, http.StatusUnauthorized, "failed to decrypt refresh token", err)
	}

	tokenSource := p.oauthConfig.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := tokenSource.Token()
	if err != nil {
		return classifyGoogleStorageAuthError(err)
	}

	payload, err := json.Marshal(map[string]string{"name": newName})
	if err != nil {
		return usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to encode rename payload", err)
	}

	url := fmt.Sprintf("%s/%s", googleDriveFilesURL, providerFileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		return usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to create rename HTTP request", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return classifyGoogleStorageNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return classifyGoogleStorageHTTPError(resp.StatusCode, string(body))
	}

	return nil
}

func (p *GoogleDriveStorageProvider) Move(ctx context.Context, account *domain.CloudAccount, providerFileID string, targetFolderID string) error {
	if account == nil || strings.TrimSpace(providerFileID) == "" {
		return usecase.NewAppError(usecase.ErrorCodeInvalidUploadRequest, http.StatusBadRequest, "invalid move parameters", nil)
	}

	if strings.TrimSpace(targetFolderID) == "" {
		return nil
	}

	refreshToken, err := p.encryptor.Decrypt(account.RefreshToken)
	if err != nil {
		return usecase.NewAppError(usecase.ErrorCodeStorageAuthenticationFailed, http.StatusUnauthorized, "failed to decrypt refresh token", err)
	}

	tokenSource := p.oauthConfig.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := tokenSource.Token()
	if err != nil {
		return classifyGoogleStorageAuthError(err)
	}

	url := fmt.Sprintf("%s/%s?addParents=%s", googleDriveFilesURL, providerFileID, targetFolderID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, nil)
	if err != nil {
		return usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to create move HTTP request", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return classifyGoogleStorageNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return classifyGoogleStorageHTTPError(resp.StatusCode, string(body))
	}

	return nil
}

func (p *GoogleDriveStorageProvider) GetMetadata(ctx context.Context, account *domain.CloudAccount, providerFileID string) (*domain.StorageFileMetadata, error) {
	if account == nil || strings.TrimSpace(providerFileID) == "" {
		return nil, usecase.NewAppError(usecase.ErrorCodeInvalidUploadRequest, http.StatusBadRequest, "invalid metadata request parameters", nil)
	}

	refreshToken, err := p.encryptor.Decrypt(account.RefreshToken)
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeStorageAuthenticationFailed, http.StatusUnauthorized, "failed to decrypt refresh token", err)
	}

	tokenSource := p.oauthConfig.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := tokenSource.Token()
	if err != nil {
		return nil, classifyGoogleStorageAuthError(err)
	}

	url := fmt.Sprintf("%s/%s?fields=id,name,mimeType,size,createdTime,modifiedTime,headRevisionId", googleDriveFilesURL, providerFileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to create metadata HTTP request", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, classifyGoogleStorageNetworkError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeStorageProviderUnavailable, http.StatusServiceUnavailable, "failed to read metadata response", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, classifyGoogleStorageHTTPError(resp.StatusCode, string(respBody))
	}

	var driveResp struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		MimeType       string `json:"mimeType"`
		Size           string `json:"size"`
		CreatedTime    string `json:"createdTime"`
		ModifiedTime   string `json:"modifiedTime"`
		HeadRevisionID string `json:"headRevisionId"`
	}

	if err := json.Unmarshal(respBody, &driveResp); err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusBadGateway, "invalid metadata response from Google Drive", err)
	}

	var sizeBytes int64
	if parsedSize, err := strconv.ParseInt(driveResp.Size, 10, 64); err == nil {
		sizeBytes = parsedSize
	}

	createdTime := time.Now().UTC()
	if parsedCreated, err := time.Parse(time.RFC3339, driveResp.CreatedTime); err == nil {
		createdTime = parsedCreated
	}

	modifiedTime := createdTime
	if parsedModified, err := time.Parse(time.RFC3339, driveResp.ModifiedTime); err == nil {
		modifiedTime = parsedModified
	}

	return &domain.StorageFileMetadata{
		ProviderFileID: driveResp.ID,
		Filename:       driveResp.Name,
		MimeType:       driveResp.MimeType,
		SizeBytes:      sizeBytes,
		CreatedTime:    createdTime,
		ModifiedTime:   modifiedTime,
		ETag:           driveResp.HeadRevisionID,
	}, nil
}

// List walks the account's Drive one page at a time. Folders and trashed
// items are filtered server-side; everything else the user owns or has been
// given is returned, which is the point — these are the files that were
// already there before Cumulus existed.
func (p *GoogleDriveStorageProvider) List(ctx context.Context, account *domain.CloudAccount, input domain.StorageListInput) (*domain.StorageListPage, error) {
	if account == nil {
		return nil, usecase.NewAppError(usecase.ErrorCodePrimaryAccountMissing, http.StatusBadRequest, "cloud account is nil", nil)
	}

	token, err := p.accessToken(ctx, account)
	if err != nil {
		return nil, err
	}

	pageSize := input.PageSize
	if pageSize <= 0 || pageSize > driveListMaxPageSize {
		pageSize = driveListMaxPageSize
	}

	query := url.Values{}
	query.Set("q", "trashed = false and mimeType != 'application/vnd.google-apps.folder'")
	query.Set("fields", "nextPageToken,files(id,name,mimeType,size,createdTime,modifiedTime,headRevisionId)")
	query.Set("pageSize", strconv.Itoa(pageSize))
	query.Set("spaces", "drive")
	// Shared drives are a different storage pool with their own quota, so
	// they are deliberately out of scope here.
	query.Set("supportsAllDrives", "false")
	if input.PageToken != "" {
		query.Set("pageToken", input.PageToken)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleDriveFilesURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusInternalServerError, "failed to create list HTTP request", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, classifyGoogleStorageNetworkError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeStorageProviderUnavailable, http.StatusServiceUnavailable, "failed to read list response", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, classifyGoogleStorageHTTPError(resp.StatusCode, string(respBody))
	}

	var listResp struct {
		NextPageToken string `json:"nextPageToken"`
		Files         []struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			MimeType       string `json:"mimeType"`
			Size           string `json:"size"`
			CreatedTime    string `json:"createdTime"`
			ModifiedTime   string `json:"modifiedTime"`
			HeadRevisionID string `json:"headRevisionId"`
		} `json:"files"`
	}

	if err := json.Unmarshal(respBody, &listResp); err != nil {
		return nil, usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusBadGateway, "invalid list response from Google Drive", err)
	}

	files := make([]domain.StorageFileMetadata, 0, len(listResp.Files))
	for _, item := range listResp.Files {
		// Google-native documents report no size; they occupy no quota either,
		// so zero is the honest number rather than a missing one.
		var sizeBytes int64
		if parsed, err := strconv.ParseInt(item.Size, 10, 64); err == nil {
			sizeBytes = parsed
		}

		createdTime := time.Now().UTC()
		if parsed, err := time.Parse(time.RFC3339, item.CreatedTime); err == nil {
			createdTime = parsed
		}
		modifiedTime := createdTime
		if parsed, err := time.Parse(time.RFC3339, item.ModifiedTime); err == nil {
			modifiedTime = parsed
		}

		files = append(files, domain.StorageFileMetadata{
			ProviderFileID: item.ID,
			Filename:       item.Name,
			MimeType:       item.MimeType,
			SizeBytes:      sizeBytes,
			CreatedTime:    createdTime,
			ModifiedTime:   modifiedTime,
			ETag:           item.HeadRevisionID,
		})
	}

	return &domain.StorageListPage{Files: files, NextPageToken: listResp.NextPageToken}, nil
}

// accessToken decrypts the stored refresh token and exchanges it. Every call
// that talks to Drive needs this, so it lives in one place.
func (p *GoogleDriveStorageProvider) accessToken(ctx context.Context, account *domain.CloudAccount) (string, error) {
	if strings.TrimSpace(account.RefreshToken) == "" {
		return "", usecase.NewAppError(usecase.ErrorCodeStorageAuthenticationFailed, http.StatusUnauthorized, "account missing refresh token", nil)
	}

	refreshToken, err := p.encryptor.Decrypt(account.RefreshToken)
	if err != nil {
		return "", usecase.NewAppError(usecase.ErrorCodeStorageAuthenticationFailed, http.StatusUnauthorized, "failed to decrypt refresh token", err)
	}

	token, err := p.oauthConfig.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken}).Token()
	if err != nil {
		return "", classifyGoogleStorageAuthError(err)
	}
	return token.AccessToken, nil
}

// googleNativeExportMIME maps a Google-native document to the format it is
// exported as. Docs, Sheets and Slides have no bytes of their own, so
// alt=media returns an error for them and only /export works.
func googleNativeExportMIME(mimeType string) (string, bool) {
	switch mimeType {
	case "application/vnd.google-apps.document":
		return "application/pdf", true
	case "application/vnd.google-apps.spreadsheet":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true
	case "application/vnd.google-apps.presentation":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation", true
	case "application/vnd.google-apps.drawing":
		return "image/png", true
	case "application/vnd.google-apps.script":
		return "application/vnd.google-apps.script+json", true
	}
	if strings.HasPrefix(mimeType, "application/vnd.google-apps.") {
		return "application/pdf", true
	}
	return "", false
}

func classifyGoogleStorageNetworkError(err error) error {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "context deadline exceeded") {
		return usecase.NewAppError(usecase.ErrorCodeStorageProviderUnavailable, http.StatusGatewayTimeout, "google drive upload timed out", err)
	}
	return usecase.NewAppError(usecase.ErrorCodeStorageProviderUnavailable, http.StatusServiceUnavailable, "google drive provider unavailable", err)
}

func classifyGoogleStorageAuthError(err error) error {
	return usecase.NewAppError(usecase.ErrorCodeStorageAuthenticationFailed, http.StatusUnauthorized, "google authentication failed", err)
}

func classifyGoogleStorageHTTPError(statusCode int, body string) error {
	msg := strings.TrimSpace(body)
	if msg == "" {
		msg = fmt.Sprintf("google drive api returned status %d", statusCode)
	}

	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return usecase.NewAppError(usecase.ErrorCodeStorageAuthenticationFailed, statusCode, msg, nil)
	case statusCode == http.StatusTooManyRequests:
		return usecase.NewAppError(usecase.ErrorCodeStorageProviderUnavailable, statusCode, msg, nil)
	case statusCode >= 500:
		return usecase.NewAppError(usecase.ErrorCodeStorageProviderUnavailable, http.StatusBadGateway, msg, nil)
	default:
		return usecase.NewAppError(usecase.ErrorCodeFileUploadFailed, http.StatusBadRequest, msg, nil)
	}
}
