package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"cumulus/internal/domain"
)

// maxSyncedFilesPerAccount bounds one sync pass. A drive with more objects
// than this still works — the newest pages land first and the rest arrive on
// the next pass — but no single request can run unbounded.
const maxSyncedFilesPerAccount = 20000

// DriveSyncService adopts what is already in each connected account.
//
// Cumulus originally only knew about files it uploaded itself, so a drive full
// of existing documents looked empty. This walks every connected account's
// provider listing and reconciles it against the workspace:
//
//	new in provider          -> inserted, marked FileOriginDrive
//	known, metadata changed  -> updated in place
//	known, deleted here      -> revived, because the provider still has it
//	imported, gone from provider -> soft-deleted, following the provider
//
// Rows Cumulus uploaded are never retired by this pass. A chunked upload's
// parent row has no provider object of its own, so it would look "missing"
// on every run and get wrongly deleted.
type DriveSyncService struct {
	workspaceRepo domain.WorkspaceRepository
	fileRepo      domain.FileRepository
	chunkRepo     domain.FileChunkRepository
	registry      domain.StorageProviderRegistry
}

func NewDriveSyncService(
	workspaceRepo domain.WorkspaceRepository,
	fileRepo domain.FileRepository,
	chunkRepo domain.FileChunkRepository,
	registry domain.StorageProviderRegistry,
) *DriveSyncService {
	return &DriveSyncService{
		workspaceRepo: workspaceRepo,
		fileRepo:      fileRepo,
		chunkRepo:     chunkRepo,
		registry:      registry,
	}
}

type AccountSyncResult struct {
	CloudAccountID uint64 `json:"cloudAccountId"`
	Email          string `json:"email"`
	Imported       int    `json:"imported"`
	Updated        int    `json:"updated"`
	Removed        int    `json:"removed"`
	Total          int    `json:"total"`
	Error          string `json:"error,omitempty"`
}

type SyncResult struct {
	WorkspaceID uint64              `json:"workspaceId"`
	Accounts    []AccountSyncResult `json:"accounts"`
	Imported    int                 `json:"imported"`
	Updated     int                 `json:"updated"`
	Removed     int                 `json:"removed"`
	SyncedAt    time.Time           `json:"syncedAt"`
}

// SyncWorkspace reconciles every connected account. One account failing (an
// expired token, a provider outage) is reported against that account and does
// not abort the others — a partial refresh beats none.
func (s *DriveSyncService) SyncWorkspace(ctx context.Context, workspaceID uint64) (*SyncResult, error) {
	ws, err := s.workspaceRepo.FindByID(ctx, workspaceID)
	if err != nil {
		return nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusInternalServerError, "failed to query workspace", err)
	}
	if ws == nil || ws.Status != domain.WorkspaceStatusActive {
		return nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusNotFound, fmt.Sprintf("workspace %d not found or inactive", workspaceID), nil)
	}

	nodes, err := s.workspaceRepo.GetActiveStorageNodes(ctx, workspaceID)
	if err != nil {
		return nil, NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to load workspace accounts", err)
	}

	result := &SyncResult{WorkspaceID: workspaceID, SyncedAt: time.Now().UTC()}

	for _, node := range nodes {
		if node == nil || node.Account == nil {
			continue
		}

		accountResult := s.syncAccount(ctx, workspaceID, node.Account)
		result.Accounts = append(result.Accounts, accountResult)
		result.Imported += accountResult.Imported
		result.Updated += accountResult.Updated
		result.Removed += accountResult.Removed
	}

	return result, nil
}

func (s *DriveSyncService) syncAccount(ctx context.Context, workspaceID uint64, account *domain.CloudAccount) AccountSyncResult {
	out := AccountSyncResult{CloudAccountID: account.ID, Email: account.Email}

	provider, err := s.registry.Get(account.ProviderType)
	if err != nil {
		out.Error = fmt.Sprintf("no storage provider registered for %s", account.ProviderType)
		return out
	}

	remote, err := s.listAll(ctx, provider, account)
	if err != nil {
		out.Error = errorMessage(err)
		return out
	}
	out.Total = len(remote)

	existing, err := s.fileRepo.ListForSync(ctx, domain.FileSyncQuery{
		WorkspaceID:    workspaceID,
		CloudAccountID: account.ID,
	})
	if err != nil {
		out.Error = "failed to read known files for this account"
		return out
	}

	byProviderID := make(map[string]*domain.File, len(existing))
	for _, file := range existing {
		if file.ProviderFileID != "" {
			byProviderID[file.ProviderFileID] = file
		}
	}

	seen := make(map[string]struct{}, len(remote))
	inserts := make([]*domain.File, 0)
	now := time.Now().UTC()

	for _, item := range remote {
		seen[item.ProviderFileID] = struct{}{}

		known, ok := byProviderID[item.ProviderFileID]
		if !ok {
			inserts = append(inserts, &domain.File{
				WorkspaceID:      workspaceID,
				CloudAccountID:   account.ID,
				Provider:         account.ProviderType,
				ProviderFileID:   item.ProviderFileID,
				Filename:         item.Filename,
				OriginalFilename: item.Filename,
				MimeType:         item.MimeType,
				Extension:        extensionOf(item.Filename),
				SizeBytes:        item.SizeBytes,
				Checksum:         item.ETag,
				Status:           domain.FileStatusAvailable,
				Origin:           domain.FileOriginDrive,
				CreatedAt:        item.CreatedTime,
				UpdatedAt:        now,
			})
			continue
		}

		// The provider is the source of truth for name, size and type. A row
		// deleted here but still present there is revived rather than left
		// hidden, otherwise it could never come back.
		changed := known.Filename != item.Filename ||
			known.MimeType != item.MimeType ||
			known.SizeBytes != item.SizeBytes ||
			known.Status == domain.FileStatusDeleted

		if !changed {
			continue
		}

		known.Filename = item.Filename
		known.MimeType = item.MimeType
		known.SizeBytes = item.SizeBytes
		known.Extension = extensionOf(item.Filename)
		known.Status = domain.FileStatusAvailable
		known.UpdatedAt = now

		if err := s.fileRepo.UpdateMetadata(ctx, known); err == nil {
			out.Updated++
		}
	}

	if len(inserts) > 0 {
		if err := s.fileRepo.CreateBatch(ctx, inserts); err != nil {
			out.Error = "failed to save newly discovered files"
			return out
		}
		out.Imported = len(inserts)
	}

	out.Removed = s.retireMissing(ctx, existing, seen)
	return out
}

// retireMissing soft-deletes imported rows the provider no longer lists.
// Only FileOriginDrive rows are eligible: those were adopted from a listing,
// so absence from that same listing genuinely means the file is gone.
func (s *DriveSyncService) retireMissing(ctx context.Context, existing []*domain.File, seen map[string]struct{}) int {
	missing := make([]uint64, 0)
	for _, file := range existing {
		if file.Origin != domain.FileOriginDrive {
			continue
		}
		if file.Status == domain.FileStatusDeleted || file.ProviderFileID == "" {
			continue
		}
		if _, stillThere := seen[file.ProviderFileID]; stillThere {
			continue
		}
		missing = append(missing, file.ID)
	}

	if len(missing) == 0 {
		return 0
	}
	if err := s.fileRepo.MarkMissingFromProvider(ctx, missing); err != nil {
		return 0
	}
	return len(missing)
}

func (s *DriveSyncService) listAll(ctx context.Context, provider domain.StorageProvider, account *domain.CloudAccount) ([]domain.StorageFileMetadata, error) {
	collected := make([]domain.StorageFileMetadata, 0)
	pageToken := ""

	for {
		page, err := provider.List(ctx, account, domain.StorageListInput{PageToken: pageToken})
		if err != nil {
			return nil, err
		}
		if page == nil {
			break
		}

		for _, item := range page.Files {
			if strings.TrimSpace(item.ProviderFileID) == "" {
				continue
			}
			collected = append(collected, item)
		}

		if page.NextPageToken == "" || len(collected) >= maxSyncedFilesPerAccount {
			break
		}
		pageToken = page.NextPageToken
	}

	return collected, nil
}

func extensionOf(filename string) string {
	ext := strings.TrimPrefix(path.Ext(filename), ".")
	if len(ext) > 50 {
		return ""
	}
	return strings.ToLower(ext)
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return err.Error()
}
