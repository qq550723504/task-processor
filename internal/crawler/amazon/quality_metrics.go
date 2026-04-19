package amazon

import "sync"

type qualityMetricsRecorder interface {
	RecordValidationRetryAttempt()
	RecordValidationRetryRecovered()
	RecordTargetContextSkip(region string)
	RecordTargetContextFallback(region string)
	RecordTargetContextCheckError(region string)
}

type qualityMetrics struct {
	mu                              sync.RWMutex
	validationRetryAttempts         int64
	validationRetryRecovered        int64
	targetContextSkipTotal          int64
	targetContextFallbackTotal      int64
	targetContextCheckErrorTotal    int64
	targetContextSkipByRegion       map[string]int64
	targetContextFallbackByRegion   map[string]int64
	targetContextCheckErrorByRegion map[string]int64
}

func newQualityMetrics() *qualityMetrics {
	return &qualityMetrics{
		targetContextSkipByRegion:       make(map[string]int64),
		targetContextFallbackByRegion:   make(map[string]int64),
		targetContextCheckErrorByRegion: make(map[string]int64),
	}
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

func (m *qualityMetrics) RecordTargetContextSkip(region string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.targetContextSkipTotal++
	m.targetContextSkipByRegion[normalizeMetricsRegion(region)]++
}

func (m *qualityMetrics) RecordTargetContextFallback(region string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.targetContextFallbackTotal++
	m.targetContextFallbackByRegion[normalizeMetricsRegion(region)]++
}

func (m *qualityMetrics) RecordTargetContextCheckError(region string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.targetContextCheckErrorTotal++
	m.targetContextCheckErrorByRegion[normalizeMetricsRegion(region)]++
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
		"target_context_skip_total":                m.targetContextSkipTotal,
		"target_context_fallback_total":            m.targetContextFallbackTotal,
		"target_context_check_error_total":         m.targetContextCheckErrorTotal,
		"target_context_skip_by_region":            copyInt64Map(m.targetContextSkipByRegion),
		"target_context_fallback_by_region":        copyInt64Map(m.targetContextFallbackByRegion),
		"target_context_check_error_by_region":     copyInt64Map(m.targetContextCheckErrorByRegion),
	}
}
