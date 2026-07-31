package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
	"cumulus/internal/utils"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService      *usecase.AuthService
	accountService   *usecase.AccountService
	workspaceService *usecase.WorkspaceService
	quotaService     *usecase.QuotaService
}

func NewAuthHandler(authService *usecase.AuthService, accountService *usecase.AccountService, workspaceService *usecase.WorkspaceService, quotaService *usecase.QuotaService) *AuthHandler {
	return &AuthHandler{
		authService:      authService,
		accountService:   accountService,
		workspaceService: workspaceService,
		quotaService:     quotaService,
	}
}

func (h *AuthHandler) StartGoogleLogin(c *gin.Context) {
	workspaceIDStr := c.Query("workspace_id")
	var workspaceID uint64
	if workspaceIDStr != "" {
		fmt.Sscanf(workspaceIDStr, "%d", &workspaceID)
	}

	uiPort := "3000"
	referer := c.Request.Referer()
	if strings.Contains(referer, ":8080") || c.Query("frontend") == "8080" {
		uiPort = "8080"
	}

	redirectURL, err := h.authService.StartGoogleLoginWithContext(c.Request.Context(), workspaceID, uiPort)
	if err != nil {
		writeError(c, err)
		return
	}

	c.Redirect(http.StatusFound, redirectURL)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	if callbackError := strings.TrimSpace(c.Query("error")); callbackError != "" {
		writeError(c, usecase.NewAppError(usecase.ErrorCodeAuthorizationDenied, http.StatusBadRequest, callbackError, nil))
		return
	}

	state := c.Query("state")
	targetWS, uiPort := utils.ParseStateContext(state)

	result, err := h.authService.HandleGoogleCallback(c.Request.Context(), c.Query("code"), state)
	if err != nil {
		writeError(c, err)
		return
	}

	account, err := h.accountService.SaveGoogleAccount(c.Request.Context(), result)
	if err != nil {
		writeError(c, err)
		return
	}

	var workspace *domain.Workspace
	var quota *domain.CloudQuota
	var attachError string

	if targetWS > 0 {
		_, quota, err = h.workspaceService.ConnectSecondaryAccount(c.Request.Context(), targetWS, account)
		if err != nil {
			if appErr, ok := err.(*usecase.AppError); ok && appErr.Code == usecase.ErrorCodeCloudAccountAlreadyAttached {
				attachError = "already_attached"
				ws, _, _ := h.workspaceService.GetWorkspaceStorageNodes(c.Request.Context(), targetWS)
				workspace = ws
			} else {
				writeError(c, err)
				return
			}
		} else {
			ws, _, _ := h.workspaceService.GetWorkspaceStorageNodes(c.Request.Context(), targetWS)
			workspace = ws
		}
	} else {
		workspace, _, quota, err = h.workspaceService.ResolveLoginWorkspace(c.Request.Context(), account)
		if err != nil {
			writeError(c, err)
			return
		}
	}

	if quota == nil && account != nil {
		if _, err := h.quotaService.SyncAccountQuota(c.Request.Context(), account); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to sync quota for account %d: %v\n", account.ID, err)
		}
	}

	printSuccess(result, account)

	wsID := uint64(1)
	if workspace != nil {
		wsID = workspace.ID
	} else if targetWS > 0 {
		wsID = targetWS
	}

	baseURL := "http://localhost:3000"
	if uiPort == "8080" {
		baseURL = "http://localhost:8080/dashboard"
	}

	cleanBase := strings.TrimSuffix(baseURL, "/dashboard")
	cleanBase = strings.TrimSuffix(cleanBase, "/")

	redirectPath := fmt.Sprintf("%s/?workspace_id=%d&connected=true", cleanBase, wsID)
	if attachError != "" {
		redirectPath = fmt.Sprintf("%s/?workspace_id=%d&error=%s&email=%s", cleanBase, wsID, attachError, account.Email)
	}

	c.Redirect(http.StatusFound, redirectPath)
}

func printSuccess(result *usecase.AuthResult, account *domain.CloudAccount) {
	fmt.Fprintln(os.Stdout, "Authenticated successfully")
	fmt.Fprintf(os.Stdout, "Email: %s\n", result.Email)
	fmt.Fprintf(os.Stdout, "Google Account ID: %s\n", result.GoogleAccountID)
	fmt.Fprintf(os.Stdout, "Granted scopes: %s\n", strings.Join(result.GrantedScopes, ", "))
	fmt.Fprintf(os.Stdout, "Refresh token received: %t\n", result.RefreshTokenReceived)
	if account != nil {
		fmt.Fprintf(os.Stdout, "Account ID: %d\n", account.ID)
	}
}

func writeError(c *gin.Context, err error) {
	appErr, ok := err.(*usecase.AppError)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}

	statusCode := appErr.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, gin.H{
		"error":   appErr.Code,
		"message": appErr.Message,
	})
}

func (h *AuthHandler) ConnectSecondaryGoogleAccount(c *gin.Context) {
	workspaceIDStr := c.Param("workspaceID")
	var workspaceID uint64
	if _, err := fmt.Sscanf(workspaceIDStr, "%d", &workspaceID); err != nil || workspaceID == 0 {
		writeError(c, usecase.NewAppError(usecase.ErrorCodeWorkspaceNotFound, http.StatusBadRequest, "invalid workspace ID", nil))
		return
	}

	var req struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		writeError(c, usecase.NewAppError(usecase.ErrorCodeMissingCode, http.StatusBadRequest, "missing authorization code in body", nil))
		return
	}

	result, err := h.authService.HandleGoogleCallback(c.Request.Context(), req.Code, req.State)
	if err != nil {
		writeError(c, err)
		return
	}

	account, err := h.accountService.SaveGoogleAccount(c.Request.Context(), result)
	if err != nil {
		writeError(c, err)
		return
	}

	membership, quota, err := h.workspaceService.ConnectSecondaryAccount(c.Request.Context(), workspaceID, account)
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Secondary Google account connected successfully",
		"workspace_id": workspaceID,
		"account_id":   account.ID,
		"role":         membership.Role,
		"status":       membership.Status,
		"quota":        quota,
	})
}

func (h *AuthHandler) DisconnectAccount(c *gin.Context) {
	workspaceIDStr := c.Param("workspaceID")
	cloudAccountIDStr := c.Param("cloudAccountID")
	var workspaceID, cloudAccountID uint64
	if _, err := fmt.Sscanf(workspaceIDStr, "%d", &workspaceID); err != nil || workspaceID == 0 {
		writeError(c, usecase.NewAppError(usecase.ErrorCodeWorkspaceNotFound, http.StatusBadRequest, "invalid workspace ID", nil))
		return
	}
	if _, err := fmt.Sscanf(cloudAccountIDStr, "%d", &cloudAccountID); err != nil || cloudAccountID == 0 {
		writeError(c, usecase.NewAppError(usecase.ErrorCodeWorkspaceNotFound, http.StatusBadRequest, "invalid cloud account ID", nil))
		return
	}

	if err := h.workspaceService.DisconnectAccount(c.Request.Context(), workspaceID, cloudAccountID); err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "Cloud account disconnected successfully",
		"workspace_id":     workspaceID,
		"cloud_account_id": cloudAccountID,
		"status":           domain.WorkspaceConnectionStatusDisconnected,
	})
}
