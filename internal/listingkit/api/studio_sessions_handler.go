package api

import (
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

type studioSessionHandler struct {
	service          listingkit.StudioSessionHandlerService
	resumeDispatcher *studioBatchResumeDispatcher
}

func NewStudioSessionHandler(service listingkit.StudioSessionHandlerService) (listingkit.StudioSessionHandler, error) {
	if service == nil {
		return nil, errors.New("service cannot be nil")
	}
	return &studioSessionHandler{
		service:          service,
		resumeDispatcher: newStudioBatchResumeDispatcher(),
	}, nil
}

type studioBatchResumeDispatcher struct {
	mu       sync.Mutex
	inFlight map[string]studioBatchResumeSlot
}

type studioBatchResumeSlot struct {
	queued bool
}

func newStudioBatchResumeDispatcher() *studioBatchResumeDispatcher {
	return &studioBatchResumeDispatcher{
		inFlight: make(map[string]studioBatchResumeSlot),
	}
}

func (h *studioSessionHandler) ensureResumeDispatcher() *studioBatchResumeDispatcher {
	if h == nil {
		return nil
	}
	if h.resumeDispatcher == nil {
		h.resumeDispatcher = newStudioBatchResumeDispatcher()
	}
	return h.resumeDispatcher
}

func (d *studioBatchResumeDispatcher) Launch(batchID string, fn func()) bool {
	if d == nil || fn == nil {
		return false
	}
	d.mu.Lock()
	slot, exists := d.inFlight[batchID]
	if exists {
		slot.queued = true
		d.inFlight[batchID] = slot
		d.mu.Unlock()
		return false
	}
	d.inFlight[batchID] = studioBatchResumeSlot{}
	d.mu.Unlock()

	go func() {
		for {
			fn()

			d.mu.Lock()
			slot := d.inFlight[batchID]
			if slot.queued {
				slot.queued = false
				d.inFlight[batchID] = slot
				d.mu.Unlock()
				continue
			}
			delete(d.inFlight, batchID)
			d.mu.Unlock()
			return
		}
	}()
	return true
}

func (h *studioSessionHandler) launchBatchResume(c *gin.Context, batchID string) {
	resumeCtx := detachedRequestContext(c)
	h.ensureResumeDispatcher().Launch(batchID, func() {
		_, _ = h.service.ResumeStudioBatchGeneration(resumeCtx, batchID)
	})
}

func (h *studioSessionHandler) ListStudioSessionGallery(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "240"))
	response, err := h.service.ListStudioSessionGallery(requestContext(c), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "studio_session_gallery_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *studioSessionHandler) ListStudioBatches(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
	response, err := h.service.ListStudioBatches(requestContext(c), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "studio_batch_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *studioSessionHandler) GetStudioBatch(c *gin.Context) {
	batchID := c.Param("batch_id")
	h.launchBatchResume(c, batchID)

	detail, err := h.service.GetStudioBatchDetail(requestContext(c), batchID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrStudioSessionNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "studio_batch_query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) StartStudioBatchGeneration(c *gin.Context) {
	batchID := c.Param("batch_id")
	detail, err := h.service.PrepareStudioBatchGeneration(requestContext(c), batchID)
	if err != nil {
		writeStudioBatchActionError(c, "studio_batch_generate_failed", err)
		return
	}
	h.launchBatchResume(c, batchID)
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) RetryStudioBatchItems(c *gin.Context) {
	var req listingkit.RetryStudioBatchItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	batchID := c.Param("batch_id")
	detail, err := h.service.PrepareRetryStudioBatchItems(requestContext(c), batchID, &req)
	if err != nil {
		writeStudioBatchActionError(c, "studio_batch_retry_failed", err)
		return
	}
	h.launchBatchResume(c, batchID)
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) ApproveStudioBatchDesigns(c *gin.Context) {
	var req listingkit.ApproveStudioBatchDesignsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	detail, err := h.service.ApproveStudioBatchDesigns(requestContext(c), c.Param("batch_id"), &req)
	if err != nil {
		writeStudioBatchActionError(c, "studio_batch_approve_failed", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) CreateStudioBatchTasks(c *gin.Context) {
	var req listingkit.CreateStudioBatchTasksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	batchID := c.Param("batch_id")
	result, err := h.service.PrepareCreateStudioBatchTasks(requestContext(c), batchID, &req)
	if err != nil {
		writeStudioBatchActionError(c, "studio_batch_tasks_failed", err)
		return
	}
	h.launchBatchResume(c, batchID)
	c.JSON(http.StatusAccepted, result)
}

func (h *studioSessionHandler) UpsertStudioBatch(c *gin.Context) {
	var req listingkit.UpsertStudioBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	detail, err := h.service.UpsertStudioBatch(requestContext(c), &req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, listingkit.ErrStudioSessionNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, listingkit.ErrStudioSessionConflict) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": "studio_batch_save_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) DeleteStudioBatch(c *gin.Context) {
	if err := h.service.DeleteStudioBatch(requestContext(c), c.Param("batch_id")); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrStudioSessionNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "studio_batch_delete_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func writeStudioBatchActionError(c *gin.Context, errorCode string, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, listingkit.ErrStudioSessionNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		status = http.StatusNotFound
	case errors.Is(err, listingkit.ErrStudioSessionConflict):
		status = http.StatusConflict
	case errors.Is(err, listingkit.ErrStudioBatchActionValidation):
		status = http.StatusBadRequest
	}
	c.JSON(status, gin.H{"error": errorCode, "message": err.Error()})
}
