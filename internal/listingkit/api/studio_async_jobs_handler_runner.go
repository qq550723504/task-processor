package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

var executeStudioDesignBatch = listingkit.ExecuteStudioDesignBatch

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
