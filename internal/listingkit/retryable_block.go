package listingkit

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
)

type RetryableBlock struct {
	ReasonCode           string     `json:"reason_code,omitempty"`
	ReasonMessage        string     `json:"reason_message,omitempty"`
	BlockedAt            time.Time  `json:"blocked_at,omitempty"`
	LastRetryAt          *time.Time `json:"last_retry_at,omitempty"`
	NextRetryAt          *time.Time `json:"next_retry_at,omitempty"`
	RetryAttempts        int        `json:"retry_attempts,omitempty"`
	MaxAutoRetryAttempts int        `json:"max_auto_retry_attempts,omitempty"`
	RecoveryScope        string     `json:"recovery_scope,omitempty"`
	AutoResumeEnabled    bool       `json:"auto_resume_enabled,omitempty"`
	AutoRetryPaused      bool       `json:"auto_retry_paused,omitempty"`
}

func (r RetryableBlock) Value() (driver.Value, error) { return json.Marshal(r) }

func (r *RetryableBlock) Scan(value any) error {
	if value == nil {
		*r = RetryableBlock{}
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, r)
}

func cloneRetryableBlock(src *RetryableBlock) *RetryableBlock {
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

func adaptRetryableBlockState(src *RetryableBlock) *submissiondomain.RetryableBlockState {
	if src == nil {
		return nil
	}
	return &submissiondomain.RetryableBlockState{
		ReasonCode:           src.ReasonCode,
		ReasonMessage:        src.ReasonMessage,
		BlockedAt:            src.BlockedAt,
		LastRetryAt:          src.LastRetryAt,
		NextRetryAt:          src.NextRetryAt,
		RetryAttempts:        src.RetryAttempts,
		MaxAutoRetryAttempts: src.MaxAutoRetryAttempts,
		RecoveryScope:        src.RecoveryScope,
		AutoResumeEnabled:    src.AutoResumeEnabled,
		AutoRetryPaused:      src.AutoRetryPaused,
	}
}

func adaptSubmissionRetryableBlock(src *submissiondomain.RetryableBlockState) *RetryableBlock {
	if src == nil {
		return nil
	}
	return cloneRetryableBlock(&RetryableBlock{
		ReasonCode:           src.ReasonCode,
		ReasonMessage:        src.ReasonMessage,
		BlockedAt:            src.BlockedAt,
		LastRetryAt:          src.LastRetryAt,
		NextRetryAt:          src.NextRetryAt,
		RetryAttempts:        src.RetryAttempts,
		MaxAutoRetryAttempts: src.MaxAutoRetryAttempts,
		RecoveryScope:        src.RecoveryScope,
		AutoResumeEnabled:    src.AutoResumeEnabled,
		AutoRetryPaused:      src.AutoRetryPaused,
	})
}
