package routes

import (
	"net/http"

	"cumulus/internal/transport/http/handlers"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Disposition")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func Register(
	router *gin.Engine,
	authHandler *handlers.AuthHandler,
	uploadHandler *handlers.UploadHandler,
	fileHandler *handlers.FileHandler,
	syncHandler *handlers.SyncHandler,
) {
	router.Use(CORSMiddleware())

	router.GET("/", authHandler.ServeDashboard)
	router.GET("/dashboard", authHandler.ServeDashboard)
	router.GET("/api/workspaces/:workspaceID/nodes", authHandler.GetWorkspaceNodesHandler)
	router.GET("/auth/google", authHandler.StartGoogleLogin)
	router.GET("/auth/google/callback", authHandler.GoogleCallback)
	router.POST("/workspaces/:workspaceID/accounts/google", authHandler.ConnectSecondaryGoogleAccount)
	router.DELETE("/workspaces/:workspaceID/accounts/:cloudAccountID", authHandler.DisconnectAccount)
	router.POST("/workspaces/:workspaceID/files", uploadHandler.UploadFileHandler)

	if fileHandler != nil {
		router.GET("/workspaces/:workspaceID/files", fileHandler.ListFilesHandler)
		router.GET("/workspaces/:workspaceID/files/:fileID", fileHandler.GetFileHandler)
		router.GET("/workspaces/:workspaceID/files/:fileID/download", fileHandler.DownloadFileHandler)
		router.DELETE("/workspaces/:workspaceID/files/:fileID", fileHandler.DeleteFileHandler)
		router.PATCH("/workspaces/:workspaceID/files/:fileID", fileHandler.RenameFileHandler)
		router.PATCH("/workspaces/:workspaceID/files/:fileID/move", fileHandler.MoveFileHandler)
	}

	if syncHandler != nil {
		router.POST("/workspaces/:workspaceID/sync", syncHandler.SyncWorkspaceHandler)
	}
}
