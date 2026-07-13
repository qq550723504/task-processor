package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) GetTaskSDSRepair(c *gin.Context) {
	if h.taskSDSRepairService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sds_repair_not_supported", "message": "SDS repair is not supported"})
		return
	}
	session, err := h.taskSDSRepairService.GetTaskSDSRepair(requestContext(c), c.Param("task_id"))
	if err != nil {
		writeSDSRepairError(c, err)
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *handler) RepairAndRetryTaskSDS(c *gin.Context) {
	if h.taskSDSRepairService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sds_repair_not_supported", "message": "SDS repair is not supported"})
		return
	}
	var req listingkit.ApplyTaskSDSRepairRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	result, err := h.taskSDSRepairService.RepairAndRetryTaskSDS(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		writeSDSRepairError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func writeSDSRepairError(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "sds_repair_failed"
	switch {
	case errors.Is(err, listingkit.ErrTaskNotFound):
		status, code = http.StatusNotFound, "task_not_found"
	case errors.Is(err, listingkit.ErrSDSRepairInvalidRequest):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, listingkit.ErrSDSRepairLayerUnavailable):
		status, code = http.StatusUnprocessableEntity, "sds_repair_layer_unavailable"
	case errors.Is(err, listingkit.ErrSDSRepairNotEligible):
		status, code = http.StatusConflict, "sds_repair_not_eligible"
	case errors.Is(err, listingkit.ErrSDSRepairUnavailable):
		status, code = http.StatusServiceUnavailable, "sds_repair_unavailable"
	}
	c.JSON(status, gin.H{"error": code, "message": err.Error()})
}
