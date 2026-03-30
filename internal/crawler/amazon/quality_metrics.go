package amazon

import "sync"

type qualityMetricsRecorder interface {
	RecordValidationRetryAttempt()
	RecordValidationRetryRecovered()
}

type qualityMetrics struct {
	mu                       sync.RWMutex
	validationRetryAttempts  int64
	validationRetryRecovered int64
}

func newQualityMetrics() *qualityMetrics {
	return &qualityMetrics{}
}

func (m *qualityMetrics) RecordValidationRetryAttempt() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validationRetryAttempts++
}

func (m *qualityMetrics) RecordValidationRetryRecovered() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validationRetryRecovered++
}

func (m *qualityMetrics) Snapshot() map[string]any {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	recoveryRate := float64(0)
	if m.validationRetryAttempts > 0 {
		recoveryRate = float64(m.validationRetryRecovered) / float64(m.validationRetryAttempts)
	}

	return map[string]any{
		"quality_validation_retry_attempt_total":   m.validationRetryAttempts,
		"quality_validation_retry_recovered_total": m.validationRetryRecovered,
		"quality_validation_retry_recovery_rate":   recoveryRate,
	}
}
