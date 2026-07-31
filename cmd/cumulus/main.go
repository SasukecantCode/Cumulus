package main

import (
	"log"
	"time"

	"cumulus/internal/config"
	"cumulus/internal/infrastructure/encryption"
	"cumulus/internal/infrastructure/google"
	"cumulus/internal/infrastructure/postgres"
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

	// Initialize database connection
	db, err := postgres.Connect(cfg.Database.URL)
	if err != nil {
		log.Fatal(err)
	}
	cfg.Database.DB = db

	// Run database migrations
	if err := postgres.Migrate(db); err != nil {
		log.Fatal(err)
	}

	// Initialize services and handlers
	stateManager := utils.NewInMemoryStateManager(10 * time.Minute)
	oauthClient := google.NewOAuthClient(cfg)
	authService := usecase.NewAuthService(oauthClient, stateManager)

	// Initialize token encryptor
	encryptor, err := encryption.NewTokenEncryptor(cfg.EncryptionKey)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize repository and account service
	cloudAccountRepo := postgres.NewCloudAccountRepository(db)
	accountService := usecase.NewAccountService(cloudAccountRepo, encryptor)

	authHandler := handlers.NewAuthHandler(authService, accountService)

	router := gin.Default()
	routes.Register(router, authHandler)

	if err := router.Run(":" + cfg.HTTPPort); err != nil {
		log.Fatal(err)
	}
}
