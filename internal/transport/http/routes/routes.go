package routes

import (
	"cumulus/internal/transport/http/handlers"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func Register(router *gin.Engine, authHandler *handlers.AuthHandler) {
	router.Use(CORSMiddleware())

	router.GET("/", authHandler.ServeDashboard)
	router.GET("/dashboard", authHandler.ServeDashboard)
	router.GET("/api/workspaces/:workspaceID/nodes", authHandler.GetWorkspaceNodesHandler)
	router.GET("/auth/google", authHandler.StartGoogleLogin)
	router.GET("/auth/google/callback", authHandler.GoogleCallback)
	router.POST("/workspaces/:workspaceID/accounts/google", authHandler.ConnectSecondaryGoogleAccount)
	router.DELETE("/workspaces/:workspaceID/accounts/:cloudAccountID", authHandler.DisconnectAccount)
}
