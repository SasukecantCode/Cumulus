package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService    *usecase.AuthService
	accountService *usecase.AccountService
}

func NewAuthHandler(authService *usecase.AuthService, accountService *usecase.AccountService) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		accountService: accountService,
	}
}

func (h *AuthHandler) StartGoogleLogin(c *gin.Context) {
	redirectURL, err := h.authService.StartGoogleLogin(c.Request.Context())
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

	result, err := h.authService.HandleGoogleCallback(c.Request.Context(), c.Query("code"), c.Query("state"))
	if err != nil {
		writeError(c, err)
		return
	}

	// Save the authenticated account to the database
	// Authentication is not complete until persistence succeeds
	account, err := h.accountService.SaveGoogleAccount(c.Request.Context(), result)
	if err != nil {
		writeError(c, err)
		return
	}

	printSuccess(result, account)
	c.String(http.StatusOK, "Authenticated successfully\nEmail: %s\nGoogle Account ID: %s\nGranted scopes: %s\nRefresh token received: %t\nAccount ID: %d\n", result.Email, result.GoogleAccountID, strings.Join(result.GrantedScopes, ", "), result.RefreshTokenReceived, account.ID)
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
