package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"cumulus/internal/usecase"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *usecase.AuthService
}

func NewAuthHandler(authService *usecase.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
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

	printSuccess(result)
	c.String(http.StatusOK, "Authenticated successfully\nEmail: %s\nGoogle Account ID: %s\nGranted scopes: %s\nRefresh token received: %t\n", result.Email, result.GoogleAccountID, strings.Join(result.GrantedScopes, ", "), result.RefreshTokenReceived)
}

func printSuccess(result *usecase.AuthResult) {
	fmt.Fprintln(os.Stdout, "Authenticated successfully")
	fmt.Fprintf(os.Stdout, "Email: %s\n", result.Email)
	fmt.Fprintf(os.Stdout, "Google Account ID: %s\n", result.GoogleAccountID)
	fmt.Fprintf(os.Stdout, "Granted scopes: %s\n", strings.Join(result.GrantedScopes, ", "))
	fmt.Fprintf(os.Stdout, "Refresh token received: %t\n", result.RefreshTokenReceived)
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
