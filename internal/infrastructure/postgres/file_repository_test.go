package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"cumulus/internal/domain"
	"cumulus/internal/infrastructure/postgres"
)

func TestFileRepository(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/cumulus?sslmode=disable"
	}

	db, err := postgres.Connect(dbURL)
	if err != nil {
		t.Skipf("Skipping DB integration test: PostgreSQL not available (%v)", err)
	}

	if err := postgres.Migrate(db); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	wsRepo := postgres.NewWorkspaceRepository(db)
	accRepo := postgres.NewCloudAccountRepository(db)
	fileRepo := postgres.NewFileRepository(db)
	ctx := context.Background()

	now := time.Now().UnixNano()

	acc := &domain.CloudAccount{
		ProviderType:      "google",
		ProviderAccountID: fmt.Sprintf("file-repo-account-%d", now),
		Email:             fmt.Sprintf("file.repo.%d@example.com", now),
		RefreshToken:      "ref-token",
		GrantedScopes:     "[]",
		Status:            domain.AccountStatusActive,
	}
	if err := accRepo.CreateOrUpdate(ctx, acc); err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	ws := &domain.Workspace{
		UUID:   fmt.Sprintf("file-repo-uuid-%d", now),
		Name:   "File Repo WS",
		Slug:   fmt.Sprintf("file-repo-slug-%d", now),
		Status: domain.WorkspaceStatusActive,
	}
	if _, err := wsRepo.CreateWorkspaceWithPrimary(ctx, ws, acc.ID); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	file := &domain.File{
		WorkspaceID:      ws.ID,
		CloudAccountID:   acc.ID,
		Provider:         "google",
		ProviderFileID:   fmt.Sprintf("gdrive-%d", now),
		Filename:         "doc.pdf",
		OriginalFilename: "my_doc.pdf",
		MimeType:         "application/pdf",
		Extension:        ".pdf",
		SizeBytes:        1024,
		Checksum:         "hash123",
		Status:           domain.FileStatusUploading,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := fileRepo.Create(ctx, file); err != nil {
		t.Fatalf("FileRepo.Create failed: %v", err)
	}

	if file.ID == 0 {
		t.Fatalf("expected non-zero file ID after Create")
	}

	foundByID, err := fileRepo.FindByID(ctx, file.ID)
	if err != nil || foundByID == nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if foundByID.Filename != "doc.pdf" || foundByID.Status != domain.FileStatusUploading {
		t.Fatalf("FindByID returned unexpected file data: %+v", foundByID)
	}

	foundByProviderID, err := fileRepo.FindByProviderFileID(ctx, file.ProviderFileID)
	if err != nil || foundByProviderID == nil {
		t.Fatalf("FindByProviderFileID failed: %v", err)
	}

	filesInWS, err := fileRepo.FindByWorkspace(ctx, ws.ID)
	if err != nil || len(filesInWS) == 0 {
		t.Fatalf("FindByWorkspace failed or returned 0 files")
	}

	// Test UpdateStatus
	if err := fileRepo.UpdateStatus(ctx, file.ID, domain.FileStatusAvailable); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	updatedStatus, _ := fileRepo.FindByID(ctx, file.ID)
	if updatedStatus.Status != domain.FileStatusAvailable {
		t.Fatalf("expected status AVAILABLE, got %s", updatedStatus.Status)
	}

	// Test ListWorkspaceFiles
	filesList, count, err := fileRepo.ListWorkspaceFiles(ctx, domain.FileListFilter{WorkspaceID: ws.ID})
	if err != nil || count != 1 || len(filesList) != 1 {
		t.Fatalf("ListWorkspaceFiles failed: err=%v count=%d len=%d", err, count, len(filesList))
	}

	// Test Rename
	if err := fileRepo.Rename(ctx, file.ID, "renamed_doc.pdf"); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	renamedFile, _ := fileRepo.FindByID(ctx, file.ID)
	if renamedFile.Filename != "renamed_doc.pdf" {
		t.Fatalf("expected filename renamed_doc.pdf, got %s", renamedFile.Filename)
	}

	// Test SoftDelete
	if err := fileRepo.SoftDelete(ctx, file.ID); err != nil {
		t.Fatalf("SoftDelete failed: %v", err)
	}

	softDeletedFile, _ := fileRepo.FindByID(ctx, file.ID)
	if softDeletedFile.Status != domain.FileStatusDeleted || softDeletedFile.DeletedAt == nil {
		t.Fatalf("expected soft deleted status and non-nil DeletedAt, got status=%s deletedAt=%v", softDeletedFile.Status, softDeletedFile.DeletedAt)
	}

	filesActive, activeCount, _ := fileRepo.ListWorkspaceFiles(ctx, domain.FileListFilter{WorkspaceID: ws.ID})
	if activeCount != 0 || len(filesActive) != 0 {
		t.Fatalf("expected 0 active files after soft delete, got activeCount=%d len=%d", activeCount, len(filesActive))
	}

	// Test Hard Delete
	if err := fileRepo.Delete(ctx, file.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	deletedFile, _ := fileRepo.FindByID(ctx, file.ID)
	if deletedFile != nil {
		t.Fatalf("expected file to be deleted from DB, got %+v", deletedFile)
	}
}
