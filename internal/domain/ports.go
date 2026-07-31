package domain

import "context"

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id uint64) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
}

type CloudAccountRepository interface {
	CreateOrUpdate(ctx context.Context, account *CloudAccount) error
	FindByProviderAccount(ctx context.Context, providerType, providerAccountID string) (*CloudAccount, error)
	FindByEmail(ctx context.Context, email string) ([]*CloudAccount, error)
	Delete(ctx context.Context, id uint64) error
}

type QuotaRepository interface {
	Upsert(ctx context.Context, quota *CloudQuota) error
	FindByCloudAccountID(ctx context.Context, cloudAccountID uint64) (*CloudQuota, error)
}

type GoogleAccountRepository interface {
	Create(ctx context.Context, account *GoogleAccount) error
	Update(ctx context.Context, account *GoogleAccount) error
	Delete(ctx context.Context, id uint64) error
	FindByID(ctx context.Context, id uint64) (*GoogleAccount, error)
	FindByUserID(ctx context.Context, userID uint64) ([]GoogleAccount, error)
	FindEligibleForUpload(ctx context.Context, userID uint64) ([]GoogleAccount, error)
}

type FileRepository interface {
	Create(ctx context.Context, file *File) error
	FindByID(ctx context.Context, id uint64) (*File, error)
	FindByOwnerID(ctx context.Context, ownerID uint64) ([]File, error)
	Delete(ctx context.Context, id uint64) error
}

type FolderRepository interface {
	Create(ctx context.Context, folder *Folder) error
	FindByID(ctx context.Context, id uint64) (*Folder, error)
	FindByOwnerID(ctx context.Context, ownerID uint64) ([]Folder, error)
}

type StoragePolicyRepository interface {
	GetForOwner(ctx context.Context, ownerID uint64) (*StoragePolicy, error)
	Save(ctx context.Context, policy *StoragePolicy) error
}

type OAuthProvider interface {
	ExchangeCode(ctx context.Context, code string) (OAuthTokens, error)
	GetAuthURL(state string) string
	GetUserProfile(ctx context.Context, accessToken string) (OAuthUserProfile, error)
}

type QuotaProvider interface {
	GetQuota(ctx context.Context, refreshToken string) (QuotaSnapshot, error)
}

type DriveProvider interface {
	Upload(ctx context.Context, input UploadObject) (UploadResult, error)
	Download(ctx context.Context, fileID string) ([]byte, error)
	Delete(ctx context.Context, fileID string) error
	Rename(ctx context.Context, fileID string, newName string) error
	CreateFolder(ctx context.Context, name string, parentID string) (string, error)
	GetQuota(ctx context.Context) (StorageQuota, error)
}

type Scheduler interface {
	ChooseAccount(ctx context.Context, request ScheduleRequest) (*GoogleAccount, error)
}

type Checksummer interface {
	Checksum(data []byte) (string, error)
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

type UploadObject struct {
	OwnerID         uint64
	Filename        string
	MimeType        string
	Size            int64
	Checksum        string
	GoogleAccountID uint64
	Payload         []byte
}

type UploadResult struct {
	GoogleFileID string
	FolderID     string
}

type StorageQuota struct {
	Total     int64
	Used      int64
	Remaining int64
}

type QuotaSnapshot struct {
	TotalBytes     int64
	UsedBytes      int64
	AvailableBytes int64
	RawResponse    string
}

type ScheduleRequest struct {
	OwnerID  uint64
	Strategy UploadStrategy
	Size     int64
}
