package main

import (
	"log"
	"time"

	"cumulus/internal/config"
	"cumulus/internal/infrastructure/encryption"
	"cumulus/internal/infrastructure/google"
	"cumulus/internal/infrastructure/postgres"
	"cumulus/internal/infrastructure/scheduler"

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
	googleQuotaProvider := google.NewGoogleDriveQuotaProvider(cfg)
	authService := usecase.NewAuthService(oauthClient, stateManager)

	// Initialize token encryptor
	encryptor, err := encryption.NewTokenEncryptor(cfg.EncryptionKey)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize quota provider registry
	quotaRegistry := usecase.NewMemoryQuotaProviderRegistry()
	quotaRegistry.Register("google", googleQuotaProvider)

	// Initialize storage provider registry
	googleDriveStorageProvider := google.NewGoogleDriveStorageProvider(cfg, encryptor)
	storageRegistry := usecase.NewMemoryStorageProviderRegistry()
	storageRegistry.Register("google", googleDriveStorageProvider)

	// Initialize repository and account service
	cloudAccountRepo := postgres.NewCloudAccountRepository(db)
	accountService := usecase.NewAccountService(cloudAccountRepo, encryptor)
	workspaceRepo := postgres.NewWorkspaceRepository(db)
	quotaRepo := postgres.NewQuotaRepository(db)
	quotaService := usecase.NewQuotaService(quotaRepo, quotaRegistry, encryptor)
	workspaceService := usecase.NewWorkspaceService(workspaceRepo, quotaService, encryptor)

	fileRepo := postgres.NewFileRepository(db)
	chunkRepo := postgres.NewFileChunkRepository(db)
	settingsRepo := postgres.NewWorkspaceSettingsRepository(db)
	uploadScheduler := scheduler.NewUploadScheduler(workspaceRepo, settingsRepo)
	uploadService := usecase.NewUploadService(workspaceRepo, fileRepo, chunkRepo, storageRegistry, uploadScheduler)
	fileService := usecase.NewFileService(workspaceRepo, fileRepo, chunkRepo, cloudAccountRepo, storageRegistry)

	authHandler := handlers.NewAuthHandler(authService, accountService, workspaceService, quotaService)
	uploadHandler := handlers.NewUploadHandler(uploadService)
	fileHandler := handlers.NewFileHandler(fileService)

	router := gin.Default()
	routes.Register(router, authHandler, uploadHandler, fileHandler)

	if err := router.Run(":" + cfg.HTTPPort); err != nil {
		log.Fatal(err)
	}
}
