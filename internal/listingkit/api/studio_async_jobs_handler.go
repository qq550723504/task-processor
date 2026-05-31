package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

type startStudioAsyncJobRequest struct {
	Path      string          `json:"path"`
	Body      json.RawMessage `json:"body"`
	SessionID string          `json:"session_id,omitempty"`
}

type studioAsyncJob struct {
	ID             string                          `json:"job_id"`
	Path           string                          `json:"path"`
	Status         listingkit.StudioAsyncJobStatus `json:"status"`
	Result         any                             `json:"result,omitempty"`
	Error          string                          `json:"error,omitempty"`
	UpstreamStatus int                             `json:"upstream_status,omitempty"`
	CreatedAt      time.Time                       `json:"created_at"`
	UpdatedAt      time.Time                       `json:"updated_at"`
	FinishedAt     *time.Time                      `json:"finished_at,omitempty"`
}

type studioAsyncJobStore struct {
	repo listingkit.StudioAsyncJobRepository
}

func newStudioAsyncJobStore(repo listingkit.StudioAsyncJobRepository) (*studioAsyncJobStore, error) {
	if repo == nil {
		repo = listingkit.NewMemStudioAsyncJobRepository()
	}
	return &studioAsyncJobStore{repo: repo}, nil
}

func (s *studioAsyncJobStore) create(ctx context.Context, path string) (studioAsyncJob, error) {
	now := time.Now().UTC()
	record := &listingkit.StudioAsyncJobRecord{
		ID:        newStudioAsyncJobID(),
		Path:      path,
		Status:    listingkit.StudioAsyncJobStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateStudioAsyncJob(ctx, record); err != nil {
		return studioAsyncJob{}, err
	}
	return mapStudioAsyncJobRecord(record)
}

func (s *studioAsyncJobStore) get(ctx context.Context, id string) (studioAsyncJob, bool) {
	record, err := s.repo.GetStudioAsyncJob(ctx, id)
	if err != nil || record == nil {
		return studioAsyncJob{}, false
	}
	job, err := mapStudioAsyncJobRecord(record)
	if err != nil {
		return studioAsyncJob{}, false
	}
	return job, true
}

func (s *studioAsyncJobStore) succeed(ctx context.Context, id string, result any) {
	_ = s.update(ctx, id, listingkit.StudioAsyncJobStatusSucceeded, result, "", http.StatusOK)
}

func (s *studioAsyncJobStore) fail(ctx context.Context, id string, err error, status int) {
	message := "async job failed"
	if err != nil {
		message = err.Error()
	}
	_ = s.update(ctx, id, listingkit.StudioAsyncJobStatusFailed, nil, message, status)
}

func (s *studioAsyncJobStore) update(ctx context.Context, id string, status listingkit.StudioAsyncJobStatus, result any, message string, upstreamStatus int) error {
	record, err := s.repo.GetStudioAsyncJob(ctx, id)
	if err != nil || record == nil {
		return err
	}
	now := time.Now().UTC()
	record.Status = status
	record.Error = message
	record.UpstreamStatus = upstreamStatus
	record.UpdatedAt = now
	record.FinishedAt = &now
	if err := record.EncodeResult(result); err != nil {
		return err
	}
	return s.repo.UpdateStudioAsyncJob(ctx, record)
}

func mapStudioAsyncJobRecord(record *listingkit.StudioAsyncJobRecord) (studioAsyncJob, error) {
	if record == nil {
		return studioAsyncJob{}, nil
	}
	result, err := record.DecodeResult()
	if err != nil {
		return studioAsyncJob{}, err
	}
	return studioAsyncJob{
		ID:             record.ID,
		Path:           record.Path,
		Status:         record.Status,
		Result:         result,
		Error:          record.Error,
		UpstreamStatus: record.UpstreamStatus,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
		FinishedAt:     record.FinishedAt,
	}, nil
}

func newStudioAsyncJobID() string {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		return strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "")
	}
	return hex.EncodeToString(data[:])
}

var studioAsyncJobLogger = corelogger.GetGlobalLogger("listingkit.studio.async")

var executeStudioDesignBatch = listingkit.ExecuteStudioDesignBatch

func (h *handler) StartStudioAsyncJob(c *gin.Context) {
	var req startStudioAsyncJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.Path = strings.TrimSpace(req.Path)
	if req.Path != "/studio/designs" && req.Path != "/studio/product-images" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "unsupported async job path"})
		return
	}
	if len(req.Body) == 0 {
		req.Body = json.RawMessage(`{}`)
	}
	metric := "design_jobs"
	if req.Path == "/studio/product-images" {
		metric = "product_image_jobs"
	}
	if !h.authorizeSubscriptionUsage(c, listingsubscription.ModuleStudio, metric, 1) {
		return
	}

	reqCtx := requestContext(c)
	job, err := h.studioAsyncJobs.create(reqCtx, req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "async_job_create_failed", "message": err.Error()})
		return
	}
	studioAsyncJobLogger.WithFields(studioAsyncLogFields(reqCtx, logrus.Fields{
		"job_id":       job.ID,
		"path":         req.Path,
		"session_id":   strings.TrimSpace(req.SessionID),
		"body_bytes":   len(req.Body),
		"usage_metric": metric,
	})).Info("studio async job accepted")
	ctx := detachedRequestContext(c)
	baseURL := requestBaseURL(c)
	sessionID := strings.TrimSpace(req.SessionID)
	if req.Path == "/studio/designs" {
		h.syncStudioDesignAsyncJobSession(reqCtx, sessionID, listingkit.StudioAsyncJobStatusRunning, job.ID, "")
	}
	go h.runStudioAsyncJob(ctx, job.ID, req.Path, req.Body, sessionID, baseURL, metric)

	c.JSON(http.StatusAccepted, job)
}

func (h *handler) GetStudioAsyncJob(c *gin.Context) {
	job, ok := h.studioAsyncJobs.get(requestContext(c), c.Param("job_id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "studio async job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *handler) runStudioAsyncJob(ctx context.Context, jobID string, path string, body json.RawMessage, sessionID string, baseURL string, usageMetric string) {
	startedAt := time.Now()
	studioAsyncJobLogger.WithFields(studioAsyncLogFields(ctx, logrus.Fields{
		"job_id":       jobID,
		"path":         path,
		"session_id":   strings.TrimSpace(sessionID),
		"body_bytes":   len(body),
		"usage_metric": usageMetric,
	})).Info("studio async job started")
	var result any
	var err error
	status := http.StatusInternalServerError

	switch path {
	case "/studio/designs":
		var req listingkit.StudioDesignRequest
		if decodeErr := json.Unmarshal(body, &req); decodeErr != nil {
			err = decodeErr
			status = http.StatusBadRequest
			break
		}
		execution, callErr := executeStudioDesignBatch(ctx, h.studioMediaService, listingkit.StudioBatchGenerateExecutionInput{
			Request:   &req,
			SessionID: sessionID,
		})
		if callErr != nil {
			err = callErr
			break
		}
		var response *listingkit.StudioDesignResponse
		if execution != nil {
			response = execution.Response
			sessionID = execution.SessionID
		}
		if response != nil {
			for idx := range response.Images {
				response.Images[idx].ImageURL = absolutizeUploadedImageURLsWithBase(baseURL, []string{response.Images[idx].ImageURL})[0]
			}
		}
		h.syncStudioDesignAsyncJobSession(ctx, sessionID, listingkit.StudioAsyncJobStatusSucceeded, jobID, "")
		result = response
	case "/studio/product-images":
		var req listingkit.StudioProductImageRequest
		if decodeErr := json.Unmarshal(body, &req); decodeErr != nil {
			err = decodeErr
			status = http.StatusBadRequest
			break
		}
		response, callErr := h.studioMediaService.GenerateStudioProductImages(ctx, &req)
		if callErr != nil {
			err = callErr
			break
		}
		if response != nil {
			for idx := range response.Images {
				response.Images[idx].ImageURL = absolutizeUploadedImageURLsWithBase(baseURL, []string{response.Images[idx].ImageURL})[0]
			}
		}
		result = response
	default:
		err = listingkit.ErrTaskNotFound
		status = http.StatusBadRequest
	}

	if err != nil {
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
		}
		if path == "/studio/designs" {
			h.syncStudioDesignAsyncJobSession(ctx, sessionID, listingkit.StudioAsyncJobStatusFailed, jobID, err.Error())
		}
		studioAsyncJobLogger.WithFields(studioAsyncLogFields(ctx, logrus.Fields{
			"job_id":       jobID,
			"path":         path,
			"session_id":   strings.TrimSpace(sessionID),
			"duration_ms":  time.Since(startedAt).Milliseconds(),
			"status_code":  status,
			"usage_metric": usageMetric,
		})).WithError(err).Warn("studio async job failed")
		h.studioAsyncJobs.fail(ctx, jobID, err, status)
		return
	}
	if h.subscriptionService != nil && strings.TrimSpace(usageMetric) != "" {
		_, _ = h.subscriptionService.RecordUsage(ctx, listingkit.TenantIDFromContext(ctx), listingsubscription.ModuleStudio, usageMetric, 1)
	}
	h.studioAsyncJobs.succeed(ctx, jobID, result)
	studioAsyncJobLogger.WithFields(studioAsyncLogFields(ctx, logrus.Fields{
		"job_id":       jobID,
		"path":         path,
		"session_id":   strings.TrimSpace(sessionID),
		"duration_ms":  time.Since(startedAt).Milliseconds(),
		"status_code":  http.StatusOK,
		"usage_metric": usageMetric,
	})).Info("studio async job succeeded")
}

func (h *handler) syncStudioDesignAsyncJobSession(
	ctx context.Context,
	sessionID string,
	jobStatus listingkit.StudioAsyncJobStatus,
	jobID string,
	errMessage string,
) {
	if h == nil || h.studioSessionService == nil || strings.TrimSpace(sessionID) == "" {
		return
	}

	sessionStatus := listingkit.SheinStudioSessionStatusGenerating
	switch jobStatus {
	case listingkit.StudioAsyncJobStatusSucceeded:
		sessionStatus = listingkit.SheinStudioSessionStatusGenerated
	case listingkit.StudioAsyncJobStatusFailed:
		sessionStatus = listingkit.SheinStudioSessionStatusFailed
	}

	trimmedJobID := strings.TrimSpace(jobID)
	trimmedErr := strings.TrimSpace(errMessage)
	_, _ = h.studioSessionService.UpdateStudioSession(ctx, sessionID, &listingkit.UpdateStudioSessionRequest{
		Status:          &sessionStatus,
		GenerationJobID: &trimmedJobID,
		GenerationError: &trimmedErr,
	})
}

func studioAsyncLogFields(ctx context.Context, fields logrus.Fields) logrus.Fields {
	if fields == nil {
		fields = logrus.Fields{}
	}
	for key, value := range listingkit.RequestTraceFromContext(ctx).LogFields() {
		if value == "" || value == 0 {
			continue
		}
		fields[key] = value
	}
	return fields
}

func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
