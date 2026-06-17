package submission

import (
	"fmt"
	"strings"
	"time"
)

const (
	RetryableReasonCodeOpenAIInsufficientCredits    = "openai_insufficient_credits"
	RetryableReasonCodeOpenAIRateLimited            = "openai_rate_limited"
	RetryableReasonCodeUpstreamTimeout              = "upstream_timeout"
	RetryableReasonCodeUpstreamTransientUnavailable = "upstream_transient_unavailable"
	RetryableReasonCodeWorkerQueueBackpressure      = "worker_queue_backpressure"
	RetryableRecoveryScopeTask                      = "task"
)

type RetryableBlockState struct {
	ReasonCode           string
	ReasonMessage        string
	BlockedAt            time.Time
	LastRetryAt          *time.Time
	NextRetryAt          *time.Time
	RetryAttempts        int
	MaxAutoRetryAttempts int
	RecoveryScope        string
	AutoResumeEnabled    bool
	AutoRetryPaused      bool
}

type RetryableFailurePersistenceRequest struct {
	DefaultRecoveryScope string
	ErrorMessage         string
	Cause                error
	MarkBlockedRetryable func(*RetryableBlockState, string) error
	MarkFailed           func(string) error
}

func CloneRetryableBlockState(src *RetryableBlockState) *RetryableBlockState {
	if src == nil {
		return nil
	}
	cloned := *src
	if src.LastRetryAt != nil {
		lastRetryAt := *src.LastRetryAt
		cloned.LastRetryAt = &lastRetryAt
	}
	if src.NextRetryAt != nil {
		nextRetryAt := *src.NextRetryAt
		cloned.NextRetryAt = &nextRetryAt
	}
	cloned.ReasonCode = strings.TrimSpace(src.ReasonCode)
	cloned.ReasonMessage = strings.TrimSpace(src.ReasonMessage)
	cloned.RecoveryScope = strings.TrimSpace(src.RecoveryScope)
	return &cloned
}

func ClassifyRetryableFailure(err error, defaultRecoveryScope string) (*RetryableBlockState, bool) {
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
		return newRetryableClassification(RetryableReasonCodeOpenAIInsufficientCredits, message, defaultRecoveryScope), true
	case strings.Contains(normalized, "rate limit"),
		strings.Contains(normalized, "rate limited"),
		strings.Contains(normalized, "too many requests"),
		strings.Contains(normalized, "status code: 429"),
		strings.Contains(normalized, "error code: 429"):
		return newRetryableClassification(RetryableReasonCodeOpenAIRateLimited, message, defaultRecoveryScope), true
	case strings.Contains(normalized, "upstream timeout"),
		strings.Contains(normalized, "gateway timeout"),
		strings.Contains(normalized, "context deadline exceeded"),
		strings.Contains(normalized, "i/o timeout"),
		strings.Contains(normalized, "client timeout exceeded while awaiting headers"):
		return newRetryableClassification(RetryableReasonCodeUpstreamTimeout, message, defaultRecoveryScope), true
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
		return newRetryableClassification(RetryableReasonCodeUpstreamTransientUnavailable, message, defaultRecoveryScope), true
	case strings.Contains(normalized, "worker queue"),
		strings.Contains(normalized, "queue full"),
		strings.Contains(message, "工作队列已满"):
		return newRetryableClassification(RetryableReasonCodeWorkerQueueBackpressure, message, defaultRecoveryScope), true
	default:
		return nil, false
	}
}

func BuildReblockedRetryableBlock(previous *RetryableBlockState, classified *RetryableBlockState, recoveredAt time.Time, defaultRecoveryScope string) *RetryableBlockState {
	block := CloneRetryableBlockState(previous)
	if block == nil {
		block = CloneRetryableBlockState(classified)
	}
	if block == nil {
		block = &RetryableBlockState{}
	}
	if classified != nil {
		if strings.TrimSpace(classified.ReasonCode) != "" {
			block.ReasonCode = strings.TrimSpace(classified.ReasonCode)
		}
		if strings.TrimSpace(classified.ReasonMessage) != "" {
			block.ReasonMessage = strings.TrimSpace(classified.ReasonMessage)
		}
		if strings.TrimSpace(classified.RecoveryScope) != "" {
			block.RecoveryScope = strings.TrimSpace(classified.RecoveryScope)
		}
		if block.ReasonCode == "" && classified.AutoResumeEnabled {
			block.AutoResumeEnabled = true
		}
	}
	if block.BlockedAt.IsZero() {
		block.BlockedAt = recoveredAt
	}
	block.RetryAttempts++
	block.LastRetryAt = cloneTimePointer(recoveredAt)
	if strings.TrimSpace(block.RecoveryScope) == "" {
		block.RecoveryScope = strings.TrimSpace(defaultRecoveryScope)
	}
	if block.AutoRetryPaused {
		block.NextRetryAt = nil
		return block
	}
	if block.MaxAutoRetryAttempts > 0 && block.RetryAttempts >= block.MaxAutoRetryAttempts {
		block.AutoRetryPaused = true
		block.NextRetryAt = nil
		return block
	}
	if block.AutoResumeEnabled {
		nextRetryAt := recoveredAt.Add(BoundedEnqueueRetryDelay(block.RetryAttempts))
		block.NextRetryAt = cloneTimePointer(nextRetryAt)
	} else {
		block.NextRetryAt = nil
	}
	return block
}

func BuildBackfilledRetryableBlock(err error, blockedAt time.Time, backfilledAt time.Time, maxAutoRetryAttempts int, defaultRecoveryScope string) (*RetryableBlockState, bool) {
	block, ok := ClassifyRetryableFailure(err, defaultRecoveryScope)
	if !ok {
		return nil, false
	}
	block.BlockedAt = blockedAt.UTC()
	if block.BlockedAt.IsZero() {
		block.BlockedAt = backfilledAt.UTC()
	}
	block.RetryAttempts = 0
	block.LastRetryAt = nil
	block.AutoRetryPaused = false
	block.MaxAutoRetryAttempts = maxAutoRetryAttempts
	nextRetryAt := backfilledAt.UTC().Add(BoundedEnqueueRetryDelay(1))
	block.NextRetryAt = cloneTimePointer(nextRetryAt)
	return block, true
}

func PersistClassifiedRetryableFailure(request RetryableFailurePersistenceRequest) error {
	if block, ok := ClassifyRetryableFailure(request.Cause, request.DefaultRecoveryScope); ok {
		if request.MarkBlockedRetryable == nil {
			return fmt.Errorf("mark blocked retryable callback is not configured")
		}
		if err := request.MarkBlockedRetryable(block, request.ErrorMessage); err != nil {
			return fmt.Errorf("mark blocked retryable: %w", err)
		}
		return nil
	}
	if request.MarkFailed == nil {
		return fmt.Errorf("mark failed callback is not configured")
	}
	if err := request.MarkFailed(request.ErrorMessage); err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}

func newRetryableClassification(reasonCode string, message string, defaultRecoveryScope string) *RetryableBlockState {
	return &RetryableBlockState{
		ReasonCode:        strings.TrimSpace(reasonCode),
		ReasonMessage:     strings.TrimSpace(message),
		RecoveryScope:     strings.TrimSpace(defaultRecoveryScope),
		AutoResumeEnabled: true,
	}
}

func cloneTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}
