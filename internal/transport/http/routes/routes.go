package routes

import (
	"cumulus/internal/transport/http/handlers"
	"github.com/gin-gonic/gin"
)

func Register(router *gin.Engine, authHandler *handlers.AuthHandler) {
	router.GET("/", func(c *gin.Context) {
		c.String(200, "Cumulus backend is running. Start authentication at /auth/google\n")
	})
	router.GET("/auth/google", authHandler.StartGoogleLogin)
	router.GET("/auth/google/callback", authHandler.GoogleCallback)
	router.POST("/workspaces/:workspaceID/accounts/google", authHandler.ConnectSecondaryGoogleAccount)
	router.DELETE("/workspaces/:workspaceID/accounts/:cloudAccountID", authHandler.DisconnectAccount)
}
