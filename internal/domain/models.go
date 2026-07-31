package domain

import "time"

type AccountStatus string

const (
	AccountStatusActive   AccountStatus = "active"
	AccountStatusDisabled AccountStatus = "disabled"
	AccountStatusExpired  AccountStatus = "expired"
)

type UploadStrategy string

const (
	UploadStrategyFillFirst       UploadStrategy = "fill_first"
	UploadStrategyLeastUsed       UploadStrategy = "least_used"
	UploadStrategyMostAvailable   UploadStrategy = "most_available"
	UploadStrategyRoundRobin      UploadStrategy = "round_robin"
	UploadStrategyFastestProvider UploadStrategy = "fastest_provider"
	UploadStrategyUserSelected    UploadStrategy = "user_selected"
)

type User struct {
	ID        uint64
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CloudAccount represents a connected cloud storage account (provider-agnostic)
type CloudAccount struct {
	ID                uint64 `gorm:"primaryKey"`
	ProviderType      string `gorm:"uniqueIndex:idx_provider_account"` // "google", "dropbox", "onedrive", etc.
	ProviderAccountID string `gorm:"uniqueIndex:idx_provider_account"` // Google ID, Dropbox ID, etc. (unique per provider)
	Email             string
	RefreshToken      string // For accessing the provider API (encrypted)
	GrantedScopes     string // JSON array of granted scopes
	Status            AccountStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type QuotaSyncStatus string

const (
	QuotaSyncStatusPending QuotaSyncStatus = "pending"
	QuotaSyncStatusSuccess QuotaSyncStatus = "success"
	QuotaSyncStatusFailed  QuotaSyncStatus = "failed"
)

type CloudQuota struct {
	ID             uint64 `gorm:"primaryKey"`
	CloudAccountID uint64 `gorm:"uniqueIndex:idx_cloud_quota_account"`
	Provider       string `gorm:"index:idx_cloud_quota_provider"`
	TotalBytes     int64
	UsedBytes      int64
	AvailableBytes int64
	LastSyncedAt   time.Time       `gorm:"index:idx_cloud_quota_last_synced_at"`
	SyncStatus     QuotaSyncStatus `gorm:"index:idx_cloud_quota_status"`
	SyncError      string          `gorm:"type:text"`
	RawResponse    string          `gorm:"type:text"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TableName specifies the table name for GORM
func (CloudQuota) TableName() string {
	return "cloud_quotas"
}

// TableName specifies the table name for GORM
func (CloudAccount) TableName() string {
	return "cloud_accounts"
}

type WorkspaceStatus string

const (
	WorkspaceStatusActive WorkspaceStatus = "active"
)

type WorkspaceRole string

const (
	WorkspaceRolePrimary   WorkspaceRole = "PRIMARY"
	WorkspaceRoleSecondary WorkspaceRole = "SECONDARY"
)

type WorkspaceConnectionStatus string

const (
	WorkspaceConnectionStatusConnected    WorkspaceConnectionStatus = "CONNECTED"
	WorkspaceConnectionStatusDisconnected WorkspaceConnectionStatus = "DISCONNECTED"
)

type Workspace struct {
	ID        uint64 `gorm:"primaryKey"`
	UUID      string `gorm:"uniqueIndex"`
	Name      string
	Slug      string          `gorm:"uniqueIndex"`
	Status    WorkspaceStatus `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Workspace) TableName() string {
	return "workspaces"
}

type WorkspaceCloudAccount struct {
	ID             uint64                    `gorm:"primaryKey"`
	WorkspaceID    uint64                    `gorm:"index:idx_workspace_cloud_accounts_workspace_id"`
	CloudAccountID uint64                    `gorm:"index:idx_workspace_cloud_accounts_cloud_account_id"`
	Role           WorkspaceRole             `gorm:"index:idx_workspace_cloud_accounts_role"`
	Status         WorkspaceConnectionStatus `gorm:"index:idx_workspace_cloud_accounts_status"`
	ConnectedAt    time.Time                 `gorm:"index:idx_workspace_cloud_accounts_connected_at"`
	DisconnectedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (WorkspaceCloudAccount) TableName() string {
	return "workspace_cloud_accounts"
}

type WorkspaceStorageNode struct {
	Account *CloudAccount
	Quota   *CloudQuota
	Role    WorkspaceRole
	Status  WorkspaceConnectionStatus
}

type GoogleAccount struct {
	ID               uint64
	UserID           uint64
	Gmail            string
	RefreshToken     string
	AccessToken      string
	StorageTotal     int64
	StorageUsed      int64
	StorageRemaining int64
	Priority         int
	Status           AccountStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type File struct {
	ID              uint64
	OwnerID         uint64
	Filename        string
	GoogleFileID    string
	GoogleAccountID uint64
	Size            int64
	Checksum        string
	MimeType        string
	UploadedAt      time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Folder struct {
	ID        uint64
	OwnerID   uint64
	Name      string
	Path      string
	ParentID  *uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type StoragePolicy struct {
	ID               uint64
	OwnerID          uint64
	Name             string
	PrimaryStrategy  UploadStrategy
	FallbackStrategy UploadStrategy
	PreferredAccount uint64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
