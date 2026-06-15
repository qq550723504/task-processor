package api

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/listingkit"
)

var studioAsyncJobLogger = corelogger.GetGlobalLogger("listingkit.studio.async")

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
	_ = h.studioSessionService.SyncStudioDesignAsyncJob(ctx, sessionID, jobStatus, jobID, errMessage)
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
