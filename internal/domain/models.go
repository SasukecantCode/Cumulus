package domain

import "time"

type AccountStatus string

const (
	AccountStatusActive   AccountStatus = "active"
	AccountStatusDisabled AccountStatus = "disabled"
	AccountStatusExpired  AccountStatus = "expired"
)

// UploadStrategy defines how storage accounts are selected for single-node file uploads.
// Cumulus defaults to FillFirst to minimize fragmentation by filling one account
// to capacity before utilizing additional storage nodes.
type UploadStrategy string

const (
	// UploadStrategyFillFirst is the DEFAULT primary strategy for Cumulus.
	// It fills accounts sequentially (Primary first, then Secondary by ID) to minimize fragmentation.
	UploadStrategyFillFirst UploadStrategy = "fill_first"

	// UploadStrategyLeastUsed is an optional advanced strategy.
	// It distributes files to the account with the lowest utilization percentage.
	UploadStrategyLeastUsed UploadStrategy = "least_used"

	// UploadStrategyMostAvailable is an optional advanced strategy.
	// It distributes files to the account with the highest absolute available capacity.
	UploadStrategyMostAvailable UploadStrategy = "most_available"

	// UploadStrategyRoundRobin is an optional advanced strategy.
	// It rotates uploads evenly across eligible accounts. Note: This conflicts with
	// the core Cumulus philosophy of minimizing fragmentation and must never be the default.
	UploadStrategyRoundRobin UploadStrategy = "round_robin"

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

type UploadContext struct {
	Filename  string
	SizeBytes int64
	MimeType  string
}

type WorkspaceSettings struct {
	WorkspaceID    uint64         `gorm:"primaryKey"`
	UploadStrategy UploadStrategy `gorm:"type:varchar(50);default:'fill_first'"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (WorkspaceSettings) TableName() string {
	return "workspace_settings"
}

type WorkspaceStorageNode struct {
	Account *CloudAccount
	Quota   *CloudQuota
	Role    WorkspaceRole
	Status  WorkspaceConnectionStatus
}

type FileStatus string

const (
	FileStatusUploading    FileStatus = "UPLOADING"
	FileStatusAvailable    FileStatus = "AVAILABLE"
	FileStatusFailed       FileStatus = "FAILED"
	FileStatusDeleting     FileStatus = "DELETING"
	FileStatusDeleteFailed FileStatus = "DELETE_FAILED"
	FileStatusDeleted      FileStatus = "DELETED"
)

type FileListFilter struct {
	WorkspaceID uint64
	Limit       int
	Offset      int
	Filename    string
	MimeType    string
	Status      FileStatus
}

type File struct {
	ID               uint64  `gorm:"primaryKey"`
	WorkspaceID      uint64  `gorm:"index:idx_files_workspace_id"`
	CloudAccountID   uint64  `gorm:"index:idx_files_cloud_account_id"`
	ParentID         *uint64 `gorm:"index:idx_files_parent_id"`
	Provider         string  `gorm:"index:idx_files_provider"`
	ProviderFileID   string  `gorm:"index:idx_files_provider_file_id"`
	Filename         string  `gorm:"type:varchar(255)"`
	OriginalFilename string  `gorm:"type:varchar(255)"`
	MimeType         string  `gorm:"type:varchar(255)"`
	Extension        string  `gorm:"type:varchar(50)"`
	SizeBytes        int64
	Checksum         string     `gorm:"type:varchar(64)"`
	Status           FileStatus `gorm:"type:varchar(50);index:idx_files_status"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time `gorm:"index:idx_files_deleted_at"`
}

func (File) TableName() string {
	return "files"
}

type FileChunk struct {
	ID             uint64 `gorm:"primaryKey"`
	FileID         uint64 `gorm:"index:idx_file_chunks_file_id"`
	CloudAccountID uint64 `gorm:"index:idx_file_chunks_cloud_account_id"`
	Provider       string `gorm:"type:varchar(50)"`
	ProviderFileID string `gorm:"type:varchar(255)"`
	ChunkIndex     int    `gorm:"index:idx_file_chunks_chunk_index"`
	OffsetBytes    int64
	SizeBytes      int64
	Checksum       string     `gorm:"type:varchar(64)"`
	Status         FileStatus `gorm:"type:varchar(50)"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (FileChunk) TableName() string {
	return "file_chunks"
}
