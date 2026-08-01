package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"cumulus/internal/domain"
)

const DefaultMaxUploadSize = 100 * 1024 * 1024 // 100MB

type UploadService struct {
	workspaceRepo domain.WorkspaceRepository
	fileRepo      domain.FileRepository
	chunkRepo     domain.FileChunkRepository
	registry      domain.StorageProviderRegistry
	scheduler     domain.UploadScheduler
	chunkPlanner  *ChunkPlanner
	maxUploadSize int64
}

func NewUploadService(
	workspaceRepo domain.WorkspaceRepository,
	fileRepo domain.FileRepository,
	chunkRepo domain.FileChunkRepository,
	registry domain.StorageProviderRegistry,
	scheduler domain.UploadScheduler,
) *UploadService {
	return &UploadService{
		workspaceRepo: workspaceRepo,
		fileRepo:      fileRepo,
		chunkRepo:     chunkRepo,
		registry:      registry,
		scheduler:     scheduler,
		chunkPlanner:  NewChunkPlanner(),
		maxUploadSize: DefaultMaxUploadSize,
	}
}

func (s *UploadService) SetMaxUploadSize(bytes int64) {
	if bytes > 0 {
		s.maxUploadSize = bytes
	}
}

func (s *UploadService) UploadFile(ctx context.Context, workspaceID uint64, originalFilename string, sizeBytes int64, reader io.Reader) (*domain.File, error) {
	if reader == nil {
		return nil, NewAppError(ErrorCodeInvalidUploadRequest, http.StatusBadRequest, "file payload stream is nil", nil)
	}

	if sizeBytes > s.maxUploadSize {
		return nil, NewAppError(ErrorCodeFileTooLarge, http.StatusRequestEntityTooLarge, fmt.Sprintf("file size %d bytes exceeds maximum limit of %d bytes", sizeBytes, s.maxUploadSize), nil)
	}

	// Step 1: Validate Workspace
	workspace, err := s.workspaceRepo.FindByID(ctx, workspaceID)
	if err != nil {
		return nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusInternalServerError, "failed to query workspace", err)
	}
	if workspace == nil || workspace.Status != domain.WorkspaceStatusActive {
		return nil, NewAppError(ErrorCodeWorkspaceNotFound, http.StatusNotFound, fmt.Sprintf("workspace %d not found or inactive", workspaceID), nil)
	}

	// Step 2: Try single storage node selection via UploadScheduler
	var targetNode *domain.WorkspaceStorageNode
	if s.scheduler != nil {
		node, err := s.scheduler.SelectNode(ctx, workspaceID, &domain.UploadContext{
			Filename:  originalFilename,
			SizeBytes: sizeBytes,
		})
		if err == nil {
			targetNode = node
		}
	} else {
		// Fallback to PRIMARY node resolution if scheduler not supplied
		nodes, err := s.workspaceRepo.GetActiveStorageNodes(ctx, workspaceID)
		if err == nil {
			for _, node := range nodes {
				if node.Role == domain.WorkspaceRolePrimary && node.Status == domain.WorkspaceConnectionStatusConnected && node.Account != nil {
					if node.Quota == nil || node.Quota.AvailableBytes >= sizeBytes {
						targetNode = node
						break
					}
				}
			}
		}

	}

	// Step 3: Decision Flow
	if targetNode != nil {
		// Single Upload Path
		return s.uploadSingleNode(ctx, workspace, targetNode, originalFilename, sizeBytes, reader)
	}

	// Chunked Upload Path (Invoke ChunkPlanner)
	return s.uploadChunked(ctx, workspace, originalFilename, sizeBytes, reader)
}

func (s *UploadService) uploadSingleNode(
	ctx context.Context,
	workspace *domain.Workspace,
	targetNode *domain.WorkspaceStorageNode,
	originalFilename string,
	sizeBytes int64,
	reader io.Reader,
) (*domain.File, error) {
	provider, err := s.registry.Get(targetNode.Account.ProviderType)
	if err != nil {
		return nil, NewAppError(ErrorCodeStorageProviderUnavailable, http.StatusServiceUnavailable, fmt.Sprintf("no storage provider registered for provider %s", targetNode.Account.ProviderType), err)
	}

	sanitizedFilename := sanitizeFilename(originalFilename)
	ext := strings.ToLower(filepath.Ext(sanitizedFilename))

	headerBuf := make([]byte, 512)
	n, readErr := io.ReadFull(reader, headerBuf)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return nil, NewAppError(ErrorCodeInvalidUploadRequest, http.StatusBadRequest, "failed to read file header for content-type detection", readErr)
	}
	headerBuf = headerBuf[:n]
	detectedMimeType := http.DetectContentType(headerBuf)

	fullStream := io.MultiReader(bytes.NewReader(headerBuf), reader)
	hasher := sha256.New()
	teeStream := io.TeeReader(fullStream, hasher)

	fileRecord := &domain.File{
		WorkspaceID:      workspace.ID,
		CloudAccountID:   targetNode.Account.ID,
		Provider:         targetNode.Account.ProviderType,
		ProviderFileID:   "",
		Filename:         sanitizedFilename,
		OriginalFilename: originalFilename,
		MimeType:         detectedMimeType,
		Extension:        ext,
		SizeBytes:        sizeBytes,
		Checksum:         "",
		Status:           domain.FileStatusUploading,
		Origin:           domain.FileOriginCumulus,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := s.fileRepo.Create(ctx, fileRecord); err != nil {
		return nil, NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to record initial file upload metadata", err)
	}

	uploadInput := domain.StorageUploadInput{
		Filename:  sanitizedFilename,
		MimeType:  detectedMimeType,
		SizeBytes: sizeBytes,
		Reader:    teeStream,
	}

	uploadedObject, uploadErr := provider.Upload(ctx, targetNode.Account, uploadInput)
	if uploadErr != nil {
		_ = s.fileRepo.UpdateStatus(ctx, fileRecord.ID, domain.FileStatusFailed)
		return nil, uploadErr
	}

	checksumHex := hex.EncodeToString(hasher.Sum(nil))

	fileRecord.ProviderFileID = uploadedObject.ProviderFileID
	if uploadedObject.SizeBytes > 0 {
		fileRecord.SizeBytes = uploadedObject.SizeBytes
	}
	fileRecord.Checksum = checksumHex
	fileRecord.Status = domain.FileStatusAvailable
	fileRecord.UpdatedAt = time.Now().UTC()

	if err := s.fileRepo.UpdateMetadata(ctx, fileRecord); err != nil {
		_ = provider.Delete(ctx, targetNode.Account, uploadedObject.ProviderFileID)
		_ = s.fileRepo.UpdateStatus(ctx, fileRecord.ID, domain.FileStatusFailed)
		return nil, NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to persist file metadata after remote upload; upload rolled back", err)
	}

	return fileRecord, nil
}

func (s *UploadService) uploadChunked(
	ctx context.Context,
	workspace *domain.Workspace,
	originalFilename string,
	sizeBytes int64,
	reader io.Reader,
) (*domain.File, error) {
	nodes, err := s.workspaceRepo.GetActiveStorageNodes(ctx, workspace.ID)
	if err != nil {
		return nil, NewAppError(ErrorCodePrimaryAccountMissing, http.StatusInternalServerError, "failed to query workspace storage nodes", err)
	}

	var eligible []*domain.WorkspaceStorageNode
	for _, node := range nodes {
		if node != nil && node.Account != nil && node.Quota != nil {
			if node.Status == domain.WorkspaceConnectionStatusConnected &&
				node.Account.Status == domain.AccountStatusActive &&
				node.Quota.SyncStatus == domain.QuotaSyncStatusSuccess &&
				node.Quota.AvailableBytes > 0 {
				eligible = append(eligible, node)
			}
		}
	}

	plan, err := s.chunkPlanner.Plan(sizeBytes, eligible)
	if err != nil {
		return nil, err
	}

	sanitizedFilename := sanitizeFilename(originalFilename)
	ext := strings.ToLower(filepath.Ext(sanitizedFilename))

	headerBuf := make([]byte, 512)
	n, readErr := io.ReadFull(reader, headerBuf)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return nil, NewAppError(ErrorCodeInvalidUploadRequest, http.StatusBadRequest, "failed to read file header for content-type detection", readErr)
	}
	headerBuf = headerBuf[:n]
	detectedMimeType := http.DetectContentType(headerBuf)

	fullStream := io.MultiReader(bytes.NewReader(headerBuf), reader)
	fileHasher := sha256.New()
	teeStream := io.TeeReader(fullStream, fileHasher)

	firstAccountID := plan.Chunks[0].Node.Account.ID
	fileRecord := &domain.File{
		WorkspaceID:      workspace.ID,
		CloudAccountID:   firstAccountID,
		Provider:         plan.Chunks[0].Node.Account.ProviderType,
		ProviderFileID:   "",
		Filename:         sanitizedFilename,
		OriginalFilename: originalFilename,
		MimeType:         detectedMimeType,
		Extension:        ext,
		SizeBytes:        sizeBytes,
		Checksum:         "",
		Status:           domain.FileStatusUploading,
		Origin:           domain.FileOriginCumulus,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := s.fileRepo.Create(ctx, fileRecord); err != nil {
		return nil, NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to record initial file upload metadata", err)
	}

	var uploadedChunkRecords []*domain.FileChunk
	for _, chunkSpec := range plan.Chunks {
		chunkReader := io.LimitReader(teeStream, chunkSpec.SizeBytes)
		chunkHasher := sha256.New()
		chunkTee := io.TeeReader(chunkReader, chunkHasher)

		provider, err := s.registry.Get(chunkSpec.Node.Account.ProviderType)
		if err != nil {
			s.rollbackChunkedUpload(ctx, fileRecord.ID, uploadedChunkRecords)
			return nil, NewAppError(ErrorCodeStorageProviderUnavailable, http.StatusServiceUnavailable, fmt.Sprintf("no storage provider registered for provider %s", chunkSpec.Node.Account.ProviderType), err)
		}

		uploadInput := domain.StorageUploadInput{
			Filename:  fmt.Sprintf("%s.chunk%d", sanitizedFilename, chunkSpec.ChunkIndex),
			MimeType:  "application/octet-stream",
			SizeBytes: chunkSpec.SizeBytes,
			Reader:    chunkTee,
		}

		uploadedObj, uploadErr := provider.Upload(ctx, chunkSpec.Node.Account, uploadInput)
		if uploadErr != nil {
			s.rollbackChunkedUpload(ctx, fileRecord.ID, uploadedChunkRecords)
			return nil, uploadErr
		}

		chunkChecksum := hex.EncodeToString(chunkHasher.Sum(nil))
		chunkRecord := &domain.FileChunk{
			FileID:         fileRecord.ID,
			CloudAccountID: chunkSpec.Node.Account.ID,
			Provider:       chunkSpec.Node.Account.ProviderType,
			ProviderFileID: uploadedObj.ProviderFileID,
			ChunkIndex:     chunkSpec.ChunkIndex,
			OffsetBytes:    chunkSpec.OffsetBytes,
			SizeBytes:      chunkSpec.SizeBytes,
			Checksum:       chunkChecksum,
			Status:         domain.FileStatusAvailable,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		if s.chunkRepo != nil {
			if err := s.chunkRepo.Create(ctx, chunkRecord); err != nil {
				_ = provider.Delete(ctx, chunkSpec.Node.Account, uploadedObj.ProviderFileID)
				s.rollbackChunkedUpload(ctx, fileRecord.ID, uploadedChunkRecords)
				return nil, NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to persist chunk metadata", err)
			}
		}
		uploadedChunkRecords = append(uploadedChunkRecords, chunkRecord)
	}

	checksumHex := hex.EncodeToString(fileHasher.Sum(nil))
	fileRecord.Checksum = checksumHex
	fileRecord.Status = domain.FileStatusAvailable
	fileRecord.UpdatedAt = time.Now().UTC()

	if err := s.fileRepo.UpdateMetadata(ctx, fileRecord); err != nil {
		s.rollbackChunkedUpload(ctx, fileRecord.ID, uploadedChunkRecords)
		return nil, NewAppError(ErrorCodeDatabasePersistenceFailed, http.StatusInternalServerError, "failed to persist final file metadata after chunked upload", err)
	}

	return fileRecord, nil
}

func (s *UploadService) rollbackChunkedUpload(ctx context.Context, fileID uint64, uploadedChunks []*domain.FileChunk) {
	for _, chunk := range uploadedChunks {
		if chunk == nil || chunk.ProviderFileID == "" {
			continue
		}
		if provider, err := s.registry.Get(chunk.Provider); err == nil {
			account := &domain.CloudAccount{
				ID:           chunk.CloudAccountID,
				ProviderType: chunk.Provider,
			}
			_ = provider.Delete(ctx, account, chunk.ProviderFileID)
		}
	}
	if s.chunkRepo != nil {
		_ = s.chunkRepo.DeleteByFileID(ctx, fileID)
	}
	_ = s.fileRepo.UpdateStatus(ctx, fileID, domain.FileStatusFailed)
}

func sanitizeFilename(filename string) string {
	base := filepath.Base(strings.TrimSpace(filename))
	base = strings.ReplaceAll(base, "\\", "_")
	base = strings.ReplaceAll(base, "/", "_")
	if base == "" || base == "." || base == ".." {
		return "file_" + fmt.Sprintf("%d", time.Now().Unix())
	}
	return base
}
