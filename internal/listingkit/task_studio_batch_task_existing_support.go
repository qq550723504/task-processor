package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"task-processor/internal/listingkit/core"
	"time"

	"gorm.io/gorm"
)

const (
	studioBatchTaskLinkStatusReserved                = "reserved"
	studioBatchTaskLinkStatusCreating                = "creating"
	studioBatchTaskLinkStatusCreated                 = "created"
	studioBatchTaskLinkStatusFailed                  = "failed"
	studioBatchTaskLinkSourceBatchCreated            = "batch_created"
	studioBatchTaskLinkSourceLegacySessionBackfilled = "legacy_session_backfilled"
	studioBatchTaskLinkSourceRejected                = "rejected"
	studioBatchCreatedTaskStatus                     = "task_created"
	studioBatchReusedTaskReasonCode                  = "__studio_batch_reused"
	studioBatchTaskCreatingWait                      = 5 * time.Second
)

func (s *taskStudioBatchService) findExistingStudioBatchTask(
	ctx context.Context,
	recorded SheinStudioCreatedTaskList,
	candidate studioBatchTaskCandidate,
) (SheinStudioCreatedTask, bool) {
	if s == nil {
		return SheinStudioCreatedTask{}, false
	}
	if task, ok := s.findDurableStudioBatchTask(ctx, candidate); ok {
		return markStudioBatchReusedTask(task), true
	}
	if s.getTask == nil || len(recorded) == 0 {
		return SheinStudioCreatedTask{}, false
	}
	created, ok, err := s.findLegacyStudioBatchTask(ctx, recorded, candidate)
	if ok && err == nil {
		return markStudioBatchReusedTask(created), true
	}
	return SheinStudioCreatedTask{}, false
}

func (s *taskStudioBatchService) findLegacyStudioBatchTask(
	ctx context.Context,
	recorded SheinStudioCreatedTaskList,
	candidate studioBatchTaskCandidate,
) (SheinStudioCreatedTask, bool, error) {
	if s == nil || s.getTask == nil || len(recorded) == 0 {
		return SheinStudioCreatedTask{}, false, nil
	}
	designID := strings.TrimSpace(candidate.Design.ID)
	for _, created := range recorded {
		if strings.TrimSpace(created.DesignID) != designID || strings.TrimSpace(created.ID) == "" {
			continue
		}
		// Legacy session records do not persist the effective product-image
		// configuration. Do not reuse an AI task through this fallback because
		// its prompt/mode/count/role prompts cannot be proven compatible.
		if normalizeSheinImageStrategy(candidate.ImageStrategy) == sheinImageStrategyAIGenerated &&
			strings.TrimSpace(candidate.ProductImageSettingsFingerprint) != "" {
			continue
		}
		task, err := s.getTask(ctx, created.ID)
		if err != nil || task == nil || task.Status == core.TaskStatusFailed {
			continue
		}
		if !studioBatchTaskMatchesSelection(task, candidate) {
			continue
		}
		if strings.TrimSpace(created.Title) == "" {
			created.Title = candidate.Title
		}
		created = normalizeStudioBatchCreatedTask(created, candidate)
		if err := s.persistStudioBatchTaskLink(ctx, candidate, created.ID, studioBatchTaskLinkStatusCreated, studioBatchTaskLinkSourceLegacySessionBackfilled, "", ""); err != nil {
			return SheinStudioCreatedTask{}, false, err
		}
		created.Source = studioBatchTaskLinkSourceLegacySessionBackfilled
		return created, true, nil
	}
	return SheinStudioCreatedTask{}, false, nil
}

func (s *taskStudioBatchService) findDurableStudioBatchTask(ctx context.Context, candidate studioBatchTaskCandidate) (SheinStudioCreatedTask, bool) {
	if s == nil || s.batchTaskLinkRepo == nil {
		return SheinStudioCreatedTask{}, false
	}
	candidateKey := strings.TrimSpace(candidate.CandidateKey)
	if candidateKey == "" {
		return SheinStudioCreatedTask{}, false
	}
	candidateKeys := []string{candidateKey}
	if historicalKey := strings.TrimSpace(candidate.HistoricalCandidateKey); historicalKey != "" && historicalKey != candidateKey {
		candidateKeys = append(candidateKeys, historicalKey)
	}
	deadline := time.Now().Add(studioBatchTaskCreatingWait)
	for {
		creating := false
		for _, lookupKey := range candidateKeys {
			link, err := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, lookupKey)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if err != nil || link == nil {
				return SheinStudioCreatedTask{}, false
			}
			if task, ok := s.createdTaskFromDurableLink(ctx, link, candidate); ok {
				return task, true
			}
			if link.Status == studioBatchTaskLinkStatusCreating && !s.studioBatchTaskLinkIsStale(link) &&
				(lookupKey == candidateKey || studioBatchTaskLinkHasStoredImageStrategy(link, candidate)) {
				creating = true
			}
		}
		if !creating || time.Now().After(deadline) {
			return SheinStudioCreatedTask{}, false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func studioBatchTaskLinkHasStoredImageStrategy(link *StudioBatchTaskLinkRecord, candidate studioBatchTaskCandidate) bool {
	if link == nil || strings.TrimSpace(link.ImageStrategy) == "" {
		return false
	}
	return normalizeSheinImageStrategy(link.ImageStrategy) == normalizeSheinImageStrategy(candidate.ImageStrategy)
}

func (s *taskStudioBatchService) createdTaskFromDurableLink(ctx context.Context, link *StudioBatchTaskLinkRecord, candidate studioBatchTaskCandidate) (SheinStudioCreatedTask, bool) {
	if link == nil || strings.TrimSpace(link.ListingKitTaskID) == "" {
		return SheinStudioCreatedTask{}, false
	}
	// A persisted strategy is authoritative for the historical link. Reject a
	// mismatched candidate before fetching the linked task so a transient task
	// read error cannot invalidate an unrelated durable link.
	if storedStrategy := strings.TrimSpace(link.ImageStrategy); storedStrategy != "" &&
		normalizeSheinImageStrategy(storedStrategy) != normalizeSheinImageStrategy(candidate.ImageStrategy) {
		return SheinStudioCreatedTask{}, false
	}
	// Historical AI links can be found through the SDS candidate key. Their
	// legacy tuple predates the product-image settings fingerprint, so strategy
	// and generated-image checks alone cannot prove prompt/mode/count/theme
	// compatibility. Require the extended identity before reusing any AI link.
	if normalizeSheinImageStrategy(candidate.ImageStrategy) == sheinImageStrategyAIGenerated &&
		!studioBatchTaskLinkMatchesProductImageSettings(link, candidate) {
		return SheinStudioCreatedTask{}, false
	}
	var task *Task
	if s != nil && s.getTask != nil {
		var err error
		task, err = s.getTask(ctx, link.ListingKitTaskID)
		if err != nil || task == nil || task.Status == core.TaskStatusFailed {
			if link.Status == studioBatchTaskLinkStatusCreated {
				link.Status = studioBatchTaskLinkStatusFailed
				link.ReasonCode = "linked_task_invalid"
				link.Message = "durable link points to a missing or failed ListingKit task"
				link.UpdatedAt = s.currentTime().UTC()
				_ = s.batchTaskLinkRepo.UpdateStudioBatchTaskLink(ctx, link)
			}
			return SheinStudioCreatedTask{}, false
		}
	}
	if !studioBatchTaskLinkMatchesImageStrategy(link, task, candidate) {
		return SheinStudioCreatedTask{}, false
	}
	if link.Status != studioBatchTaskLinkStatusCreated {
		link.Status = studioBatchTaskLinkStatusCreated
		link.ReasonCode = ""
		link.Message = ""
		if s != nil {
			link.UpdatedAt = s.currentTime().UTC()
		}
		_ = s.batchTaskLinkRepo.UpdateStudioBatchTaskLink(ctx, link)
	}
	created := normalizeStudioBatchCreatedTask(SheinStudioCreatedTask{
		ID:                       link.ListingKitTaskID,
		DesignID:                 link.DesignID,
		ItemID:                   link.ItemID,
		SelectionID:              link.SelectionID,
		CompatibilityFingerprint: link.CompatibilityFingerprint,
		Status:                   studioBatchCreatedTaskStatus,
		Source:                   resolveStudioBatchTaskLinkSource(link, task),
		ReasonCode:               link.ReasonCode,
		Message:                  link.Message,
	}, candidate)
	created = projectStudioBatchCreatedTaskFromListingTask(created, task)
	return created, true
}

func studioBatchTaskLinkMatchesProductImageSettings(link *StudioBatchTaskLinkRecord, candidate studioBatchTaskCandidate) bool {
	if link == nil || normalizeSheinImageStrategy(candidate.ImageStrategy) != sheinImageStrategyAIGenerated {
		return true
	}
	settings := strings.TrimSpace(candidate.ProductImageSettingsFingerprint)
	if settings == "" {
		return false
	}
	return strings.TrimSpace(link.CompatibilityFingerprint) == studioBatchTaskLinkCompatibilityFingerprint(candidate)
}

func resolveStudioBatchTaskLinkSource(link *StudioBatchTaskLinkRecord, task *Task) string {
	if link == nil {
		return ""
	}
	if source := strings.TrimSpace(link.Source); source != "" {
		return source
	}
	if task == nil || task.Request == nil || task.Request.Options == nil || task.Request.Options.SheinStudio == nil {
		return ""
	}
	styleID := strings.TrimSpace(task.Request.Options.SheinStudio.StyleID)
	if styleID == "" {
		return ""
	}
	if styleID == buildStudioBatchTaskStyleID(strings.TrimSpace(link.DesignID)) {
		return studioBatchTaskLinkSourceBatchCreated
	}
	return studioBatchTaskLinkSourceLegacySessionBackfilled
}

func (s *taskStudioBatchService) studioBatchTaskLinkIsStale(link *StudioBatchTaskLinkRecord) bool {
	if s == nil || link == nil || link.Status != studioBatchTaskLinkStatusCreating {
		return false
	}
	if strings.TrimSpace(link.ListingKitTaskID) != "" {
		return false
	}
	return s.currentTime().UTC().Sub(link.UpdatedAt.UTC()) > 2*time.Minute
}

func normalizeStudioBatchCreatedTask(task SheinStudioCreatedTask, candidate studioBatchTaskCandidate) SheinStudioCreatedTask {
	if strings.TrimSpace(task.Title) == "" {
		task.Title = candidate.Title
	}
	if strings.TrimSpace(task.DesignID) == "" {
		task.DesignID = candidate.Design.ID
	}
	if strings.TrimSpace(task.ItemID) == "" {
		task.ItemID = candidate.Item.ID
	}
	if strings.TrimSpace(task.SelectionID) == "" {
		task.SelectionID = candidate.SelectionID
	}
	if strings.TrimSpace(task.CompatibilityFingerprint) == "" {
		task.CompatibilityFingerprint = candidate.CompatibilityFingerprint
	}
	if strings.TrimSpace(task.Status) == "" {
		task.Status = studioBatchCreatedTaskStatus
	}
	return task
}

func markStudioBatchReusedTask(task SheinStudioCreatedTask) SheinStudioCreatedTask {
	if strings.TrimSpace(task.ReasonCode) == "" {
		task.ReasonCode = studioBatchReusedTaskReasonCode
	}
	return task
}

func (s *taskStudioBatchService) persistStudioBatchTaskLink(ctx context.Context, candidate studioBatchTaskCandidate, taskID string, status string, source string, reasonCode string, message string) error {
	if s == nil || s.batchTaskLinkRepo == nil {
		return nil
	}
	now := s.currentTime().UTC()
	link := &StudioBatchTaskLinkRecord{
		ID:                       buildStudioBatchTaskLinkID(candidate),
		BatchID:                  strings.TrimSpace(candidate.Design.BatchID),
		ItemID:                   strings.TrimSpace(candidate.Item.ID),
		DesignID:                 strings.TrimSpace(candidate.Design.ID),
		SelectionID:              strings.TrimSpace(candidate.SelectionID),
		CompatibilityFingerprint: studioBatchTaskLinkCompatibilityFingerprint(candidate),
		ImageStrategy:            normalizeSheinImageStrategy(candidate.ImageStrategy),
		SheinStoreID:             candidate.SheinStoreID,
		ListingKitTaskID:         strings.TrimSpace(taskID),
		CandidateKey:             strings.TrimSpace(candidate.CandidateKey),
		ClaimToken:               "",
		Status:                   strings.TrimSpace(status),
		Source:                   strings.TrimSpace(source),
		ReasonCode:               strings.TrimSpace(reasonCode),
		Message:                  strings.TrimSpace(message),
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if strings.TrimSpace(candidate.ClaimToken) != "" {
		// Retain the completed transition's ownership token internally so a
		// heartbeat that was already in flight can distinguish this worker's
		// terminal update from a replacement worker's update.
		link.ClaimToken = strings.TrimSpace(candidate.ClaimToken)
	}
	if link.BatchID == "" && candidate.Item.BatchID != "" {
		link.BatchID = strings.TrimSpace(candidate.Item.BatchID)
	}
	if err := s.batchTaskLinkRepo.CreateStudioBatchTaskLink(ctx, link); err == nil {
		return nil
	}
	existing, err := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil {
		return err
	}
	var linkedTask *Task
	if s.getTask != nil && strings.TrimSpace(existing.ListingKitTaskID) != "" {
		linkedTask, _ = s.getTask(ctx, existing.ListingKitTaskID)
	}
	if !studioBatchTaskLinkMatchesImageStrategy(existing, linkedTask, candidate) {
		candidate.CandidateKey = buildDisambiguatedStudioBatchTaskCandidateKey(candidate)
		link.ID = buildStudioBatchTaskLinkID(candidate)
		link.CandidateKey = candidate.CandidateKey
		if createErr := s.batchTaskLinkRepo.CreateStudioBatchTaskLink(ctx, link); createErr == nil {
			return nil
		}
		existing, err = s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
		if err != nil {
			return err
		}
		linkedTask = nil
		if s.getTask != nil && strings.TrimSpace(existing.ListingKitTaskID) != "" {
			linkedTask, _ = s.getTask(ctx, existing.ListingKitTaskID)
		}
		if !studioBatchTaskLinkMatchesImageStrategy(existing, linkedTask, candidate) {
			return fmt.Errorf("studio batch task link strategy mismatch for candidate %s", candidate.CandidateKey)
		}
	}
	existing.ListingKitTaskID = link.ListingKitTaskID
	existing.Status = link.Status
	if link.Source != "" {
		existing.Source = link.Source
	}
	existing.ReasonCode = link.ReasonCode
	existing.Message = link.Message
	if strings.TrimSpace(candidate.ClaimToken) != "" {
		existing.ClaimToken = strings.TrimSpace(candidate.ClaimToken)
	} else {
		existing.ClaimToken = link.ClaimToken
	}
	existing.UpdatedAt = now
	if claimToken := strings.TrimSpace(candidate.ClaimToken); claimToken != "" {
		leaseRepo, ok := s.batchTaskLinkRepo.(studioBatchTaskLinkLeaseRepository)
		if !ok {
			return fmt.Errorf("studio batch task link repository does not support lease-token updates")
		}
		updated, updateErr := leaseRepo.UpdateStudioBatchTaskLinkWithClaimToken(ctx, existing, claimToken)
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			return fmt.Errorf("studio batch task claim is no longer owned")
		}
		return nil
	}
	return s.batchTaskLinkRepo.UpdateStudioBatchTaskLink(ctx, existing)
}

func buildStudioBatchTaskLinkID(candidate studioBatchTaskCandidate) string {
	key := strings.TrimSpace(candidate.CandidateKey)
	if len(key) > 24 {
		key = key[:24]
	}
	return "sbtlink_" + key
}

func studioBatchTaskMatchesSelection(
	task *Task,
	candidate studioBatchTaskCandidate,
) bool {
	if task == nil || task.Request == nil || task.Request.Options == nil {
		return false
	}
	studio := task.Request.Options.SheinStudio
	sds := task.Request.Options.SDS
	if studio == nil || sds == nil {
		return false
	}
	if normalizeSheinImageStrategy(resolveSheinImageStrategy(task.Request)) != normalizeSheinImageStrategy(candidate.ImageStrategy) {
		return false
	}
	if normalizeSheinImageStrategy(candidate.ImageStrategy) == sheinImageStrategyAIGenerated && !studioBatchTaskHasGeneratedProductImages(task) {
		return false
	}
	styleID := strings.TrimSpace(studio.StyleID)
	if styleID != candidate.StyleID && styleID != buildStudioBatchTaskStyleID(candidate.Design.ID) {
		return false
	}
	if len(task.Request.ImageURLs) == 0 || strings.TrimSpace(task.Request.ImageURLs[0]) != strings.TrimSpace(candidate.Design.ImageURL) {
		return false
	}
	return sds.VariantID == candidate.SelectionSnapshot.VariantID &&
		sds.ParentProductID == candidate.SelectionSnapshot.ParentProductID &&
		sds.PrototypeGroupID == candidate.SelectionSnapshot.PrototypeGroupID &&
		strings.TrimSpace(sds.LayerID) == strings.TrimSpace(candidate.SelectionSnapshot.LayerID)
}

func studioBatchTaskLinkMatchesImageStrategy(link *StudioBatchTaskLinkRecord, task *Task, candidate studioBatchTaskCandidate) bool {
	if link == nil {
		return false
	}
	candidateStrategy := normalizeSheinImageStrategy(candidate.ImageStrategy)
	if candidateStrategy == sheinImageStrategyAIGenerated && strings.TrimSpace(link.ListingKitTaskID) != "" && !studioBatchTaskHasGeneratedProductImages(task) {
		return false
	}
	if storedStrategy := strings.TrimSpace(link.ImageStrategy); storedStrategy != "" {
		return normalizeSheinImageStrategy(storedStrategy) == candidateStrategy
	}
	if task == nil || task.Request == nil {
		return strings.TrimSpace(link.ListingKitTaskID) == "" && link.Status != studioBatchTaskLinkStatusCreated && candidateStrategy == sheinImageStrategySDSOfficial
	}
	return resolveSheinImageStrategy(task.Request) == candidateStrategy
}

func studioBatchTaskHasGeneratedProductImages(task *Task) bool {
	if task == nil || task.Request == nil || task.Request.Options == nil || task.Request.Options.SheinStudio == nil {
		return false
	}
	for _, raw := range task.Request.Options.SheinStudio.ProductImageURLs {
		if strings.TrimSpace(raw) != "" {
			return true
		}
	}
	return false
}

func mergeStudioCreatedTasks(
	existing SheinStudioCreatedTaskList,
	created []SheinStudioCreatedTask,
) SheinStudioCreatedTaskList {
	if len(existing) == 0 && len(created) == 0 {
		return nil
	}
	merged := make(SheinStudioCreatedTaskList, 0, len(existing)+len(created))
	seen := make(map[string]int, len(existing)+len(created))
	appendOrMerge := func(task SheinStudioCreatedTask) {
		id := strings.TrimSpace(task.ID)
		if id == "" {
			return
		}
		if index, ok := seen[id]; ok {
			merged[index] = mergeStudioCreatedTaskMetadata(merged[index], task)
			return
		}
		seen[id] = len(merged)
		merged = append(merged, task)
	}
	for _, task := range existing {
		appendOrMerge(task)
	}
	for _, task := range created {
		appendOrMerge(task)
	}
	return merged
}

func mergeStudioCreatedTaskMetadata(base SheinStudioCreatedTask, richer SheinStudioCreatedTask) SheinStudioCreatedTask {
	if strings.TrimSpace(richer.Title) != "" {
		base.Title = richer.Title
	}
	if strings.TrimSpace(base.DesignID) == "" {
		base.DesignID = richer.DesignID
	}
	if strings.TrimSpace(base.ItemID) == "" {
		base.ItemID = richer.ItemID
	}
	if strings.TrimSpace(base.SelectionID) == "" {
		base.SelectionID = richer.SelectionID
	}
	if strings.TrimSpace(base.CompatibilityFingerprint) == "" {
		base.CompatibilityFingerprint = richer.CompatibilityFingerprint
	}
	if strings.TrimSpace(richer.Status) != "" && strings.TrimSpace(base.Status) == studioBatchCreatedTaskStatus {
		base.Status = richer.Status
	}
	if strings.TrimSpace(base.SubmissionState) == "" {
		base.SubmissionState = richer.SubmissionState
	}
	if strings.TrimSpace(base.LastSubmissionAction) == "" {
		base.LastSubmissionAction = richer.LastSubmissionAction
	}
	if strings.TrimSpace(base.Source) == "" {
		base.Source = richer.Source
	}
	if strings.TrimSpace(base.ReasonCode) == "" {
		base.ReasonCode = richer.ReasonCode
	}
	if strings.TrimSpace(richer.Message) != "" {
		base.Message = richer.Message
	}
	return base
}
