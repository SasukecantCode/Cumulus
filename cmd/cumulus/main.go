package main

import (
	"log"
	"time"

	"cumulus/internal/config"
	"cumulus/internal/infrastructure/google"
	"cumulus/internal/transport/http/handlers"
	"cumulus/internal/transport/http/routes"
	"cumulus/internal/usecase"
	"cumulus/internal/utils"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	stateManager := utils.NewInMemoryStateManager(10 * time.Minute)
	oauthClient := google.NewOAuthClient(cfg)
	authService := usecase.NewAuthService(oauthClient, stateManager)
	authHandler := handlers.NewAuthHandler(authService)

	router := gin.Default()
	routes.Register(router, authHandler)

	if err := router.Run(":" + cfg.HTTPPort); err != nil {
		log.Fatal(err)
	}
}
