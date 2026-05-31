package listingkit

import (
	"context"
	"strconv"
	"strings"
)

type RequestTrace struct {
	BatchRunID string
	BatchID    string
	SessionID  string
	QueueMode  string
	QueueIndex int
	QueueTotal int
}

type requestTraceContextKey struct{}

func WithRequestTrace(ctx context.Context, trace RequestTrace) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized := normalizeRequestTrace(trace)
	if normalized.IsZero() {
		return ctx
	}
	return context.WithValue(ctx, requestTraceContextKey{}, normalized)
}

func RequestTraceFromContext(ctx context.Context) RequestTrace {
	if ctx == nil {
		return RequestTrace{}
	}
	trace, ok := ctx.Value(requestTraceContextKey{}).(RequestTrace)
	if !ok {
		return RequestTrace{}
	}
	return normalizeRequestTrace(trace)
}

func (trace RequestTrace) IsZero() bool {
	return trace.BatchRunID == "" &&
		trace.BatchID == "" &&
		trace.SessionID == "" &&
		trace.QueueMode == "" &&
		trace.QueueIndex == 0 &&
		trace.QueueTotal == 0
}

func (trace RequestTrace) LogFields() map[string]any {
	return map[string]any{
		"batch_run_id": trace.BatchRunID,
		"batch_id":     trace.BatchID,
		"session_id":   trace.SessionID,
		"queue_mode":   trace.QueueMode,
		"queue_index":  trace.QueueIndex,
		"queue_total":  trace.QueueTotal,
	}
}

func ParseRequestTrace(batchRunID string, batchID string, sessionID string, queueMode string, queueIndex string, queueTotal string) RequestTrace {
	return normalizeRequestTrace(RequestTrace{
		BatchRunID: batchRunID,
		BatchID:    batchID,
		SessionID:  sessionID,
		QueueMode:  queueMode,
		QueueIndex: parseTracePositiveInt(queueIndex),
		QueueTotal: parseTracePositiveInt(queueTotal),
	})
}

func normalizeRequestTrace(trace RequestTrace) RequestTrace {
	trace.BatchRunID = strings.TrimSpace(trace.BatchRunID)
	trace.BatchID = strings.TrimSpace(trace.BatchID)
	trace.SessionID = strings.TrimSpace(trace.SessionID)
	trace.QueueMode = strings.TrimSpace(trace.QueueMode)
	if trace.QueueIndex < 0 {
		trace.QueueIndex = 0
	}
	if trace.QueueTotal < 0 {
		trace.QueueTotal = 0
	}
	return trace
}

func parseTracePositiveInt(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}
