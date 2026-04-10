package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"task-processor/internal/model"
)

type SheinMetrics struct {
	mu sync.RWMutex

	PublishedCount            int64
	PausedCount               int64
	DraftCount                int64
	TerminatedCount           int64
	AuthExpiredCount          int64
	CookieLoadFailedCount     int64
	DailyLimitReachedCount    int64
	ShelfQuotaExhaustedCount  int64
	DraftSavedValidationCount int64
	SkuDuplicatedCount        int64
	FilterRuleRejectedCount   int64
	RetryableFailureCount     int64
	NonRetryableFailureCount  int64

	StoreStats map[string]*SheinStoreStats
}

type SheinMetricsSnapshot struct {
	PublishedCount            int64
	PausedCount               int64
	DraftCount                int64
	TerminatedCount           int64
	AuthExpiredCount          int64
	CookieLoadFailedCount     int64
	DailyLimitReachedCount    int64
	ShelfQuotaExhaustedCount  int64
	DraftSavedValidationCount int64
	SkuDuplicatedCount        int64
	FilterRuleRejectedCount   int64
	RetryableFailureCount     int64
	NonRetryableFailureCount  int64
	TopStores                 []SheinStoreStatsSnapshot
	TopSuccessStores          []SheinStoreStatsSnapshot
	TopProblemStores          []SheinStoreStatsSnapshot
	TopAuthExpiredStores      []SheinStoreStatsSnapshot
	TopCookieLoadFailedStores []SheinStoreStatsSnapshot
	TopDailyLimitStores       []SheinStoreStatsSnapshot
	TopShelfQuotaStores       []SheinStoreStatsSnapshot
	TopDraftValidationStores  []SheinStoreStatsSnapshot
	TopSkuDuplicatedStores    []SheinStoreStatsSnapshot
	TopFilterRejectedStores   []SheinStoreStatsSnapshot
	TopRetryableFailureStores []SheinStoreStatsSnapshot
	TopNonRetryableStores     []SheinStoreStatsSnapshot
}

type SheinStoreStats struct {
	TenantID                  int64
	StoreID                   int64
	PublishedCount            int64
	PausedCount               int64
	DraftCount                int64
	TerminatedCount           int64
	AuthExpiredCount          int64
	CookieLoadFailedCount     int64
	DailyLimitReachedCount    int64
	ShelfQuotaExhaustedCount  int64
	DraftSavedValidationCount int64
	SkuDuplicatedCount        int64
	FilterRuleRejectedCount   int64
	RetryableFailureCount     int64
	NonRetryableFailureCount  int64
}

type SheinStoreStatsSnapshot struct {
	TenantID                  int64 `json:"tenant_id"`
	StoreID                   int64 `json:"store_id"`
	PublishedCount            int64 `json:"published_count"`
	PausedCount               int64 `json:"paused_count"`
	DraftCount                int64 `json:"draft_count"`
	TerminatedCount           int64 `json:"terminated_count"`
	AuthExpiredCount          int64 `json:"auth_expired_count"`
	CookieLoadFailedCount     int64 `json:"cookie_load_failed_count"`
	DailyLimitReachedCount    int64 `json:"daily_limit_reached_count"`
	ShelfQuotaExhaustedCount  int64 `json:"shelf_quota_exhausted_count"`
	DraftSavedValidationCount int64 `json:"draft_saved_validation_count"`
	SkuDuplicatedCount        int64 `json:"sku_duplicated_count"`
	FilterRuleRejectedCount   int64 `json:"filter_rule_rejected_count"`
	RetryableFailureCount     int64 `json:"retryable_failure_count"`
	NonRetryableFailureCount  int64 `json:"non_retryable_failure_count"`
	TotalEvents               int64 `json:"total_events"`
	ProblemEvents             int64 `json:"problem_events"`
}

var globalSheinMetrics = &SheinMetrics{
	StoreStats: make(map[string]*SheinStoreStats),
}

func GlobalSheinMetrics() *SheinMetrics {
	return globalSheinMetrics
}

func (m *SheinMetrics) IncrementPublished() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PublishedCount++
}

func (m *SheinMetrics) IncrementPublishedForStore(tenantID, storeID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PublishedCount++
	m.ensureStoreStatsLocked(tenantID, storeID).PublishedCount++
}

func (m *SheinMetrics) IncrementHandledStatus(status model.TaskStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch status {
	case model.TaskStatusPublished:
		m.PublishedCount++
	case model.TaskStatusPaused:
		m.PausedCount++
	case model.TaskStatusDraft:
		m.DraftCount++
	case model.TaskStatusTerminated:
		m.TerminatedCount++
	}
}

func (m *SheinMetrics) IncrementHandledStatusForStore(tenantID, storeID int64, status model.TaskStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	storeStats := m.ensureStoreStatsLocked(tenantID, storeID)
	switch status {
	case model.TaskStatusPublished:
		m.PublishedCount++
		storeStats.PublishedCount++
	case model.TaskStatusPaused:
		m.PausedCount++
		storeStats.PausedCount++
	case model.TaskStatusDraft:
		m.DraftCount++
		storeStats.DraftCount++
	case model.TaskStatusTerminated:
		m.TerminatedCount++
		storeStats.TerminatedCount++
	}
}

func (m *SheinMetrics) IncrementReason(reasonCode string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch strings.TrimSpace(reasonCode) {
	case "AUTH_EXPIRED":
		m.AuthExpiredCount++
	case "COOKIE_LOAD_FAILED":
		m.CookieLoadFailedCount++
	case "DAILY_LIMIT_REACHED":
		m.DailyLimitReachedCount++
	case "SHELF_QUOTA_EXHAUSTED":
		m.ShelfQuotaExhaustedCount++
	case "DRAFT_SAVED_VALIDATION_FAILED":
		m.DraftSavedValidationCount++
	case "SKU_DUPLICATED":
		m.SkuDuplicatedCount++
	case "FILTER_RULE_REJECTED":
		m.FilterRuleRejectedCount++
	case "RETRYABLE_FAILURE":
		m.RetryableFailureCount++
	case "NON_RETRYABLE_FAILURE":
		m.NonRetryableFailureCount++
	}
}

func (m *SheinMetrics) IncrementReasonForStore(tenantID, storeID int64, reasonCode string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	storeStats := m.ensureStoreStatsLocked(tenantID, storeID)
	switch strings.TrimSpace(reasonCode) {
	case "AUTH_EXPIRED":
		m.AuthExpiredCount++
		storeStats.AuthExpiredCount++
	case "COOKIE_LOAD_FAILED":
		m.CookieLoadFailedCount++
		storeStats.CookieLoadFailedCount++
	case "DAILY_LIMIT_REACHED":
		m.DailyLimitReachedCount++
		storeStats.DailyLimitReachedCount++
	case "SHELF_QUOTA_EXHAUSTED":
		m.ShelfQuotaExhaustedCount++
		storeStats.ShelfQuotaExhaustedCount++
	case "DRAFT_SAVED_VALIDATION_FAILED":
		m.DraftSavedValidationCount++
		storeStats.DraftSavedValidationCount++
	case "SKU_DUPLICATED":
		m.SkuDuplicatedCount++
		storeStats.SkuDuplicatedCount++
	case "FILTER_RULE_REJECTED":
		m.FilterRuleRejectedCount++
		storeStats.FilterRuleRejectedCount++
	case "RETRYABLE_FAILURE":
		m.RetryableFailureCount++
		storeStats.RetryableFailureCount++
	case "NON_RETRYABLE_FAILURE":
		m.NonRetryableFailureCount++
		storeStats.NonRetryableFailureCount++
	}
}

func (m *SheinMetrics) GetSnapshot() SheinMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allStores := m.storeSnapshotsLocked()

	return SheinMetricsSnapshot{
		PublishedCount:            m.PublishedCount,
		PausedCount:               m.PausedCount,
		DraftCount:                m.DraftCount,
		TerminatedCount:           m.TerminatedCount,
		AuthExpiredCount:          m.AuthExpiredCount,
		CookieLoadFailedCount:     m.CookieLoadFailedCount,
		DailyLimitReachedCount:    m.DailyLimitReachedCount,
		ShelfQuotaExhaustedCount:  m.ShelfQuotaExhaustedCount,
		DraftSavedValidationCount: m.DraftSavedValidationCount,
		SkuDuplicatedCount:        m.SkuDuplicatedCount,
		FilterRuleRejectedCount:   m.FilterRuleRejectedCount,
		RetryableFailureCount:     m.RetryableFailureCount,
		NonRetryableFailureCount:  m.NonRetryableFailureCount,
		TopStores:                 sortStoreSnapshots(allStores, 10, compareTotalEventsDesc),
		TopSuccessStores:          sortStoreSnapshots(allStores, 10, compareSuccessDesc),
		TopProblemStores:          sortStoreSnapshots(allStores, 10, compareProblemDesc),
		TopAuthExpiredStores:      sortStoreSnapshots(allStores, 10, compareMetricDesc(func(item SheinStoreStatsSnapshot) int64 { return item.AuthExpiredCount })),
		TopCookieLoadFailedStores: sortStoreSnapshots(allStores, 10, compareMetricDesc(func(item SheinStoreStatsSnapshot) int64 { return item.CookieLoadFailedCount })),
		TopDailyLimitStores:       sortStoreSnapshots(allStores, 10, compareMetricDesc(func(item SheinStoreStatsSnapshot) int64 { return item.DailyLimitReachedCount })),
		TopShelfQuotaStores:       sortStoreSnapshots(allStores, 10, compareMetricDesc(func(item SheinStoreStatsSnapshot) int64 { return item.ShelfQuotaExhaustedCount })),
		TopDraftValidationStores:  sortStoreSnapshots(allStores, 10, compareMetricDesc(func(item SheinStoreStatsSnapshot) int64 { return item.DraftSavedValidationCount })),
		TopSkuDuplicatedStores:    sortStoreSnapshots(allStores, 10, compareMetricDesc(func(item SheinStoreStatsSnapshot) int64 { return item.SkuDuplicatedCount })),
		TopFilterRejectedStores:   sortStoreSnapshots(allStores, 10, compareMetricDesc(func(item SheinStoreStatsSnapshot) int64 { return item.FilterRuleRejectedCount })),
		TopRetryableFailureStores: sortStoreSnapshots(allStores, 10, compareMetricDesc(func(item SheinStoreStatsSnapshot) int64 { return item.RetryableFailureCount })),
		TopNonRetryableStores:     sortStoreSnapshots(allStores, 10, compareMetricDesc(func(item SheinStoreStatsSnapshot) int64 { return item.NonRetryableFailureCount })),
	}
}

func (m *SheinMetrics) ensureStoreStatsLocked(tenantID, storeID int64) *SheinStoreStats {
	key := fmt.Sprintf("%d:%d", tenantID, storeID)
	storeStats, ok := m.StoreStats[key]
	if !ok {
		storeStats = &SheinStoreStats{TenantID: tenantID, StoreID: storeID}
		m.StoreStats[key] = storeStats
	}
	return storeStats
}

func (m *SheinMetrics) storeSnapshotsLocked() []SheinStoreStatsSnapshot {
	items := make([]SheinStoreStatsSnapshot, 0, len(m.StoreStats))
	for _, stats := range m.StoreStats {
		items = append(items, SheinStoreStatsSnapshot{
			TenantID:                  stats.TenantID,
			StoreID:                   stats.StoreID,
			PublishedCount:            stats.PublishedCount,
			PausedCount:               stats.PausedCount,
			DraftCount:                stats.DraftCount,
			TerminatedCount:           stats.TerminatedCount,
			AuthExpiredCount:          stats.AuthExpiredCount,
			CookieLoadFailedCount:     stats.CookieLoadFailedCount,
			DailyLimitReachedCount:    stats.DailyLimitReachedCount,
			ShelfQuotaExhaustedCount:  stats.ShelfQuotaExhaustedCount,
			DraftSavedValidationCount: stats.DraftSavedValidationCount,
			SkuDuplicatedCount:        stats.SkuDuplicatedCount,
			FilterRuleRejectedCount:   stats.FilterRuleRejectedCount,
			RetryableFailureCount:     stats.RetryableFailureCount,
			NonRetryableFailureCount:  stats.NonRetryableFailureCount,
			TotalEvents: stats.PublishedCount + stats.PausedCount + stats.DraftCount + stats.TerminatedCount +
				stats.AuthExpiredCount + stats.CookieLoadFailedCount + stats.DailyLimitReachedCount +
				stats.ShelfQuotaExhaustedCount + stats.DraftSavedValidationCount + stats.SkuDuplicatedCount +
				stats.FilterRuleRejectedCount + stats.RetryableFailureCount + stats.NonRetryableFailureCount,
			ProblemEvents: stats.PausedCount + stats.DraftCount + stats.TerminatedCount +
				stats.AuthExpiredCount + stats.CookieLoadFailedCount + stats.DailyLimitReachedCount +
				stats.ShelfQuotaExhaustedCount + stats.DraftSavedValidationCount + stats.SkuDuplicatedCount +
				stats.FilterRuleRejectedCount + stats.RetryableFailureCount + stats.NonRetryableFailureCount,
		})
	}
	return items
}

func sortStoreSnapshots(items []SheinStoreStatsSnapshot, limit int, less func(a, b SheinStoreStatsSnapshot) bool) []SheinStoreStatsSnapshot {
	if limit <= 0 || len(items) == 0 {
		return nil
	}
	sorted := append([]SheinStoreStatsSnapshot(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		return less(sorted[i], sorted[j])
	})
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted
}

func compareTotalEventsDesc(a, b SheinStoreStatsSnapshot) bool {
	if a.TotalEvents == b.TotalEvents {
		if a.ProblemEvents == b.ProblemEvents {
			return a.StoreID < b.StoreID
		}
		return a.ProblemEvents > b.ProblemEvents
	}
	return a.TotalEvents > b.TotalEvents
}

func compareSuccessDesc(a, b SheinStoreStatsSnapshot) bool {
	if a.PublishedCount == b.PublishedCount {
		if a.TotalEvents == b.TotalEvents {
			return a.StoreID < b.StoreID
		}
		return a.TotalEvents > b.TotalEvents
	}
	return a.PublishedCount > b.PublishedCount
}

func compareProblemDesc(a, b SheinStoreStatsSnapshot) bool {
	if a.ProblemEvents == b.ProblemEvents {
		if a.AuthExpiredCount == b.AuthExpiredCount {
			return a.StoreID < b.StoreID
		}
		return a.AuthExpiredCount > b.AuthExpiredCount
	}
	return a.ProblemEvents > b.ProblemEvents
}

func compareMetricDesc(valueFn func(item SheinStoreStatsSnapshot) int64) func(a, b SheinStoreStatsSnapshot) bool {
	return func(a, b SheinStoreStatsSnapshot) bool {
		av := valueFn(a)
		bv := valueFn(b)
		if av == bv {
			if a.ProblemEvents == b.ProblemEvents {
				if a.TotalEvents == b.TotalEvents {
					return a.StoreID < b.StoreID
				}
				return a.TotalEvents > b.TotalEvents
			}
			return a.ProblemEvents > b.ProblemEvents
		}
		return av > bv
	}
}

func ExtractReasonCode(message string) string {
	for _, part := range strings.Fields(strings.TrimSpace(message)) {
		if !strings.HasPrefix(part, "[") || !strings.HasSuffix(part, "]") {
			continue
		}
		code := strings.TrimSuffix(strings.TrimPrefix(part, "["), "]")
		if strings.HasPrefix(code, "stage:") {
			continue
		}
		if code != "" {
			return code
		}
	}
	return ""
}
