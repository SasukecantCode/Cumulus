package routes

import (
	"cumulus/internal/transport/http/handlers"
	"github.com/gin-gonic/gin"
)

func Register(router *gin.Engine, authHandler *handlers.AuthHandler) {
	router.GET("/auth/google", authHandler.StartGoogleLogin)
	router.GET("/auth/google/callback", authHandler.GoogleCallback)
}
