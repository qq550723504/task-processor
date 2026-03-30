package amazon

import "sync"

type serviceMetrics struct {
	mu                       sync.RWMutex
	totalFetches             int64
	totalSuccesses           int64
	totalFailures            int64
	retryableFailures        int64
	dedupeSharedHits         int64
	regionGuardOpenTotal     int64
	regionGuardBlockTotal    int64
	successByMode            map[string]int64
	failureByMode            map[string]int64
	successByRegion          map[string]int64
	failureByRegion          map[string]int64
	failureByType            map[string]int64
	retryableByType          map[string]int64
	failureByRegionType      map[string]map[string]int64
	retryableByRegionType    map[string]map[string]int64
	regionGuardOpenByRegion  map[string]int64
	regionGuardBlockByRegion map[string]int64
}

func newServiceMetrics() *serviceMetrics {
	return &serviceMetrics{
		successByMode:            make(map[string]int64),
		failureByMode:            make(map[string]int64),
		successByRegion:          make(map[string]int64),
		failureByRegion:          make(map[string]int64),
		failureByType:            make(map[string]int64),
		retryableByType:          make(map[string]int64),
		failureByRegionType:      make(map[string]map[string]int64),
		retryableByRegionType:    make(map[string]map[string]int64),
		regionGuardOpenByRegion:  make(map[string]int64),
		regionGuardBlockByRegion: make(map[string]int64),
	}
}

func (m *serviceMetrics) RecordSuccess(mode, region string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalFetches++
	m.totalSuccesses++
	m.successByMode[normalizeMetricsMode(mode)]++
	m.successByRegion[normalizeMetricsRegion(region)]++
}

func (m *serviceMetrics) RecordFailure(mode, region string, err error) {
	if m == nil {
		return
	}
	classified := ClassifyFetchError(err)
	errorType := FetchErrorTypeUnknown
	retryable := false
	if classified != nil {
		errorType = classified.ErrorType()
		retryable = classified.RetryableError()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalFetches++
	m.totalFailures++
	m.failureByMode[normalizeMetricsMode(mode)]++
	normalizedRegion := normalizeMetricsRegion(region)
	m.failureByRegion[normalizedRegion]++
	m.failureByType[errorType]++
	incrementNestedCounter(m.failureByRegionType, normalizedRegion, errorType)
	if retryable {
		m.retryableFailures++
		m.retryableByType[errorType]++
		incrementNestedCounter(m.retryableByRegionType, normalizedRegion, errorType)
	}
}

func (m *serviceMetrics) RecordDedupeSharedHit(region string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dedupeSharedHits++
	m.successByRegion[normalizeMetricsRegion(region)] += 0
}

func (m *serviceMetrics) RecordRegionGuardOpen(region string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.regionGuardOpenTotal++
	m.regionGuardOpenByRegion[normalizeMetricsRegion(region)]++
}

func (m *serviceMetrics) RecordRegionGuardBlocked(region string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.regionGuardBlockTotal++
	m.regionGuardBlockByRegion[normalizeMetricsRegion(region)]++
}

func (m *serviceMetrics) Snapshot() map[string]any {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]any{
		"fetch_total":                  m.totalFetches,
		"fetch_success_total":          m.totalSuccesses,
		"fetch_failure_total":          m.totalFailures,
		"retryable_failure_total":      m.retryableFailures,
		"dedupe_shared_hit_total":      m.dedupeSharedHits,
		"region_guard_open_total":      m.regionGuardOpenTotal,
		"region_guard_block_total":     m.regionGuardBlockTotal,
		"success_by_mode":              copyInt64Map(m.successByMode),
		"failure_by_mode":              copyInt64Map(m.failureByMode),
		"success_by_region":            copyInt64Map(m.successByRegion),
		"failure_by_region":            copyInt64Map(m.failureByRegion),
		"failure_by_type":              copyInt64Map(m.failureByType),
		"retryable_failure_by_type":    copyInt64Map(m.retryableByType),
		"failure_by_region_type":       copyNestedInt64Map(m.failureByRegionType),
		"retryable_by_region_type":     copyNestedInt64Map(m.retryableByRegionType),
		"region_guard_open_by_region":  copyInt64Map(m.regionGuardOpenByRegion),
		"region_guard_block_by_region": copyInt64Map(m.regionGuardBlockByRegion),
	}

	if m.totalFetches > 0 {
		stats["fetch_success_rate"] = float64(m.totalSuccesses) / float64(m.totalFetches)
		stats["fetch_failure_rate"] = float64(m.totalFailures) / float64(m.totalFetches)
	} else {
		stats["fetch_success_rate"] = float64(0)
		stats["fetch_failure_rate"] = float64(0)
	}

	return stats
}

func copyInt64Map(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func copyNestedInt64Map(src map[string]map[string]int64) map[string]map[string]int64 {
	dst := make(map[string]map[string]int64, len(src))
	for outerKey, inner := range src {
		dst[outerKey] = copyInt64Map(inner)
	}
	return dst
}

func incrementNestedCounter(target map[string]map[string]int64, outerKey, innerKey string) {
	if _, ok := target[outerKey]; !ok {
		target[outerKey] = make(map[string]int64)
	}
	target[outerKey][innerKey]++
}

func normalizeMetricsMode(mode string) string {
	switch mode {
	case "sync_api", "async_task":
		return mode
	default:
		return "unknown"
	}
}

func normalizeMetricsRegion(region string) string {
	switch region {
	case "us", "uk", "de", "fr", "it", "es", "ca", "jp", "au", "mx", "br", "in", "ae", "sa":
		return region
	case "":
		return "unknown"
	default:
		return region
	}
}
