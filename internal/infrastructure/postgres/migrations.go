package postgres

import (
	"log"

	"gorm.io/gorm"
)

// CreateCloudAccountsTable creates the cloud_accounts table
func CreateCloudAccountsTable(db *gorm.DB) error {
	sql := `
	CREATE TABLE IF NOT EXISTS cloud_accounts (
		id BIGSERIAL PRIMARY KEY,
		provider_type VARCHAR(50) NOT NULL,
		provider_account_id VARCHAR(255) NOT NULL,
		email VARCHAR(255) NOT NULL,
		refresh_token TEXT NOT NULL,
		granted_scopes TEXT,
		status VARCHAR(50) NOT NULL DEFAULT 'active',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(provider_type, provider_account_id)
	);
	`

	if err := db.Exec(sql).Error; err != nil {
		log.Printf("Error creating cloud_accounts table: %v", err)
		return err
	}

	log.Println("cloud_accounts table created successfully")
	return nil
}

// Migrate runs all database migrations
func Migrate(db *gorm.DB) error {
	return CreateCloudAccountsTable(db)
}
