package usecase

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"cumulus/internal/domain"
)

type FileService struct {
	workspaceRepo    domain.WorkspaceRepository
	fileRepo         domain.FileRepository
	chunkRepo        domain.FileChunkRepository
	cloudAccountRepo domain.CloudAccountRepository
	registry         domain.StorageProviderRegistry
}

func NewFileService(
	workspaceRepo domain.WorkspaceRepository,
	fileRepo domain.FileRepository,
	chunkRepo domain.FileChunkRepository,
	cloudAccountRepo domain.CloudAccountRepository,
	registry domain.StorageProviderRegistry,
) *FileService {
	return &FileService{
		workspaceRepo:    workspaceRepo,
		fileRepo:         fileRepo,
		chunkRepo:        chunkRepo,
		cloudAccountRepo: cloudAccountRepo,
		registry:         registry,
	}
}

func (s *FileService) validateWorkspace(ctx context.Context, workspaceID uint64) (*domain.Workspace, error) {
	ws, err := s.workspaceRepo.FindByID(ctx, workspaceID)
	if err != nil {
		return nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusInternalServerError, "failed to query workspace", err)
	}
	if ws == nil || ws.Status != domain.WorkspaceStatusActive {
		return nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusNotFound, fmt.Sprintf("workspace %d not found or inactive", workspaceID), nil)
	}
	return ws, nil
}

func (s *FileService) ListFiles(ctx context.Context, workspaceID uint64, filter domain.FileListFilter) ([]*domain.File, int64, error) {
	if _, err := s.validateWorkspace(ctx, workspaceID); err != nil {
		return nil, 0, err
	}
	filter.WorkspaceID = workspaceID
	files, total, err := s.fileRepo.ListWorkspaceFiles(ctx, filter)
	if err != nil {
		return nil, 0, NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to list workspace files", err)
	}
	return files, total, nil
}

func (s *FileService) GetFile(ctx context.Context, workspaceID uint64, fileID uint64) (*domain.File, error) {
	if _, err := s.validateWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}
	file, err := s.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		return nil, NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to query file metadata", err)
	}
	if file == nil || file.WorkspaceID != workspaceID || file.Status == domain.FileStatusDeleted {
		return nil, NewAppError(ErrorCodeFileNotFound, http.StatusNotFound, fmt.Sprintf("file %d not found in workspace %d", fileID, workspaceID), nil)
	}
	return file, nil
}

func (s *FileService) DownloadFile(ctx context.Context, workspaceID uint64, fileID uint64) (io.ReadCloser, *domain.File, error) {
	file, err := s.GetFile(ctx, workspaceID, fileID)
	if err != nil {
		return nil, nil, err
	}

	var chunks []*domain.FileChunk
	if s.chunkRepo != nil {
		c, err := s.chunkRepo.FindByFileID(ctx, fileID)
		if err == nil && len(c) > 0 {
			chunks = c
		}
	}

	// Single File Download Path
	if len(chunks) == 0 {
		account, err := s.resolveCloudAccount(ctx, file.CloudAccountID)
		if err != nil {
			return nil, nil, err
		}

		provider, err := s.registry.Get(file.Provider)
		if err != nil {
			return nil, nil, NewAppError(ErrorCodeStorageProviderUnavailable, http.StatusServiceUnavailable, fmt.Sprintf("no storage provider registered for %s", file.Provider), err)
		}

		stream, err := provider.Download(ctx, account, file.ProviderFileID)
		if err != nil {
			return nil, nil, err
		}

		return stream, file, nil
	}

	// Chunked File Download Path (Sequential Streaming)
	stream := newChunkStreamReader(ctx, chunks, s.cloudAccountRepo, s.registry)
	return stream, file, nil
}

func (s *FileService) DeleteFile(ctx context.Context, workspaceID uint64, fileID uint64) error {
	file, err := s.GetFile(ctx, workspaceID, fileID)
	if err != nil {
		return err
	}

	var chunks []*domain.FileChunk
	if s.chunkRepo != nil {
		c, err := s.chunkRepo.FindByFileID(ctx, fileID)
		if err == nil && len(c) > 0 {
			chunks = c
		}
	}

	// Single File Delete Path
	if len(chunks) == 0 {
		account, err := s.resolveCloudAccount(ctx, file.CloudAccountID)
		if err != nil {
			return err
		}

		provider, err := s.registry.Get(file.Provider)
		if err != nil {
			return NewAppError(ErrorCodeStorageProviderUnavailable, http.StatusServiceUnavailable, fmt.Sprintf("no storage provider registered for %s", file.Provider), err)
		}

		_ = s.fileRepo.UpdateStatus(ctx, fileID, domain.FileStatusDeleting)

		if err := provider.Delete(ctx, account, file.ProviderFileID); err != nil {
			_ = s.fileRepo.UpdateStatus(ctx, fileID, domain.FileStatusAvailable)
			return err
		}

		if err := s.fileRepo.SoftDelete(ctx, fileID); err != nil {
			_ = s.fileRepo.UpdateStatus(ctx, fileID, domain.FileStatusDeleteFailed)
			return NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to soft-delete file in database; marked status as DELETE_FAILED", err)
		}

		return nil
	}

	// Chunked File Delete Path
	_ = s.fileRepo.UpdateStatus(ctx, fileID, domain.FileStatusDeleting)

	for _, chunk := range chunks {
		account, err := s.resolveCloudAccount(ctx, chunk.CloudAccountID)
		if err != nil || account == nil {
			account = &domain.CloudAccount{ID: chunk.CloudAccountID, ProviderType: chunk.Provider}
		}

		provider, err := s.registry.Get(chunk.Provider)
		if err != nil {
			_ = s.fileRepo.UpdateStatus(ctx, fileID, domain.FileStatusAvailable)
			return NewAppError(ErrorCodeStorageProviderUnavailable, http.StatusServiceUnavailable, fmt.Sprintf("no storage provider registered for %s", chunk.Provider), err)
		}

		if err := provider.Delete(ctx, account, chunk.ProviderFileID); err != nil {
			_ = s.fileRepo.UpdateStatus(ctx, fileID, domain.FileStatusAvailable)
			return err
		}
	}

	if s.chunkRepo != nil {
		_ = s.chunkRepo.DeleteByFileID(ctx, fileID)
	}

	if err := s.fileRepo.SoftDelete(ctx, fileID); err != nil {
		_ = s.fileRepo.UpdateStatus(ctx, fileID, domain.FileStatusDeleteFailed)
		return NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to soft-delete chunked file in database; marked status as DELETE_FAILED", err)
	}

	return nil
}

func (s *FileService) RenameFile(ctx context.Context, workspaceID uint64, fileID uint64, newFilename string) (*domain.File, error) {
	file, err := s.GetFile(ctx, workspaceID, fileID)
	if err != nil {
		return nil, err
	}

	sanitized := sanitizeFilename(newFilename)
	if sanitized == "" {
		return nil, NewAppError(ErrorCodeInvalidUploadRequest, http.StatusBadRequest, "invalid filename", nil)
	}

	account, err := s.resolveCloudAccount(ctx, file.CloudAccountID)
	if err != nil {
		return nil, err
	}

	provider, err := s.registry.Get(file.Provider)
	if err != nil {
		return nil, NewAppError(ErrorCodeStorageProviderUnavailable, http.StatusServiceUnavailable, fmt.Sprintf("no storage provider registered for %s", file.Provider), err)
	}

	if file.ProviderFileID != "" {
		if err := provider.Rename(ctx, account, file.ProviderFileID, sanitized); err != nil {
			return nil, err
		}
	}

	if err := s.fileRepo.Rename(ctx, fileID, sanitized); err != nil {
		return nil, NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to rename file in database", err)
	}

	file.Filename = sanitized
	file.UpdatedAt = time.Now().UTC()
	return file, nil
}

func (s *FileService) MoveFile(ctx context.Context, workspaceID uint64, fileID uint64, targetParentID *uint64) (*domain.File, error) {
	file, err := s.GetFile(ctx, workspaceID, fileID)
	if err != nil {
		return nil, err
	}

	if err := s.fileRepo.Move(ctx, fileID, targetParentID); err != nil {
		return nil, NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to update file parent in database", err)
	}

	file.ParentID = targetParentID
	file.UpdatedAt = time.Now().UTC()
	return file, nil
}

func (s *FileService) resolveCloudAccount(ctx context.Context, cloudAccountID uint64) (*domain.CloudAccount, error) {
	if s.cloudAccountRepo != nil {
		acc, err := s.cloudAccountRepo.FindByID(ctx, cloudAccountID)
		if err == nil && acc != nil {
			return acc, nil
		}
	}

	return nil, NewAppError(ErrorCodePrimaryAccountMissing, http.StatusNotFound, fmt.Sprintf("cloud account %d not found", cloudAccountID), nil)
}

type chunkStreamReader struct {
	ctx              context.Context
	chunks           []*domain.FileChunk
	cloudAccountRepo domain.CloudAccountRepository
	registry         domain.StorageProviderRegistry
	currentIndex     int
	currentStream    io.ReadCloser
	closed           bool
}

func newChunkStreamReader(
	ctx context.Context,
	chunks []*domain.FileChunk,
	cloudAccountRepo domain.CloudAccountRepository,
	registry domain.StorageProviderRegistry,
) *chunkStreamReader {
	return &chunkStreamReader{
		ctx:              ctx,
		chunks:           chunks,
		cloudAccountRepo: cloudAccountRepo,
		registry:         registry,
	}
}

func (r *chunkStreamReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, io.ErrClosedPipe
	}
	for {
		if r.currentStream == nil {
			if r.currentIndex >= len(r.chunks) {
				return 0, io.EOF
			}
			chunk := r.chunks[r.currentIndex]
			var account *domain.CloudAccount
			if r.cloudAccountRepo != nil {
				acc, err := r.cloudAccountRepo.FindByID(r.ctx, chunk.CloudAccountID)
				if err == nil && acc != nil {
					account = acc
				}
			}
			if account == nil {
				account = &domain.CloudAccount{ID: chunk.CloudAccountID, ProviderType: chunk.Provider}
			}

			provider, err := r.registry.Get(chunk.Provider)
			if err != nil {
				return 0, fmt.Errorf("failed to get provider %s for chunk %d: %w", chunk.Provider, chunk.ChunkIndex, err)
			}

			stream, err := provider.Download(r.ctx, account, chunk.ProviderFileID)
			if err != nil {
				return 0, fmt.Errorf("failed to download chunk %d: %w", chunk.ChunkIndex, err)
			}
			r.currentStream = stream
		}

		n, err := r.currentStream.Read(p)
		if err == io.EOF {
			_ = r.currentStream.Close()
			r.currentStream = nil
			r.currentIndex++
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (r *chunkStreamReader) Close() error {
	r.closed = true
	if r.currentStream != nil {
		err := r.currentStream.Close()
		r.currentStream = nil
		return err
	}
	return nil
}
