package listingkit

import "strings"

func classifyRetryableTaskFailure(err error) (*RetryableBlock, bool) {
	if err == nil {
		return nil, false
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		return nil, false
	}

	normalized := strings.ToLower(message)

	switch {
	case strings.Contains(normalized, "insufficient credits"):
		return newRetryableClassification(retryableBlockReasonCodeOpenAIInsufficientCredits, message), true
	case strings.Contains(normalized, "rate limit"),
		strings.Contains(normalized, "rate limited"),
		strings.Contains(normalized, "too many requests"),
		strings.Contains(normalized, "status code: 429"),
		strings.Contains(normalized, "error code: 429"):
		return newRetryableClassification(retryableBlockReasonCodeOpenAIRateLimited, message), true
	case strings.Contains(normalized, "upstream timeout"),
		strings.Contains(normalized, "gateway timeout"),
		strings.Contains(normalized, "context deadline exceeded"),
		strings.Contains(normalized, "i/o timeout"),
		strings.Contains(normalized, "client timeout exceeded while awaiting headers"):
		return newRetryableClassification(retryableBlockReasonCodeUpstreamTimeout, message), true
	case strings.Contains(normalized, "temporarily unavailable"),
		strings.Contains(normalized, "service unavailable"),
		strings.Contains(normalized, "upstream unavailable"),
		strings.Contains(normalized, "no such host"),
		strings.Contains(normalized, "connection reset"),
		strings.Contains(normalized, "connection refused"),
		strings.Contains(normalized, "connection timed out"),
		strings.Contains(normalized, "tls handshake timeout"),
		normalized == "eof",
		strings.Contains(normalized, "unexpected eof"),
		strings.Contains(normalized, `": eof`),
		strings.Contains(normalized, "dial tcp") && strings.Contains(normalized, ": eof"),
		strings.Contains(normalized, "read tcp") && strings.Contains(normalized, ": eof"),
		strings.Contains(normalized, "write tcp") && strings.Contains(normalized, ": eof"),
		strings.Contains(normalized, "request failed: eof"),
		strings.Contains(normalized, "request error: eof"):
		return newRetryableClassification(retryableBlockReasonCodeUpstreamTransientUnavailable, message), true
	case strings.Contains(normalized, "worker queue"),
		strings.Contains(normalized, "queue full"),
		strings.Contains(message, "工作队列已满"):
		return newRetryableClassification(retryableBlockReasonCodeWorkerQueueBackpressure, message), true
	default:
		return nil, false
	}
}

func newRetryableClassification(reasonCode string, message string) *RetryableBlock {
	return &RetryableBlock{
		ReasonCode:        strings.TrimSpace(reasonCode),
		ReasonMessage:     strings.TrimSpace(message),
		RecoveryScope:     retryableRecoveryScopeTask,
		AutoResumeEnabled: true,
	}
}
