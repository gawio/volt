package handler

import (
	"context"
	"net/http"

	"github.com/gamee1910/volt/internal/application"
	"github.com/gamee1910/volt/internal/interfaces/api/handler/request"
	"github.com/gin-gonic/gin"
)

type ElectricityHandler struct {
	service          *application.ElectricityService
	notifySyncResult func(context.Context, error)
}

func NewElectricityHandler(service *application.ElectricityService) *ElectricityHandler {
	return &ElectricityHandler{service: service}
}

func (h *ElectricityHandler) SetSyncNotifier(notify func(context.Context, error)) {
	h.notifySyncResult = notify
}

func (h *ElectricityHandler) SyncFromEVNHandler(c *gin.Context) {
	var req request.GetUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.FromDate = c.Query("from_date")
		req.ToDate = c.Query("to_date")
	}

	if req.FromDate == "" || req.ToDate == "" {
		writeError(c, http.StatusBadRequest, "from_date and to_date are required")
		return
	}

	syncErr := h.service.Sync(c.Request.Context(), req.FromDate, req.ToDate)
	if h.notifySyncResult != nil {
		h.notifySyncResult(c.Request.Context(), syncErr)
	}
	if err := syncErr; err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sync successful"})
}

func (h *ElectricityHandler) GetAllHandler(c *gin.Context) {
	resp, err := h.service.All(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ElectricityHandler) GetYesterdayUsageHandler(c *gin.Context) {
	resp, err := h.service.Yesterday(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, resp)
}

func writeError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}
