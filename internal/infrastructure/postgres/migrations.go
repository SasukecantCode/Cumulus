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

// CreateCloudQuotasTable creates the cloud_quotas table
func CreateCloudQuotasTable(db *gorm.DB) error {
	sql := `
	CREATE TABLE IF NOT EXISTS cloud_quotas (
		id BIGSERIAL PRIMARY KEY,
		cloud_account_id BIGINT NOT NULL UNIQUE,
		provider VARCHAR(50) NOT NULL,
		total_bytes BIGINT NOT NULL DEFAULT 0,
		used_bytes BIGINT NOT NULL DEFAULT 0,
		available_bytes BIGINT NOT NULL DEFAULT 0,
		last_synced_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		sync_status VARCHAR(50) NOT NULL DEFAULT 'pending',
		sync_error TEXT NOT NULL DEFAULT '',
		raw_response TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_cloud_quotas_cloud_account
			FOREIGN KEY (cloud_account_id)
			REFERENCES cloud_accounts(id)
			ON DELETE CASCADE
	);
	`

	if err := db.Exec(sql).Error; err != nil {
		log.Printf("Error creating cloud_quotas table: %v", err)
		return err
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_cloud_quotas_provider ON cloud_quotas(provider);`).Error; err != nil {
		log.Printf("Error creating cloud_quotas provider index: %v", err)
		return err
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_cloud_quotas_status ON cloud_quotas(sync_status);`).Error; err != nil {
		log.Printf("Error creating cloud_quotas status index: %v", err)
		return err
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_cloud_quotas_last_synced_at ON cloud_quotas(last_synced_at);`).Error; err != nil {
		log.Printf("Error creating cloud_quotas last_synced_at index: %v", err)
		return err
	}

	log.Println("cloud_quotas table created successfully")
	return nil
}

// Migrate runs all database migrations
func Migrate(db *gorm.DB) error {
	if err := CreateCloudAccountsTable(db); err != nil {
		return err
	}
	return CreateCloudQuotasTable(db)
}
