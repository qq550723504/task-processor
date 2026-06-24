package listingcontrol

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/listingadmin"
	"task-processor/internal/model"
)

type DispatchTaskRepository interface {
	ListDispatchCandidatesFair(ctx context.Context, req listingadmin.DispatchCandidateRequest) ([]listingadmin.ImportTask, error)
	ClaimForDispatch(ctx context.Context, claim listingadmin.DispatchClaim) (bool, error)
	RecordDispatchDelay(ctx context.Context, delay listingadmin.DispatchDelay) (bool, error)
	RecordDispatchEvent(ctx context.Context, event listingadmin.DispatchEvent) error
	RollbackDispatch(ctx context.Context, taskID int64, previousStatus int16, processingNode, reason string) error
}

type TaskPublisher interface {
	PublishTask(ctx context.Context, task *model.Task) (PublishedDispatch, error)
}

type StoreReadinessProvider interface {
	ListReadiness(ctx context.Context, platform string) ([]StoreReadiness, error)
}

type ClaimTokenGenerator interface {
	NewClaimToken(prefix string, taskID int64) string
}

type SchedulerConfig struct {
	Platform         string
	BatchSize        int
	PerStoreLimit    int
	DryRun           bool
	ClaimTokenPrefix string
}

type Scheduler struct {
	repo           DispatchTaskRepository
	readiness      StoreReadinessProvider
	publisher      TaskPublisher
	config         SchedulerConfig
	TokenGenerator ClaimTokenGenerator
}

func NewScheduler(repo DispatchTaskRepository, readiness StoreReadinessProvider, publisher TaskPublisher, config SchedulerConfig) *Scheduler {
	return &Scheduler{
		repo:           repo,
		readiness:      readiness,
		publisher:      publisher,
		config:         normalizeSchedulerConfig(config),
		TokenGenerator: defaultClaimTokenGenerator{},
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) (DispatchSummary, error) {
	return s.DispatchOnce(ctx)
}

func (s *Scheduler) DispatchOnce(ctx context.Context) (DispatchSummary, error) {
	var summary DispatchSummary
	if s == nil {
		return summary, fmt.Errorf("scheduler is nil")
	}
	if s.repo == nil {
		return summary, fmt.Errorf("dispatch repository is nil")
	}
	if s.readiness == nil {
		return summary, fmt.Errorf("store readiness provider is nil")
	}
	if !s.config.DryRun && s.publisher == nil {
		return summary, fmt.Errorf("task publisher is nil")
	}

	readiness, err := s.readiness.ListReadiness(ctx, s.config.Platform)
	if err != nil {
		return summary, fmt.Errorf("list store readiness: %w", err)
	}
	byStore := readinessByStore(readiness)

	candidates, err := s.repo.ListDispatchCandidatesFair(ctx, listingadmin.DispatchCandidateRequest{
		Platform:      s.config.Platform,
		Limit:         s.config.BatchSize,
		PerStoreLimit: s.config.PerStoreLimit,
	})
	if err != nil {
		return summary, fmt.Errorf("list dispatch candidates: %w", err)
	}
	summary.Candidates = len(candidates)

	localQueued := make(map[storeKey]int64, len(byStore))
	for key, item := range byStore {
		localQueued[key] = item.Queued
	}
	usedClaimTokens := make(map[string]struct{}, len(candidates))

	for _, candidate := range candidates {
		decision := newDecision(candidate)
		key, ok := candidateStoreKey(candidate)
		if !ok {
			decision.Action = DispatchActionSkipped
			decision.Reason = ReasonStoreMissing
			s.addSkippedDecision(ctx, &summary, candidate, decision)
			continue
		}
		ready, ok := byStore[key]
		if !ok {
			decision.Action = DispatchActionSkipped
			decision.Reason = ReasonStoreUnknown
			s.addSkippedDecision(ctx, &summary, candidate, decision)
			continue
		}
		decision.OwnerNode = ready.OwnerNode
		decision.Capacity = ready.Capacity
		decision.Queued = localQueued[key]
		decision.Queue = rabbitmq.GetStoreQueueName(s.config.Platform, key.storeID)
		decision.DailyLimit = ready.DailyLimit
		decision.DailyRemaining = ready.DailyRemaining
		decision.DailyCompleted = ready.DailyCompleted
		decision.DailyProcessing = ready.DailyProcessing
		decision.DailyQueued = ready.DailyQueued

		if !ready.Dispatchable {
			decision.Action = DispatchActionSkipped
			decision.Reason = ready.Reason
			s.addSkippedDecision(ctx, &summary, candidate, decision)
			continue
		}
		if ready.Capacity <= 0 || localQueued[key] >= int64(ready.Capacity) {
			decision.Action = DispatchActionSkipped
			decision.Reason = ReasonNoCapacity
			s.addSkippedDecision(ctx, &summary, candidate, decision)
			continue
		}
		if s.config.DryRun {
			decision.Action = DispatchActionDryRun
			summary.addDecision(decision)
			continue
		}

		token := uniqueClaimToken(s.newClaimToken(candidate.ID), candidate.ID, usedClaimTokens)
		usedClaimTokens[token] = struct{}{}
		claimed, err := s.repo.ClaimForDispatch(ctx, listingadmin.DispatchClaim{
			TaskID:         candidate.ID,
			PreviousStatus: candidate.Status,
			ProcessingNode: token,
			Remark:         "Dispatch queued by listing control-plane scheduler",
		})
		if err != nil {
			decision.Action = DispatchActionFailed
			decision.Reason = fmt.Sprintf("claim dispatch: %v", err)
			summary.addDecision(decision)
			continue
		}
		if !claimed {
			decision.Action = DispatchActionSkipped
			decision.Reason = ReasonClaimConflict
			s.addSkippedDecision(ctx, &summary, candidate, decision)
			continue
		}

		task := importTaskToModelTask(candidate)
		task.Status = model.TaskStatusQueued.Int16()
		published, err := s.publisher.PublishTask(ctx, task)
		if err != nil {
			reason := fmt.Sprintf("publish dispatch: %v", err)
			if rollbackErr := s.repo.RollbackDispatch(ctx, candidate.ID, candidate.Status, token, reason); rollbackErr != nil {
				reason = fmt.Sprintf("%s; rollback dispatch: %v", reason, rollbackErr)
			}
			decision.Action = DispatchActionFailed
			decision.Reason = reason
			summary.addDecision(decision)
			continue
		}

		if strings.TrimSpace(published.Queue) != "" {
			decision.Queue = published.Queue
		}
		decision.Action = DispatchActionDispatched
		s.recordDispatchEvent(ctx, candidate, decision)
		summary.addDecision(decision)
		localQueued[key]++
	}

	return summary, nil
}

func normalizeSchedulerConfig(config SchedulerConfig) SchedulerConfig {
	config.Platform = normalizeDispatchPlatform(config.Platform)
	if config.BatchSize <= 0 {
		config.BatchSize = 500
	}
	if config.PerStoreLimit <= 0 {
		config.PerStoreLimit = 1
	}
	if strings.TrimSpace(config.ClaimTokenPrefix) == "" {
		config.ClaimTokenPrefix = "listing-dispatch"
	}
	return config
}

func (s *Scheduler) newClaimToken(taskID int64) string {
	generator := s.TokenGenerator
	if generator == nil {
		generator = defaultClaimTokenGenerator{}
	}
	token := strings.TrimSpace(generator.NewClaimToken(s.config.ClaimTokenPrefix, taskID))
	if token != "" {
		return token
	}
	return defaultClaimTokenGenerator{}.NewClaimToken(s.config.ClaimTokenPrefix, taskID)
}

func (s *DispatchSummary) addDecision(decision DispatchDecision) {
	s.Decisions = append(s.Decisions, decision)
	switch decision.Action {
	case DispatchActionDispatched:
		s.Dispatched++
	case DispatchActionFailed:
		s.Failed++
	default:
		s.Skipped++
	}
}

func (s *Scheduler) addSkippedDecision(ctx context.Context, summary *DispatchSummary, task listingadmin.ImportTask, decision DispatchDecision) {
	if err := s.recordDispatchDelay(ctx, task, decision); err != nil {
		decision.Action = DispatchActionFailed
		decision.Reason = appendDispatchReason(decision.Reason, fmt.Sprintf("record dispatch delay: %v", err))
	}
	s.recordDispatchEvent(ctx, task, decision)
	summary.addDecision(decision)
}

func (s *Scheduler) recordDispatchDelay(ctx context.Context, task listingadmin.ImportTask, decision DispatchDecision) error {
	if s == nil || s.config.DryRun || decision.Action != DispatchActionSkipped {
		return nil
	}
	reason := strings.TrimSpace(decision.Reason)
	if reason == "" {
		reason = "dispatch_delayed"
	}
	message := fmt.Sprintf("Dispatch delayed: %s", reason)
	_, err := s.repo.RecordDispatchDelay(ctx, listingadmin.DispatchDelay{
		TaskID:        task.ID,
		CurrentStatus: task.Status,
		ReasonCode:    reason,
		Stage:         "dispatch",
		ErrorMessage:  message,
		Remark:        message,
	})
	return err
}

func (s *Scheduler) recordDispatchEvent(ctx context.Context, task listingadmin.ImportTask, decision DispatchDecision) {
	if s == nil || s.config.DryRun {
		return
	}
	_ = s.repo.RecordDispatchEvent(ctx, listingadmin.DispatchEvent{
		TaskID:         task.ID,
		TenantID:       task.TenantID,
		StoreID:        decision.StoreID,
		Platform:       s.config.Platform,
		Action:         decision.Action,
		ReasonCode:     decision.Reason,
		Stage:          "dispatch",
		Capacity:       decision.Capacity,
		Queued:         decision.Queued,
		Processing:     decision.DailyProcessing,
		CompletedToday: decision.DailyCompleted,
		DailyLimit:     decision.DailyLimit,
		OwnerNode:      decision.OwnerNode,
	})
}

func appendDispatchReason(base, extra string) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	if base == "" {
		return extra
	}
	if extra == "" {
		return base
	}
	return base + "; " + extra
}

type storeKey struct {
	tenantID int64
	storeID  int64
}

func readinessByStore(items []StoreReadiness) map[storeKey]StoreReadiness {
	out := make(map[storeKey]StoreReadiness, len(items))
	for _, item := range items {
		out[storeKey{tenantID: item.Store.TenantID, storeID: item.Store.StoreID}] = item
	}
	return out
}

func candidateStoreKey(task listingadmin.ImportTask) (storeKey, bool) {
	if task.StoreID == nil || *task.StoreID == 0 {
		return storeKey{}, false
	}
	return storeKey{tenantID: task.TenantID, storeID: *task.StoreID}, true
}

func newDecision(task listingadmin.ImportTask) DispatchDecision {
	decision := DispatchDecision{
		TaskID:   task.ID,
		TenantID: task.TenantID,
	}
	if task.StoreID != nil {
		decision.StoreID = *task.StoreID
	}
	return decision
}

func importTaskToModelTask(task listingadmin.ImportTask) *model.Task {
	out := &model.Task{
		ID:             task.ID,
		TenantID:       task.TenantID,
		Platform:       firstNonBlank(task.TargetPlatform, task.Platform),
		SourcePlatform: task.SourcePlatform,
		Region:         task.Region,
		ProductID:      task.ProductID,
		Status:         task.Status,
		ErrorMessage:   task.ErrorMessage,
		RetryCount:     task.RetryCount,
		MaxRetryCount:  task.MaxRetryCount,
		Remark:         task.Remark,
		Priority:       task.Priority,
		CreateTime:     timePtrToMillis(task.CreateTime),
		UpdateTime:     timePtrToMillis(task.UpdateTime),
		Creator:        task.Creator,
		Updater:        task.Updater,
	}
	if task.StoreID != nil {
		out.StoreID = *task.StoreID
	}
	if task.CategoryID != nil {
		out.CategoryID = *task.CategoryID
	}
	return out
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func timePtrToMillis(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UnixMilli()
}

func uniqueClaimToken(token string, taskID int64, used map[string]struct{}) string {
	if used == nil {
		return token
	}
	if _, exists := used[token]; !exists {
		return token
	}
	for suffix := 1; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d-%d", token, taskID, suffix)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

type defaultClaimTokenGenerator struct{}

func (defaultClaimTokenGenerator) NewClaimToken(prefix string, taskID int64) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "listing-dispatch"
	}
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("%s-%s-%d-%d", prefix, hostnameForToken(), taskID, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%s-%d-%d-%s", prefix, hostnameForToken(), taskID, time.Now().UnixNano(), hex.EncodeToString(random))
}

func hostnameForToken() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "unknown"
	}
	return host
}
