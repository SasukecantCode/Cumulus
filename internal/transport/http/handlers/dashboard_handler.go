package handlers

import (
	_ "embed"
	"fmt"
	"net/http"

	"cumulus/internal/usecase"
	"github.com/gin-gonic/gin"
)

//go:embed dashboard.html
var dashboardHTML string

func (h *AuthHandler) GetWorkspaceNodesHandler(c *gin.Context) {
	workspaceIDStr := c.Param("workspaceID")
	var workspaceID uint64
	if _, err := fmt.Sscanf(workspaceIDStr, "%d", &workspaceID); err != nil || workspaceID == 0 {
		writeError(c, usecase.NewAppError(usecase.ErrorCodeWorkspaceNotFound, http.StatusBadRequest, "invalid workspace ID", nil))
		return
	}

	ws, nodes, err := h.workspaceService.GetWorkspaceStorageNodes(c.Request.Context(), workspaceID)
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workspace": ws,
		"nodes":     nodes,
	})
}

func (h *AuthHandler) ServeDashboard(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, dashboardHTML)
}
