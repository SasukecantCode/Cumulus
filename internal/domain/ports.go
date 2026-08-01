package domain

import (
	"context"
	"io"
	"time"
)

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id uint64) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
}

type CloudAccountRepository interface {
	CreateOrUpdate(ctx context.Context, account *CloudAccount) error
	FindByID(ctx context.Context, id uint64) (*CloudAccount, error)
	FindByProviderAccount(ctx context.Context, providerType, providerAccountID string) (*CloudAccount, error)
	FindByEmail(ctx context.Context, email string) ([]*CloudAccount, error)
	Delete(ctx context.Context, id uint64) error
}

type WorkspaceRepository interface {
	Create(ctx context.Context, workspace *Workspace) error
	CreateWorkspaceWithPrimary(ctx context.Context, workspace *Workspace, cloudAccountID uint64) (*WorkspaceCloudAccount, error)
	FindByID(ctx context.Context, id uint64) (*Workspace, error)
	FindBySlug(ctx context.Context, slug string) (*Workspace, error)
	FindByCloudAccountID(ctx context.Context, cloudAccountID uint64) (*Workspace, *WorkspaceCloudAccount, error)
	FindMembership(ctx context.Context, cloudAccountID uint64) (*WorkspaceCloudAccount, error)
	AttachAccount(ctx context.Context, membership *WorkspaceCloudAccount) error
	AttachSecondary(ctx context.Context, membership *WorkspaceCloudAccount) error
	ReconnectMembership(ctx context.Context, membershipID uint64, workspaceID uint64, role WorkspaceRole) error
	DisconnectAccount(ctx context.Context, workspaceID, cloudAccountID uint64) error
	GetActiveAccounts(ctx context.Context, workspaceID uint64) ([]*CloudAccount, error)
	GetActiveStorageNodes(ctx context.Context, workspaceID uint64) ([]*WorkspaceStorageNode, error)
}

type QuotaRepository interface {
	Upsert(ctx context.Context, quota *CloudQuota) error
	FindByCloudAccountID(ctx context.Context, cloudAccountID uint64) (*CloudQuota, error)
}

type QuotaProviderRegistry interface {
	Register(providerType string, provider QuotaProvider)
	Get(providerType string) (QuotaProvider, error)
}

type FileRepository interface {
	Create(ctx context.Context, file *File) error
	FindByID(ctx context.Context, id uint64) (*File, error)
	FindByWorkspace(ctx context.Context, workspaceID uint64) ([]*File, error)
	ListWorkspaceFiles(ctx context.Context, filter FileListFilter) ([]*File, int64, error)
	FindByProviderFileID(ctx context.Context, providerFileID string) (*File, error)
	UpdateMetadata(ctx context.Context, file *File) error
	UpdateStatus(ctx context.Context, id uint64, status FileStatus) error
	SoftDelete(ctx context.Context, id uint64) error
	Rename(ctx context.Context, id uint64, newFilename string) error
	Move(ctx context.Context, id uint64, targetParentID *uint64) error
	Delete(ctx context.Context, id uint64) error
}

type FileChunkRepository interface {
	Create(ctx context.Context, chunk *FileChunk) error
	CreateBatch(ctx context.Context, chunks []*FileChunk) error
	FindByFileID(ctx context.Context, fileID uint64) ([]*FileChunk, error)
	DeleteByFileID(ctx context.Context, fileID uint64) error
	UpdateStatus(ctx context.Context, id uint64, status FileStatus) error
}

type OAuthProvider interface {
	ExchangeCode(ctx context.Context, code string) (OAuthTokens, error)
	GetAuthURL(state string) string
	GetUserProfile(ctx context.Context, accessToken string) (OAuthUserProfile, error)
}

type QuotaProvider interface {
	GetQuota(ctx context.Context, refreshToken string) (QuotaSnapshot, error)
}

type StorageUploadInput struct {
	Filename  string
	MimeType  string
	SizeBytes int64
	Reader    io.Reader
}

type UploadedObject struct {
	ProviderFileID string
	Filename       string
	MimeType       string
	SizeBytes      int64
	CreatedTime    time.Time
	ModifiedTime   time.Time
	ETag           string
	RawResponse    string
}

type StorageFileMetadata struct {
	ProviderFileID string
	Filename       string
	MimeType       string
	SizeBytes      int64
	CreatedTime    time.Time
	ModifiedTime   time.Time
	ETag           string
}

type StorageProvider interface {
	Upload(ctx context.Context, account *CloudAccount, input StorageUploadInput) (*UploadedObject, error)
	Download(ctx context.Context, account *CloudAccount, providerFileID string) (io.ReadCloser, error)
	Delete(ctx context.Context, account *CloudAccount, providerFileID string) error
	Rename(ctx context.Context, account *CloudAccount, providerFileID string, newName string) error
	Move(ctx context.Context, account *CloudAccount, providerFileID string, targetFolderID string) error
	GetMetadata(ctx context.Context, account *CloudAccount, providerFileID string) (*StorageFileMetadata, error)
}

type StorageProviderRegistry interface {
	Register(providerType string, provider StorageProvider)
	Get(providerType string) (StorageProvider, error)
}

type OAuthTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiryUnix   int64
	Scopes       []string
}

type OAuthUserProfile struct {
	GoogleAccountID string
	Email           string
}

type QuotaSnapshot struct {
	TotalBytes     int64
	UsedBytes      int64
	AvailableBytes int64
	RawResponse    string
}

type UploadScheduler interface {
	SelectNode(ctx context.Context, workspaceID uint64, upload *UploadContext) (*WorkspaceStorageNode, error)
}

type WorkspaceSettingsRepository interface {
	GetByWorkspaceID(ctx context.Context, workspaceID uint64) (*WorkspaceSettings, error)
	Save(ctx context.Context, settings *WorkspaceSettings) error
}
