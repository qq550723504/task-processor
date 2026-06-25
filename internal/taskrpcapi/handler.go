package taskrpcapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Client defines the task RPC methods used by the HTTP handler.
type Client interface {
	GetTaskStatus(taskID int64) (*TaskStatusRespDTO, error)
	RetryTask(taskID int64) (*TaskActionRespDTO, error)
	CancelTask(taskID int64) (*TaskActionRespDTO, error)
	GetQueueStats() (string, error)
}

// LocalStatusProvider returns local task-processing health details.
type LocalStatusProvider func() map[string]any

// Handler exposes task RPC operations over the local HTTP API.
type Handler interface {
	GetTaskStatus(c *gin.Context)
	RetryTask(c *gin.Context)
	CancelTask(c *gin.Context)
	GetQueueStats(c *gin.Context)
	GetHealth(c *gin.Context)
}

type handler struct {
	client              Client
	localStatusProvider LocalStatusProvider
}

// NewHandler creates a new task RPC HTTP handler.
func NewHandler(client Client, localStatusProvider LocalStatusProvider) (Handler, error) {
	if client == nil {
		return nil, nil
	}
	return &handler{client: client, localStatusProvider: localStatusProvider}, nil
}

func (h *handler) GetTaskStatus(c *gin.Context) {
	taskID, ok := parseTaskID(c)
	if !ok {
		return
	}

	result, err := h.client.GetTaskStatus(taskID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "taskId": taskID})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *handler) RetryTask(c *gin.Context) {
	taskID, ok := parseTaskID(c)
	if !ok {
		return
	}

	result, err := h.client.RetryTask(taskID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "taskId": taskID})
		return
	}

	statusCode := http.StatusOK
	if !result.Success {
		statusCode = http.StatusBadRequest
	}
	c.JSON(statusCode, result)
}

func (h *handler) CancelTask(c *gin.Context) {
	taskID, ok := parseTaskID(c)
	if !ok {
		return
	}

	result, err := h.client.CancelTask(taskID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "taskId": taskID})
		return
	}

	statusCode := http.StatusOK
	if !result.Success {
		statusCode = http.StatusBadRequest
	}
	c.JSON(statusCode, result)
}

func (h *handler) GetQueueStats(c *gin.Context) {
	result, err := h.client.GetQueueStats()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"queueStats": result})
}

func (h *handler) GetHealth(c *gin.Context) {
	queueStats, err := h.client.GetQueueStats()
	status := "ok"
	statusCode := http.StatusOK
	errorMessage := ""
	if err != nil {
		status = "degraded"
		statusCode = http.StatusBadGateway
		errorMessage = err.Error()
	}

	localWorkers := map[string]any{}
	if h.localStatusProvider != nil {
		localWorkers = h.localStatusProvider()
	}

	c.JSON(statusCode, gin.H{
		"status":       status,
		"timestamp":    time.Now().Format(time.RFC3339),
		"queueStats":   queueStats,
		"localWorkers": localWorkers,
		"errorMessage": errorMessage,
		"source":       "management-task-rpc",
	})
}

func parseTaskID(c *gin.Context) (int64, bool) {
	taskID, err := strconv.ParseInt(c.Param("task_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id"})
		return 0, false
	}
	return taskID, true
}
