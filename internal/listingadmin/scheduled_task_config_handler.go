package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type ScheduledTaskConfigHandler struct{ repo ScheduledTaskConfigRepository }

func NewScheduledTaskConfigHandler(repo ScheduledTaskConfigRepository) *ScheduledTaskConfigHandler {
	return &ScheduledTaskConfigHandler{repo: repo}
}

func (h *ScheduledTaskConfigHandler) ListScheduledTaskConfigs(c *gin.Context) {
	scope := requestListScope(c)
	query := applyListQueryScope(&ScheduledTaskConfigQuery{
		Platform: strings.TrimSpace(c.Query("platform")),
		TaskType: strings.TrimSpace(c.Query("taskType")),
	}, scope)
	var ok bool
	query.StoreID, ok = queryInt64PtrStrict(c, "storeId", "invalid_store_id")
	if !ok {
		return
	}
	query.Enabled, ok = queryBoolPtrStrict(c, "enabled", "invalid_enabled")
	if !ok {
		return
	}
	page, err := h.repo.ListScheduledTaskConfigs(requestIdentityContext(c), *query)
	if err != nil {
		writeInternalHandlerError(c, "scheduled_task_config_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *ScheduledTaskConfigHandler) GetScheduledTaskConfig(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	config, err := h.repo.GetScheduledTaskConfig(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeScheduledTaskConfigError(c, err, "scheduled_task_config_get_failed")
		return
	}
	c.JSON(http.StatusOK, config)
}

func (h *ScheduledTaskConfigHandler) UpsertScheduledTaskConfig(c *gin.Context) {
	var req ScheduledTaskConfig
	if !bindAndValidateJSON(c, &req, "invalid_scheduled_task_config", func(value *ScheduledTaskConfig) {
		value.TenantID = requestTenantID(c)
	}, validateScheduledTaskConfig) {
		return
	}
	config, err := h.repo.UpsertScheduledTaskConfig(requestIdentityContext(c), &req)
	if err != nil {
		writeInternalHandlerError(c, "scheduled_task_config_upsert_failed", err)
		return
	}
	c.JSON(http.StatusOK, config)
}

func (h *ScheduledTaskConfigHandler) UpdateScheduledTaskConfigStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Enabled bool   `json:"enabled"`
		Remark  string `json:"remark"`
	}
	if !bindJSON(c, &req) {
		return
	}
	config, err := h.repo.UpdateScheduledTaskConfigStatus(requestIdentityContext(c), requestTenantID(c), id, req.Enabled, req.Remark)
	if err != nil {
		writeScheduledTaskConfigError(c, err, "scheduled_task_config_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, config)
}

func (h *ScheduledTaskConfigHandler) DeleteScheduledTaskConfig(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteScheduledTaskConfig(requestIdentityContext(c), requestTenantID(c), id); err != nil {
		writeScheduledTaskConfigError(c, err, "scheduled_task_config_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validateScheduledTaskConfig(config *ScheduledTaskConfig) error {
	switch {
	case config.TenantID <= 0:
		return errors.New("tenant id is required")
	case config.StoreID <= 0:
		return errors.New("storeId is required")
	case strings.TrimSpace(config.Platform) == "":
		return errors.New("platform is required")
	case strings.TrimSpace(config.TaskType) == "":
		return errors.New("taskType is required")
	case config.IntervalSeconds <= 0:
		return errors.New("intervalSeconds must be positive")
	}
	return nil
}

var writeScheduledTaskConfigError = newMappedHandlerErrorWriter(
	handlerErrorRule{match: ErrScheduledTaskConfigNotFound, status: http.StatusNotFound, errorCode: "scheduled_task_config_not_found"},
)
