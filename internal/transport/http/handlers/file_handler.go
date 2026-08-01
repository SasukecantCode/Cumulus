package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"cumulus/internal/domain"
	"cumulus/internal/usecase"
	"github.com/gin-gonic/gin"
)

func respondWithError(c *gin.Context, err error) {
	var appErr *usecase.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.StatusCode, gin.H{
			"success": false,
			"error": gin.H{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"error": gin.H{
			"code":    "internal_error",
			"message": err.Error(),
		},
	})
}

type FileHandler struct {
	fileService *usecase.FileService
}

func NewFileHandler(fileService *usecase.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

func (h *FileHandler) ListFilesHandler(c *gin.Context) {
	wsIDStr := c.Param("workspaceID")
	workspaceID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_workspace_id",
				"message": "workspace ID must be a valid positive integer",
			},
		})
		return
	}

	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	filename := c.Query("filename")
	mimeType := c.Query("mimeType")
	status := domain.FileStatus(c.Query("status"))

	filter := domain.FileListFilter{
		Limit:    limit,
		Offset:   offset,
		Filename: filename,
		MimeType: mimeType,
		Status:   status,
	}

	files, total, listErr := h.fileService.ListFiles(c.Request.Context(), workspaceID, filter)
	if listErr != nil {
		respondWithError(c, listErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    files,
		"pagination": gin.H{
			"total":  total,
			"limit":  filter.Limit,
			"offset": filter.Offset,
		},
	})
}

func (h *FileHandler) GetFileHandler(c *gin.Context) {
	wsIDStr := c.Param("workspaceID")
	workspaceID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_workspace_id",
				"message": "workspace ID must be a valid positive integer",
			},
		})
		return
	}

	fileIDStr := c.Param("fileID")
	fileID, err := strconv.ParseUint(fileIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_file_id",
				"message": "file ID must be a valid positive integer",
			},
		})
		return
	}

	file, getErr := h.fileService.GetFile(c.Request.Context(), workspaceID, fileID)
	if getErr != nil {
		respondWithError(c, getErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"file":    file,
	})
}

func (h *FileHandler) DownloadFileHandler(c *gin.Context) {
	wsIDStr := c.Param("workspaceID")
	workspaceID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_workspace_id",
				"message": "workspace ID must be a valid positive integer",
			},
		})
		return
	}

	fileIDStr := c.Param("fileID")
	fileID, err := strconv.ParseUint(fileIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_file_id",
				"message": "file ID must be a valid positive integer",
			},
		})
		return
	}

	stream, file, dlErr := h.fileService.DownloadFile(c.Request.Context(), workspaceID, fileID)
	if dlErr != nil {
		respondWithError(c, dlErr)
		return
	}
	defer stream.Close()

	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	c.Header("Content-Type", mimeType)
	c.Header("Content-Disposition", "attachment; filename=\""+file.Filename+"\"")
	if file.SizeBytes > 0 {
		c.Header("Content-Length", strconv.FormatInt(file.SizeBytes, 10))
	}

	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, stream)
}

func (h *FileHandler) DeleteFileHandler(c *gin.Context) {
	wsIDStr := c.Param("workspaceID")
	workspaceID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_workspace_id",
				"message": "workspace ID must be a valid positive integer",
			},
		})
		return
	}

	fileIDStr := c.Param("fileID")
	fileID, err := strconv.ParseUint(fileIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_file_id",
				"message": "file ID must be a valid positive integer",
			},
		})
		return
	}

	if delErr := h.fileService.DeleteFile(c.Request.Context(), workspaceID, fileID); delErr != nil {
		respondWithError(c, delErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "file deleted successfully",
		"id":      fileID,
	})
}

func (h *FileHandler) RenameFileHandler(c *gin.Context) {
	wsIDStr := c.Param("workspaceID")
	workspaceID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_workspace_id",
				"message": "workspace ID must be a valid positive integer",
			},
		})
		return
	}

	fileIDStr := c.Param("fileID")
	fileID, err := strconv.ParseUint(fileIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_file_id",
				"message": "file ID must be a valid positive integer",
			},
		})
		return
	}

	var req struct {
		Filename string `json:"filename"`
		Name     string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_json_payload",
				"message": "request body must be valid JSON",
			},
		})
		return
	}

	newFilename := req.Filename
	if newFilename == "" {
		newFilename = req.Name
	}

	file, renameErr := h.fileService.RenameFile(c.Request.Context(), workspaceID, fileID, newFilename)
	if renameErr != nil {
		respondWithError(c, renameErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"file":    file,
	})
}

func (h *FileHandler) MoveFileHandler(c *gin.Context) {
	wsIDStr := c.Param("workspaceID")
	workspaceID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_workspace_id",
				"message": "workspace ID must be a valid positive integer",
			},
		})
		return
	}

	fileIDStr := c.Param("fileID")
	fileID, err := strconv.ParseUint(fileIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_file_id",
				"message": "file ID must be a valid positive integer",
			},
		})
		return
	}

	var req struct {
		ParentID *uint64 `json:"parentId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_json_payload",
				"message": "request body must be valid JSON",
			},
		})
		return
	}

	file, moveErr := h.fileService.MoveFile(c.Request.Context(), workspaceID, fileID, req.ParentID)
	if moveErr != nil {
		respondWithError(c, moveErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"file":    file,
	})
}
