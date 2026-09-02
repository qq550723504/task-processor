package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

var executeStudioDesignBatch = listingkit.ExecuteStudioDesignBatch

var studioAsyncJobHeartbeatInterval = time.Minute

var studioAsyncJobHeartbeatFailureRecoveryAfter = 30 * time.Minute

var errStudioAsyncJobHeartbeatLost = errors.New("studio async job heartbeat lost")

func (h *handler) runStudioAsyncJob(ctx context.Context, jobID string, path string, body json.RawMessage, sessionID string, baseURL string, usageMetric string) {
	startedAt := time.Now()
	studioAsyncJobLogger.WithFields(studioAsyncLogFields(ctx, logrus.Fields{
		"job_id":       jobID,
		"path":         path,
		"session_id":   strings.TrimSpace(sessionID),
		"body_bytes":   len(body),
		"usage_metric": usageMetric,
	})).Info("studio async job started")
	jobCtx, cancelJob := context.WithCancelCause(ctx)
	defer cancelJob(nil)
	heartbeatInterval := h.studioAsyncJobHeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = studioAsyncJobHeartbeatInterval
	}
	heartbeatNow := h.studioAsyncJobHeartbeatNow
	if heartbeatNow == nil {
		heartbeatNow = time.Now
	}
	stopHeartbeat := make(chan struct{})
	heartbeatDone := make(chan struct{})
	if h.studioAsyncJobs == nil {
		close(heartbeatDone)
	} else if heartbeatErr := h.studioAsyncJobs.heartbeat(ctx, jobID); heartbeatErr != nil {
		studioAsyncJobLogger.WithFields(studioAsyncLogFields(ctx, logrus.Fields{"job_id": jobID})).WithError(heartbeatErr).Warn("studio async job initial heartbeat failed")
		cancelJob(errStudioAsyncJobHeartbeatLost)
		close(heartbeatDone)
	} else {
		go func() {
			defer close(heartbeatDone)
			ticker := time.NewTicker(heartbeatInterval)
			defer ticker.Stop()
			lastSuccessfulHeartbeatAt := heartbeatNow()
			for {
				select {
				case <-ticker.C:
					if err := h.studioAsyncJobs.heartbeat(ctx, jobID); err != nil {
						studioAsyncJobLogger.WithFields(studioAsyncLogFields(ctx, logrus.Fields{"job_id": jobID})).WithError(err).Warn("studio async job heartbeat failed")
						if heartbeatNow().Sub(lastSuccessfulHeartbeatAt) >= studioAsyncJobHeartbeatFailureRecoveryAfter {
							cancelJob(errStudioAsyncJobHeartbeatLost)
							return
						}
						continue
					}
					lastSuccessfulHeartbeatAt = heartbeatNow()
				case <-stopHeartbeat:
					return
				}
			}
		}()
	}
	defer func() {
		close(stopHeartbeat)
		<-heartbeatDone
	}()
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
		execution, callErr := executeStudioDesignBatch(jobCtx, h.studioMediaService, listingkit.StudioBatchGenerateExecutionInput{
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
				response.Images[idx].ImageURL = publicizeUploadedImageURLsWithBase(baseURL, []string{response.Images[idx].ImageURL})[0]
			}
		}
		h.syncStudioDesignAsyncJobSession(ctx, sessionID, listingkit.StudioAsyncJobStatusSucceeded, jobID, "")
		result = response
	default:
		err = fmt.Errorf("unsupported studio async job path")
		status = http.StatusBadRequest
	}
	if err == nil && jobCtx.Err() != nil {
		err = context.Cause(jobCtx)
		if err == nil {
			err = jobCtx.Err()
		}
		status = http.StatusInternalServerError
	}

	if err != nil {
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
		}
		failureErr := err
		if persistErr := h.studioAsyncJobs.failWithError(ctx, jobID, err, status); persistErr != nil {
			failureErr = errors.Join(failureErr, fmt.Errorf("persist async job failure: %w", persistErr))
		}
		if path == "/studio/designs" {
			h.syncStudioDesignAsyncJobSession(ctx, sessionID, listingkit.StudioAsyncJobStatusFailed, jobID, failureErr.Error())
		}
		studioAsyncJobLogger.WithFields(studioAsyncLogFields(ctx, logrus.Fields{
			"job_id":       jobID,
			"path":         path,
			"session_id":   strings.TrimSpace(sessionID),
			"duration_ms":  time.Since(startedAt).Milliseconds(),
			"status_code":  status,
			"usage_metric": usageMetric,
		})).WithError(failureErr).Warn("studio async job failed")
		return
	}
	if h.subscriptionService != nil && strings.TrimSpace(usageMetric) != "" {
		_, _ = h.subscriptionService.RecordUsage(ctx, listingkit.TenantIDFromContext(ctx), listingsubscription.ModuleStudio, usageMetric, 1)
		if successErr := h.studioAsyncJobs.succeedWithError(ctx, jobID, result); successErr != nil {
			studioAsyncJobLogger.WithFields(studioAsyncLogFields(ctx, logrus.Fields{
				"job_id": jobID, "path": path, "usage_metric": usageMetric,
			})).WithError(successErr).Warn("studio async success persistence failed")
			return
		}
	} else if successErr := h.studioAsyncJobs.succeedWithError(ctx, jobID, result); successErr != nil {
		studioAsyncJobLogger.WithFields(studioAsyncLogFields(ctx, logrus.Fields{
			"job_id": jobID, "path": path, "usage_metric": usageMetric,
		})).WithError(successErr).Warn("studio async success persistence failed")
		return
	}
	studioAsyncJobLogger.WithFields(studioAsyncLogFields(ctx, logrus.Fields{
		"job_id":       jobID,
		"path":         path,
		"session_id":   strings.TrimSpace(sessionID),
		"duration_ms":  time.Since(startedAt).Milliseconds(),
		"status_code":  http.StatusOK,
		"usage_metric": usageMetric,
	})).Info("studio async job succeeded")
}
