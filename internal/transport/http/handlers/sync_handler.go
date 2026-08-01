package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"cumulus/internal/usecase"
	"github.com/gin-gonic/gin"
)

// syncRequestTimeout bounds one sync. Listing metadata is cheap, but a drive
// with tens of thousands of objects across several accounts should not be able
// to hold a request open indefinitely.
const syncRequestTimeout = 90 * time.Second

type SyncHandler struct {
	syncService *usecase.DriveSyncService
}

func NewSyncHandler(syncService *usecase.DriveSyncService) *SyncHandler {
	return &SyncHandler{syncService: syncService}
}

// SyncWorkspaceHandler adopts everything already sitting in the connected
// accounts, so files that were in Drive before Cumulus — or added outside it —
// show up in the workspace.
func (h *SyncHandler) SyncWorkspaceHandler(c *gin.Context) {
	workspaceID, err := strconv.ParseUint(c.Param("workspaceID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "invalid_workspace_id",
				"message": "workspace ID must be a valid positive integer",
			},
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), syncRequestTimeout)
	defer cancel()

	result, syncErr := h.syncService.SyncWorkspace(ctx, workspaceID)
	if syncErr != nil {
		respondWithError(c, syncErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
