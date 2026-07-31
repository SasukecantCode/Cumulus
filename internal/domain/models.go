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
