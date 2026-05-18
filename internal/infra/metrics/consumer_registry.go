package metrics

import (
	"strconv"
	"sync"
	coremetrics "task-processor/internal/core/metrics"
	"task-processor/internal/infra/rabbitmq"

	"github.com/prometheus/client_golang/prometheus"
)

type ConsumerSnapshot struct {
	Load   rabbitmq.LoadStats
	Task   coremetrics.TaskMetricsSnapshot
	Shein  coremetrics.SheinMetricsSnapshot
	System map[string]float64
}

type ConsumerRegistry struct {
	mu       sync.RWMutex
	registry *prometheus.Registry
	snapshot ConsumerSnapshot

	tasksProcessedTotalDesc                                 *prometheus.Desc
	tasksSucceededTotalDesc                                 *prometheus.Desc
	tasksFailedTotalDesc                                    *prometheus.Desc
	listingTasksProcessingTotalDesc                         *prometheus.Desc
	listingTasksCompletedTotalDesc                          *prometheus.Desc
	listingTasksFailedTotalDesc                             *prometheus.Desc
	listingTasksRequeuedTotalDesc                           *prometheus.Desc
	listingTasksWaitSecondsAvgDesc                          *prometheus.Desc
	listingTasksProcessSecondsAvgDesc                       *prometheus.Desc
	listingTasksPriorityHighTotalDesc                       *prometheus.Desc
	listingTasksPriorityMediumTotalDesc                     *prometheus.Desc
	listingTasksPriorityLowTotalDesc                        *prometheus.Desc
	sheinTasksPublishedTotalDesc                            *prometheus.Desc
	sheinTasksPausedTotalDesc                               *prometheus.Desc
	sheinTasksDraftTotalDesc                                *prometheus.Desc
	sheinTasksTerminatedTotalDesc                           *prometheus.Desc
	sheinReasonAuthExpiredTotalDesc                         *prometheus.Desc
	sheinReasonCookieLoadFailedTotalDesc                    *prometheus.Desc
	sheinReasonDailyLimitReachedTotalDesc                   *prometheus.Desc
	sheinReasonShelfQuotaExhaustedTotalDesc                 *prometheus.Desc
	sheinReasonDraftSavedValidationFailedTotalDesc          *prometheus.Desc
	sheinReasonSkuDuplicatedTotalDesc                       *prometheus.Desc
	sheinReasonFilterRuleRejectedTotalDesc                  *prometheus.Desc
	sheinReasonRetryableFailureTotalDesc                    *prometheus.Desc
	sheinReasonNonRetryableFailureTotalDesc                 *prometheus.Desc
	sheinTopProblemStoreProblemEventsDesc                   *prometheus.Desc
	sheinTopProblemStoreAuthExpiredTotalDesc                *prometheus.Desc
	sheinTopProblemStoreCookieLoadFailedTotalDesc           *prometheus.Desc
	sheinTopProblemStoreDailyLimitReachedTotalDesc          *prometheus.Desc
	sheinTopProblemStoreShelfQuotaExhaustedTotalDesc        *prometheus.Desc
	sheinTopProblemStoreDraftSavedValidationFailedTotalDesc *prometheus.Desc
	sheinTopProblemStoreSkuDuplicatedTotalDesc              *prometheus.Desc
	sheinTopProblemStoreFilterRuleRejectedTotalDesc         *prometheus.Desc
	sheinTopProblemStoreRetryableFailureTotalDesc           *prometheus.Desc
	sheinTopProblemStoreNonRetryableFailureTotalDesc        *prometheus.Desc
	sheinTopSuccessStorePublishedTotalDesc                  *prometheus.Desc
	goroutineCountDesc                                      *prometheus.Desc
	cpuCoresDesc                                            *prometheus.Desc
	memoryHeapBytesDesc                                     *prometheus.Desc
	memorySysBytesDesc                                      *prometheus.Desc
	rabbitmqAvgProcessingTimeSecondsDesc                    *prometheus.Desc
}

func NewConsumerRegistry() *ConsumerRegistry {
	r := &ConsumerRegistry{
		registry: prometheus.NewRegistry(),

		tasksProcessedTotalDesc: prometheus.NewDesc("tasks_processed_total", "Total number of tasks processed", nil, nil),
		tasksSucceededTotalDesc: prometheus.NewDesc("tasks_succeeded_total", "Total number of tasks succeeded", nil, nil),
		tasksFailedTotalDesc:    prometheus.NewDesc("tasks_failed_total", "Total number of tasks failed", nil, nil),

		listingTasksProcessingTotalDesc:     prometheus.NewDesc("listing_tasks_processing_total", "Total number of listing tasks entered processing", nil, nil),
		listingTasksCompletedTotalDesc:      prometheus.NewDesc("listing_tasks_completed_total", "Total number of listing tasks completed", nil, nil),
		listingTasksFailedTotalDesc:         prometheus.NewDesc("listing_tasks_failed_total", "Total number of listing tasks failed", nil, nil),
		listingTasksRequeuedTotalDesc:       prometheus.NewDesc("listing_tasks_requeued_total", "Total number of listing tasks requeued", nil, nil),
		listingTasksWaitSecondsAvgDesc:      prometheus.NewDesc("listing_tasks_wait_seconds_avg", "Average wait time for listing tasks in seconds", nil, nil),
		listingTasksProcessSecondsAvgDesc:   prometheus.NewDesc("listing_tasks_process_seconds_avg", "Average processing time for listing tasks in seconds", nil, nil),
		listingTasksPriorityHighTotalDesc:   prometheus.NewDesc("listing_tasks_priority_high_total", "Total number of high-priority listing tasks", nil, nil),
		listingTasksPriorityMediumTotalDesc: prometheus.NewDesc("listing_tasks_priority_medium_total", "Total number of medium-priority listing tasks", nil, nil),
		listingTasksPriorityLowTotalDesc:    prometheus.NewDesc("listing_tasks_priority_low_total", "Total number of low-priority listing tasks", nil, nil),

		sheinTasksPublishedTotalDesc:                   prometheus.NewDesc("shein_tasks_published_total", "Total number of SHEIN tasks published successfully", nil, nil),
		sheinTasksPausedTotalDesc:                      prometheus.NewDesc("shein_tasks_paused_total", "Total number of SHEIN tasks paused", nil, nil),
		sheinTasksDraftTotalDesc:                       prometheus.NewDesc("shein_tasks_draft_total", "Total number of SHEIN tasks moved to draft", nil, nil),
		sheinTasksTerminatedTotalDesc:                  prometheus.NewDesc("shein_tasks_terminated_total", "Total number of SHEIN tasks terminated", nil, nil),
		sheinReasonAuthExpiredTotalDesc:                prometheus.NewDesc("shein_reason_auth_expired_total", "Total number of SHEIN auth expired events", nil, nil),
		sheinReasonCookieLoadFailedTotalDesc:           prometheus.NewDesc("shein_reason_cookie_load_failed_total", "Total number of SHEIN cookie load failures", nil, nil),
		sheinReasonDailyLimitReachedTotalDesc:          prometheus.NewDesc("shein_reason_daily_limit_reached_total", "Total number of SHEIN daily limit reached events", nil, nil),
		sheinReasonShelfQuotaExhaustedTotalDesc:        prometheus.NewDesc("shein_reason_shelf_quota_exhausted_total", "Total number of SHEIN shelf quota exhausted events", nil, nil),
		sheinReasonDraftSavedValidationFailedTotalDesc: prometheus.NewDesc("shein_reason_draft_saved_validation_failed_total", "Total number of SHEIN draft-saved validation failures", nil, nil),
		sheinReasonSkuDuplicatedTotalDesc:              prometheus.NewDesc("shein_reason_sku_duplicated_total", "Total number of SHEIN duplicated SKU events", nil, nil),
		sheinReasonFilterRuleRejectedTotalDesc:         prometheus.NewDesc("shein_reason_filter_rule_rejected_total", "Total number of SHEIN filter-rule rejections", nil, nil),
		sheinReasonRetryableFailureTotalDesc:           prometheus.NewDesc("shein_reason_retryable_failure_total", "Total number of SHEIN retryable failures", nil, nil),
		sheinReasonNonRetryableFailureTotalDesc:        prometheus.NewDesc("shein_reason_non_retryable_failure_total", "Total number of SHEIN non-retryable failures", nil, nil),

		sheinTopProblemStoreProblemEventsDesc:                   prometheus.NewDesc("shein_top_problem_store_problem_events", "Top SHEIN problem stores by problem events", []string{"rank", "tenant_id", "store_id"}, nil),
		sheinTopProblemStoreAuthExpiredTotalDesc:                prometheus.NewDesc("shein_top_problem_store_auth_expired_total", "Top SHEIN problem stores by auth expired events", []string{"rank", "tenant_id", "store_id"}, nil),
		sheinTopProblemStoreCookieLoadFailedTotalDesc:           prometheus.NewDesc("shein_top_problem_store_cookie_load_failed_total", "Top SHEIN problem stores by cookie load failures", []string{"rank", "tenant_id", "store_id"}, nil),
		sheinTopProblemStoreDailyLimitReachedTotalDesc:          prometheus.NewDesc("shein_top_problem_store_daily_limit_reached_total", "Top SHEIN problem stores by daily limit reached events", []string{"rank", "tenant_id", "store_id"}, nil),
		sheinTopProblemStoreShelfQuotaExhaustedTotalDesc:        prometheus.NewDesc("shein_top_problem_store_shelf_quota_exhausted_total", "Top SHEIN problem stores by shelf quota exhausted events", []string{"rank", "tenant_id", "store_id"}, nil),
		sheinTopProblemStoreDraftSavedValidationFailedTotalDesc: prometheus.NewDesc("shein_top_problem_store_draft_saved_validation_failed_total", "Top SHEIN problem stores by draft validation failures", []string{"rank", "tenant_id", "store_id"}, nil),
		sheinTopProblemStoreSkuDuplicatedTotalDesc:              prometheus.NewDesc("shein_top_problem_store_sku_duplicated_total", "Top SHEIN problem stores by duplicated SKU events", []string{"rank", "tenant_id", "store_id"}, nil),
		sheinTopProblemStoreFilterRuleRejectedTotalDesc:         prometheus.NewDesc("shein_top_problem_store_filter_rule_rejected_total", "Top SHEIN problem stores by filter-rule rejected events", []string{"rank", "tenant_id", "store_id"}, nil),
		sheinTopProblemStoreRetryableFailureTotalDesc:           prometheus.NewDesc("shein_top_problem_store_retryable_failure_total", "Top SHEIN problem stores by retryable failures", []string{"rank", "tenant_id", "store_id"}, nil),
		sheinTopProblemStoreNonRetryableFailureTotalDesc:        prometheus.NewDesc("shein_top_problem_store_non_retryable_failure_total", "Top SHEIN problem stores by non-retryable failures", []string{"rank", "tenant_id", "store_id"}, nil),
		sheinTopSuccessStorePublishedTotalDesc:                  prometheus.NewDesc("shein_top_success_store_published_total", "Top SHEIN success stores by published tasks", []string{"rank", "tenant_id", "store_id"}, nil),

		goroutineCountDesc:                   prometheus.NewDesc("goroutine_count", "Current number of goroutines", nil, nil),
		cpuCoresDesc:                         prometheus.NewDesc("cpu_cores", "Number of CPU cores", nil, nil),
		memoryHeapBytesDesc:                  prometheus.NewDesc("memory_heap_bytes", "Heap memory usage in bytes", nil, nil),
		memorySysBytesDesc:                   prometheus.NewDesc("memory_sys_bytes", "System memory usage in bytes", nil, nil),
		rabbitmqAvgProcessingTimeSecondsDesc: prometheus.NewDesc("rabbitmq_avg_processing_time_seconds", "Average task processing time", nil, nil),
	}
	r.registry.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
		r,
	)
	return r
}

func (r *ConsumerRegistry) UpdateConsumerSnapshot(snapshot ConsumerSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshot = snapshot.clone()
}

func (r *ConsumerRegistry) Registry() *prometheus.Registry {
	return r.registry
}

func (r *ConsumerRegistry) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range r.descriptors() {
		ch <- desc
	}
}

func (r *ConsumerRegistry) Collect(ch chan<- prometheus.Metric) {
	r.mu.RLock()
	snapshot := r.snapshot.clone()
	r.mu.RUnlock()

	ch <- prometheus.MustNewConstMetric(r.tasksProcessedTotalDesc, prometheus.CounterValue, float64(snapshot.Load.TasksProcessed))
	ch <- prometheus.MustNewConstMetric(r.tasksSucceededTotalDesc, prometheus.CounterValue, float64(snapshot.Load.TasksSucceeded))
	ch <- prometheus.MustNewConstMetric(r.tasksFailedTotalDesc, prometheus.CounterValue, float64(snapshot.Load.TasksFailed))

	ch <- prometheus.MustNewConstMetric(r.listingTasksProcessingTotalDesc, prometheus.CounterValue, float64(snapshot.Task.ProcessingCount))
	ch <- prometheus.MustNewConstMetric(r.listingTasksCompletedTotalDesc, prometheus.CounterValue, float64(snapshot.Task.CompletedCount))
	ch <- prometheus.MustNewConstMetric(r.listingTasksFailedTotalDesc, prometheus.CounterValue, float64(snapshot.Task.FailedCount))
	ch <- prometheus.MustNewConstMetric(r.listingTasksRequeuedTotalDesc, prometheus.CounterValue, float64(snapshot.Task.RequeuedCount))
	ch <- prometheus.MustNewConstMetric(r.listingTasksWaitSecondsAvgDesc, prometheus.GaugeValue, averageWaitSeconds(snapshot.Task))
	ch <- prometheus.MustNewConstMetric(r.listingTasksProcessSecondsAvgDesc, prometheus.GaugeValue, averageProcessSeconds(snapshot.Task))
	ch <- prometheus.MustNewConstMetric(r.listingTasksPriorityHighTotalDesc, prometheus.CounterValue, float64(snapshot.Task.HighPriorityCount))
	ch <- prometheus.MustNewConstMetric(r.listingTasksPriorityMediumTotalDesc, prometheus.CounterValue, float64(snapshot.Task.MediumPriorityCount))
	ch <- prometheus.MustNewConstMetric(r.listingTasksPriorityLowTotalDesc, prometheus.CounterValue, float64(snapshot.Task.LowPriorityCount))

	ch <- prometheus.MustNewConstMetric(r.sheinTasksPublishedTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.PublishedCount))
	ch <- prometheus.MustNewConstMetric(r.sheinTasksPausedTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.PausedCount))
	ch <- prometheus.MustNewConstMetric(r.sheinTasksDraftTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.DraftCount))
	ch <- prometheus.MustNewConstMetric(r.sheinTasksTerminatedTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.TerminatedCount))
	ch <- prometheus.MustNewConstMetric(r.sheinReasonAuthExpiredTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.AuthExpiredCount))
	ch <- prometheus.MustNewConstMetric(r.sheinReasonCookieLoadFailedTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.CookieLoadFailedCount))
	ch <- prometheus.MustNewConstMetric(r.sheinReasonDailyLimitReachedTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.DailyLimitReachedCount))
	ch <- prometheus.MustNewConstMetric(r.sheinReasonShelfQuotaExhaustedTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.ShelfQuotaExhaustedCount))
	ch <- prometheus.MustNewConstMetric(r.sheinReasonDraftSavedValidationFailedTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.DraftSavedValidationCount))
	ch <- prometheus.MustNewConstMetric(r.sheinReasonSkuDuplicatedTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.SkuDuplicatedCount))
	ch <- prometheus.MustNewConstMetric(r.sheinReasonFilterRuleRejectedTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.FilterRuleRejectedCount))
	ch <- prometheus.MustNewConstMetric(r.sheinReasonRetryableFailureTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.RetryableFailureCount))
	ch <- prometheus.MustNewConstMetric(r.sheinReasonNonRetryableFailureTotalDesc, prometheus.CounterValue, float64(snapshot.Shein.NonRetryableFailureCount))

	r.collectTopStores(ch, r.sheinTopProblemStoreProblemEventsDesc, snapshot.Shein.TopProblemStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.ProblemEvents })
	r.collectTopStores(ch, r.sheinTopProblemStoreAuthExpiredTotalDesc, snapshot.Shein.TopAuthExpiredStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.AuthExpiredCount })
	r.collectTopStores(ch, r.sheinTopProblemStoreCookieLoadFailedTotalDesc, snapshot.Shein.TopCookieLoadFailedStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.CookieLoadFailedCount })
	r.collectTopStores(ch, r.sheinTopProblemStoreDailyLimitReachedTotalDesc, snapshot.Shein.TopDailyLimitStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.DailyLimitReachedCount })
	r.collectTopStores(ch, r.sheinTopProblemStoreShelfQuotaExhaustedTotalDesc, snapshot.Shein.TopShelfQuotaStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.ShelfQuotaExhaustedCount })
	r.collectTopStores(ch, r.sheinTopProblemStoreDraftSavedValidationFailedTotalDesc, snapshot.Shein.TopDraftValidationStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.DraftSavedValidationCount })
	r.collectTopStores(ch, r.sheinTopProblemStoreSkuDuplicatedTotalDesc, snapshot.Shein.TopSkuDuplicatedStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.SkuDuplicatedCount })
	r.collectTopStores(ch, r.sheinTopProblemStoreFilterRuleRejectedTotalDesc, snapshot.Shein.TopFilterRejectedStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.FilterRuleRejectedCount })
	r.collectTopStores(ch, r.sheinTopProblemStoreRetryableFailureTotalDesc, snapshot.Shein.TopRetryableFailureStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.RetryableFailureCount })
	r.collectTopStores(ch, r.sheinTopProblemStoreNonRetryableFailureTotalDesc, snapshot.Shein.TopNonRetryableStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.NonRetryableFailureCount })
	r.collectTopStores(ch, r.sheinTopSuccessStorePublishedTotalDesc, snapshot.Shein.TopSuccessStores, func(item coremetrics.SheinStoreStatsSnapshot) int64 { return item.PublishedCount })

	r.collectSystemMetric(ch, r.goroutineCountDesc, snapshot.System, "system_goroutines_count")
	r.collectSystemMetric(ch, r.cpuCoresDesc, snapshot.System, "system_cpu_cores")
	r.collectSystemMetric(ch, r.memoryHeapBytesDesc, snapshot.System, "system_memory_heap_bytes")
	r.collectSystemMetric(ch, r.memorySysBytesDesc, snapshot.System, "system_memory_sys_bytes")
	r.collectSystemMetric(ch, r.rabbitmqAvgProcessingTimeSecondsDesc, snapshot.System, "rabbitmq_avg_processing_time_seconds")
}

func (r *ConsumerRegistry) collectTopStores(ch chan<- prometheus.Metric, desc *prometheus.Desc, stores []coremetrics.SheinStoreStatsSnapshot, valueFn func(coremetrics.SheinStoreStatsSnapshot) int64) {
	for idx, store := range stores {
		value := valueFn(store)
		if value <= 0 {
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			desc,
			prometheus.GaugeValue,
			float64(value),
			strconv.Itoa(idx+1),
			strconv.FormatInt(store.TenantID, 10),
			strconv.FormatInt(store.StoreID, 10),
		)
	}
}

func (r *ConsumerRegistry) collectSystemMetric(ch chan<- prometheus.Metric, desc *prometheus.Desc, values map[string]float64, key string) {
	value, ok := values[key]
	if !ok {
		return
	}
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value)
}

func (r *ConsumerRegistry) descriptors() []*prometheus.Desc {
	return []*prometheus.Desc{
		r.tasksProcessedTotalDesc,
		r.tasksSucceededTotalDesc,
		r.tasksFailedTotalDesc,
		r.listingTasksProcessingTotalDesc,
		r.listingTasksCompletedTotalDesc,
		r.listingTasksFailedTotalDesc,
		r.listingTasksRequeuedTotalDesc,
		r.listingTasksWaitSecondsAvgDesc,
		r.listingTasksProcessSecondsAvgDesc,
		r.listingTasksPriorityHighTotalDesc,
		r.listingTasksPriorityMediumTotalDesc,
		r.listingTasksPriorityLowTotalDesc,
		r.sheinTasksPublishedTotalDesc,
		r.sheinTasksPausedTotalDesc,
		r.sheinTasksDraftTotalDesc,
		r.sheinTasksTerminatedTotalDesc,
		r.sheinReasonAuthExpiredTotalDesc,
		r.sheinReasonCookieLoadFailedTotalDesc,
		r.sheinReasonDailyLimitReachedTotalDesc,
		r.sheinReasonShelfQuotaExhaustedTotalDesc,
		r.sheinReasonDraftSavedValidationFailedTotalDesc,
		r.sheinReasonSkuDuplicatedTotalDesc,
		r.sheinReasonFilterRuleRejectedTotalDesc,
		r.sheinReasonRetryableFailureTotalDesc,
		r.sheinReasonNonRetryableFailureTotalDesc,
		r.sheinTopProblemStoreProblemEventsDesc,
		r.sheinTopProblemStoreAuthExpiredTotalDesc,
		r.sheinTopProblemStoreCookieLoadFailedTotalDesc,
		r.sheinTopProblemStoreDailyLimitReachedTotalDesc,
		r.sheinTopProblemStoreShelfQuotaExhaustedTotalDesc,
		r.sheinTopProblemStoreDraftSavedValidationFailedTotalDesc,
		r.sheinTopProblemStoreSkuDuplicatedTotalDesc,
		r.sheinTopProblemStoreFilterRuleRejectedTotalDesc,
		r.sheinTopProblemStoreRetryableFailureTotalDesc,
		r.sheinTopProblemStoreNonRetryableFailureTotalDesc,
		r.sheinTopSuccessStorePublishedTotalDesc,
		r.goroutineCountDesc,
		r.cpuCoresDesc,
		r.memoryHeapBytesDesc,
		r.memorySysBytesDesc,
		r.rabbitmqAvgProcessingTimeSecondsDesc,
	}
}

func (s ConsumerSnapshot) clone() ConsumerSnapshot {
	cloned := s
	if s.System != nil {
		cloned.System = make(map[string]float64, len(s.System))
		for key, value := range s.System {
			cloned.System[key] = value
		}
	}

	cloned.Shein.TopStores = cloneStoreStats(s.Shein.TopStores)
	cloned.Shein.TopSuccessStores = cloneStoreStats(s.Shein.TopSuccessStores)
	cloned.Shein.TopProblemStores = cloneStoreStats(s.Shein.TopProblemStores)
	cloned.Shein.TopAuthExpiredStores = cloneStoreStats(s.Shein.TopAuthExpiredStores)
	cloned.Shein.TopCookieLoadFailedStores = cloneStoreStats(s.Shein.TopCookieLoadFailedStores)
	cloned.Shein.TopDailyLimitStores = cloneStoreStats(s.Shein.TopDailyLimitStores)
	cloned.Shein.TopShelfQuotaStores = cloneStoreStats(s.Shein.TopShelfQuotaStores)
	cloned.Shein.TopDraftValidationStores = cloneStoreStats(s.Shein.TopDraftValidationStores)
	cloned.Shein.TopSkuDuplicatedStores = cloneStoreStats(s.Shein.TopSkuDuplicatedStores)
	cloned.Shein.TopFilterRejectedStores = cloneStoreStats(s.Shein.TopFilterRejectedStores)
	cloned.Shein.TopRetryableFailureStores = cloneStoreStats(s.Shein.TopRetryableFailureStores)
	cloned.Shein.TopNonRetryableStores = cloneStoreStats(s.Shein.TopNonRetryableStores)
	return cloned
}

func cloneStoreStats(items []coremetrics.SheinStoreStatsSnapshot) []coremetrics.SheinStoreStatsSnapshot {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]coremetrics.SheinStoreStatsSnapshot, len(items))
	copy(cloned, items)
	return cloned
}

func averageWaitSeconds(snapshot coremetrics.TaskMetricsSnapshot) float64 {
	if snapshot.TaskCount == 0 {
		return 0
	}
	return snapshot.TotalWaitTime.Seconds() / float64(snapshot.TaskCount)
}

func averageProcessSeconds(snapshot coremetrics.TaskMetricsSnapshot) float64 {
	if snapshot.CompletedCount == 0 {
		return 0
	}
	return snapshot.TotalProcessTime.Seconds() / float64(snapshot.CompletedCount)
}
