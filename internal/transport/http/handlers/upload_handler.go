package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	uploadService *usecase.UploadService
}

func NewUploadHandler(uploadService *usecase.UploadService) *UploadHandler {
	return &UploadHandler{uploadService: uploadService}
}

type FileResponse struct {
	ID               uint64            `json:"id"`
	WorkspaceID      uint64            `json:"workspaceId"`
	CloudAccountID   uint64            `json:"cloudAccountId"`
	Provider         string            `json:"provider"`
	ProviderFileID   string            `json:"providerFileId"`
	Filename         string            `json:"filename"`
	OriginalFilename string            `json:"originalFilename"`
	MimeType         string            `json:"mimeType"`
	Extension        string            `json:"extension"`
	SizeBytes        int64             `json:"sizeBytes"`
	Checksum         string            `json:"checksum"`
	Status           domain.FileStatus `json:"status"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

type UploadSuccessResponse struct {
	Success bool         `json:"success"`
	File    FileResponse `json:"file"`
}

type ErrorDetail struct {
	Code    usecase.ErrorCode `json:"code"`
	Message string            `json:"message"`
}

type ErrorResponse struct {
	Success bool        `json:"success"`
	Error   ErrorDetail `json:"error"`
}

func (h *UploadHandler) UploadFileHandler(c *gin.Context) {
	workspaceIDStr := c.Param("workspaceID")
	workspaceID, err := strconv.ParseUint(workspaceIDStr, 10, 64)
	if err != nil || workspaceID == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error: ErrorDetail{
				Code:    usecase.ErrorCodeInvalidUploadRequest,
				Message: "invalid workspace ID parameter",
			},
		})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error: ErrorDetail{
				Code:    usecase.ErrorCodeInvalidUploadRequest,
				Message: "missing or invalid 'file' field in multipart form request",
			},
		})
		return
	}

	fileStream, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error: ErrorDetail{
				Code:    usecase.ErrorCodeInvalidUploadRequest,
				Message: "failed to open uploaded file stream",
			},
		})
		return
	}
	defer fileStream.Close()

	fileRecord, err := h.uploadService.UploadFile(c.Request.Context(), workspaceID, fileHeader.Filename, fileHeader.Size, fileStream)
	if err != nil {
		var appErr *usecase.AppError
		if errors.As(err, &appErr) {
			c.JSON(appErr.StatusCode, ErrorResponse{
				Success: false,
				Error: ErrorDetail{
					Code:    appErr.Code,
					Message: appErr.Message,
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error: ErrorDetail{
				Code:    usecase.ErrorCodeFileUploadFailed,
				Message: "internal server error during file upload",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, UploadSuccessResponse{
		Success: true,
		File: FileResponse{
			ID:               fileRecord.ID,
			WorkspaceID:      fileRecord.WorkspaceID,
			CloudAccountID:   fileRecord.CloudAccountID,
			Provider:         fileRecord.Provider,
			ProviderFileID:   fileRecord.ProviderFileID,
			Filename:         fileRecord.Filename,
			OriginalFilename: fileRecord.OriginalFilename,
			MimeType:         fileRecord.MimeType,
			Extension:        fileRecord.Extension,
			SizeBytes:        fileRecord.SizeBytes,
			Checksum:         fileRecord.Checksum,
			Status:           fileRecord.Status,
			CreatedAt:        fileRecord.CreatedAt,
			UpdatedAt:        fileRecord.UpdatedAt,
		},
	})
}
