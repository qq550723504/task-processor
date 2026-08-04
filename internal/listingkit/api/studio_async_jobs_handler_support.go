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
	forwardedProto := firstForwardedValue(c.GetHeader("X-Forwarded-Proto"))
	if c.Request.TLS != nil || strings.EqualFold(forwardedProto, "https") {
		scheme = "https"
	}
	host := firstForwardedValue(c.GetHeader("X-Forwarded-Host"))
	if !validForwardedHost(host) {
		host = c.Request.Host
	}
	return scheme + "://" + host
}

func firstForwardedValue(value string) string {
	return strings.TrimSpace(strings.Split(value, ",")[0])
}

func validForwardedHost(host string) bool {
	return host != "" && !strings.ContainsAny(host, "\r\n /\\")
}
