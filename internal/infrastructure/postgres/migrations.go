package postgres

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

// CreateWorkspacesTable creates the workspaces and workspace_cloud_accounts tables.
func CreateWorkspacesTable(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS workspaces (
			id BIGSERIAL PRIMARY KEY,
			uuid VARCHAR(64) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) NOT NULL UNIQUE,
			status VARCHAR(50) NOT NULL DEFAULT 'active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`).Error; err != nil {
		log.Printf("Error creating workspaces table: %v", err)
		return err
	}

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS workspace_cloud_accounts (
			id BIGSERIAL PRIMARY KEY,
			workspace_id BIGINT NOT NULL,
			cloud_account_id BIGINT NOT NULL,
			role VARCHAR(50) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'CONNECTED',
			connected_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			disconnected_at TIMESTAMP NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_workspace_cloud_accounts_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
			CONSTRAINT fk_workspace_cloud_accounts_cloud_account FOREIGN KEY (cloud_account_id) REFERENCES cloud_accounts(id) ON DELETE CASCADE
		);
	`).Error; err != nil {
		log.Printf("Error creating workspace_cloud_accounts table: %v", err)
		return err
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_workspace_cloud_accounts_workspace_id ON workspace_cloud_accounts(workspace_id);`).Error; err != nil {
		log.Printf("Error creating workspace_cloud_accounts workspace_id index: %v", err)
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_workspace_cloud_accounts_cloud_account_id ON workspace_cloud_accounts(cloud_account_id);`).Error; err != nil {
		log.Printf("Error creating workspace_cloud_accounts cloud_account_id index: %v", err)
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_workspace_cloud_accounts_role ON workspace_cloud_accounts(role);`).Error; err != nil {
		log.Printf("Error creating workspace_cloud_accounts role index: %v", err)
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_workspace_cloud_accounts_status ON workspace_cloud_accounts(status);`).Error; err != nil {
		log.Printf("Error creating workspace_cloud_accounts status index: %v", err)
		return err
	}

	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_active_attachment ON workspace_cloud_accounts(cloud_account_id) WHERE status = 'CONNECTED';`).Error; err != nil {
		log.Printf("Error creating workspace_cloud_accounts unique attachment index: %v", err)
		return err
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_active_primary_per_workspace ON workspace_cloud_accounts(workspace_id) WHERE role = 'PRIMARY' AND status = 'CONNECTED';`).Error; err != nil {
		log.Printf("Error creating workspace_cloud_accounts unique primary index: %v", err)
		return err
	}

	var count int64
	if err := db.Table("cloud_accounts").Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		log.Println("workspaces table created successfully")
		log.Println("workspace_cloud_accounts table created successfully")
		return nil
	}

	rows, err := db.Raw(`SELECT id, email, created_at, updated_at FROM cloud_accounts ORDER BY id ASC`).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		var email string
		var createdAt, updatedAt any
		if err := rows.Scan(&id, &email, &createdAt, &updatedAt); err != nil {
			return err
		}
		uuid, err := randomWorkspaceUUID()
		if err != nil {
			return err
		}
		slug := fmt.Sprintf("workspace-%d", id)
		if err := db.Exec(`INSERT INTO workspaces (uuid, name, slug, status, created_at, updated_at) VALUES (?, ?, ?, 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) ON CONFLICT (slug) DO NOTHING`, uuid, "Workspace - "+email, slug).Error; err != nil {
			return err
		}
		var workspaceID uint64
		if err := db.Raw(`SELECT id FROM workspaces WHERE slug = ?`, slug).Scan(&workspaceID).Error; err != nil {
			return err
		}
		if err := db.Exec(`INSERT INTO workspace_cloud_accounts (workspace_id, cloud_account_id, role, status, connected_at, created_at, updated_at) VALUES (?, ?, 'PRIMARY', 'CONNECTED', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) ON CONFLICT DO NOTHING`, workspaceID, id).Error; err != nil {
			return err
		}
	}

	log.Println("workspaces table created successfully")
	log.Println("workspace_cloud_accounts table created successfully")
	return nil
}

func randomWorkspaceUUID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// Migrate runs all database migrations
func Migrate(db *gorm.DB) error {
	if err := CreateCloudAccountsTable(db); err != nil {
		return err
	}
	if err := CreateCloudQuotasTable(db); err != nil {
		return err
	}
	return CreateWorkspacesTable(db)
}
