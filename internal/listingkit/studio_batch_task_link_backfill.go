package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type StudioBatchTaskLinkBackfillSessionRepository interface {
	ListBatchSessions(ctx context.Context, limit int) ([]SheinStudioSession, error)
}

type StudioBatchTaskLinkBackfillTaskGetter interface {
	GetTask(ctx context.Context, taskID string) (*Task, error)
}

type StudioBatchTaskLinkBackfillConfig struct {
	SessionRepository StudioBatchTaskLinkBackfillSessionRepository
	BatchRepository   StudioBatchRepository
	TaskGetter        StudioBatchTaskLinkBackfillTaskGetter
	LinkRepository    StudioBatchTaskLinkRepository
	Limit             int
	Now               func() time.Time
}

type StudioBatchTaskLinkBackfillSummary struct {
	SessionsScanned              int                                         `json:"sessions_scanned"`
	LinksCreated                 int                                         `json:"links_created"`
	LinksAlreadyPresent          int                                         `json:"links_already_present"`
	MissingTasks                 []StudioBatchTaskLinkBackfillIssue          `json:"missing_tasks,omitempty"`
	UnresolvedSelectionOwnership []StudioBatchTaskLinkBackfillIssue          `json:"unresolved_selection_ownership,omitempty"`
	Errors                       []StudioBatchTaskLinkBackfillReconcileError `json:"errors,omitempty"`
}

type StudioBatchTaskLinkBackfillIssue struct {
	SessionID string `json:"session_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	DesignID  string `json:"design_id,omitempty"`
	ItemID    string `json:"item_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
}

type StudioBatchTaskLinkBackfillReconcileError struct {
	SessionID string `json:"session_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
}

func BackfillLegacyStudioBatchTaskLinks(ctx context.Context, cfg StudioBatchTaskLinkBackfillConfig) (*StudioBatchTaskLinkBackfillSummary, error) {
	if cfg.SessionRepository == nil {
		return nil, fmt.Errorf("studio batch task link backfill session repository is required")
	}
	if cfg.BatchRepository == nil {
		return nil, fmt.Errorf("studio batch task link backfill batch repository is required")
	}
	if cfg.TaskGetter == nil {
		return nil, fmt.Errorf("studio batch task link backfill task getter is required")
	}
	if cfg.LinkRepository == nil {
		return nil, fmt.Errorf("studio batch task link backfill link repository is required")
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	summary := &StudioBatchTaskLinkBackfillSummary{}
	sessions, err := cfg.SessionRepository.ListBatchSessions(ctx, cfg.Limit)
	if err != nil {
		return summary, err
	}
	for _, session := range sessions {
		summary.SessionsScanned++
		if len(session.CreatedTasks) == 0 {
			continue
		}
		if err := backfillLegacyStudioBatchTaskLinksForSession(ctx, cfg, now, summary, session); err != nil {
			summary.Errors = append(summary.Errors, StudioBatchTaskLinkBackfillReconcileError{
				SessionID: strings.TrimSpace(session.ID),
				Reason:    "session_failed",
				Message:   err.Error(),
			})
		}
	}
	return summary, nil
}

func backfillLegacyStudioBatchTaskLinksForSession(
	ctx context.Context,
	cfg StudioBatchTaskLinkBackfillConfig,
	now func() time.Time,
	summary *StudioBatchTaskLinkBackfillSummary,
	session SheinStudioSession,
) error {
	batchID := strings.TrimSpace(session.ID)
	if batchID == "" {
		summary.Errors = append(summary.Errors, StudioBatchTaskLinkBackfillReconcileError{
			Reason:  "missing_batch_id",
			Message: "legacy studio batch session has an empty id",
		})
		return nil
	}
	detail, err := cfg.BatchRepository.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			for _, created := range session.CreatedTasks {
				summary.UnresolvedSelectionOwnership = append(summary.UnresolvedSelectionOwnership, backfillIssue(session, created, "batch_not_found", "batch detail was not found in the current tenant scope"))
			}
			return nil
		}
		return err
	}
	if detail == nil || detail.Batch == nil {
		summary.UnresolvedSelectionOwnership = append(summary.UnresolvedSelectionOwnership, StudioBatchTaskLinkBackfillIssue{
			SessionID: batchID,
			Reason:    "batch_not_found",
			Message:   "batch detail is empty",
		})
		return nil
	}
	candidatesByDesign := buildStudioBatchBackfillCandidatesByDesign(ctx, &session, detail)
	for _, created := range session.CreatedTasks {
		backfillLegacyStudioBatchCreatedTask(ctx, cfg, now, summary, session, candidatesByDesign, created)
	}
	return nil
}

func backfillLegacyStudioBatchCreatedTask(
	ctx context.Context,
	cfg StudioBatchTaskLinkBackfillConfig,
	now func() time.Time,
	summary *StudioBatchTaskLinkBackfillSummary,
	session SheinStudioSession,
	candidatesByDesign map[string][]studioBatchTaskCandidate,
	created SheinStudioCreatedTask,
) {
	taskID := strings.TrimSpace(created.ID)
	if taskID == "" {
		summary.MissingTasks = append(summary.MissingTasks, backfillIssue(session, created, "missing_task_id", "legacy created task does not include a ListingKit task id"))
		return
	}
	task, taskErr := cfg.TaskGetter.GetTask(ctx, taskID)
	if taskErr != nil || task == nil {
		task = nil
	}
	candidate, ok := resolveStudioBatchBackfillCandidate(created, task, candidatesByDesign)
	if !ok {
		summary.UnresolvedSelectionOwnership = append(summary.UnresolvedSelectionOwnership, backfillIssue(session, created, "candidate_unresolved", "legacy task could not be matched to a batch item/selection candidate"))
		return
	}
	existing, err := cfg.LinkRepository.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		summary.Errors = append(summary.Errors, backfillError(session, created, "link_lookup_failed", err))
		return
	}
	if existing != nil && err == nil {
		summary.LinksAlreadyPresent++
		return
	}
	if taskErr != nil || task == nil {
		summary.MissingTasks = append(summary.MissingTasks, backfillIssue(session, created, "task_not_found", "legacy created task is not visible in the current tenant scope"))
		return
	}
	if !studioBatchBackfillTaskTenantMatches(ctx, task) {
		summary.MissingTasks = append(summary.MissingTasks, backfillIssue(session, created, "task_cross_tenant", "legacy created task belongs to a different tenant"))
		return
	}
	if task.Status == TaskStatusFailed {
		summary.MissingTasks = append(summary.MissingTasks, backfillIssue(session, created, "task_failed", "legacy created task is failed and will not be linked"))
		return
	}
	link := buildStudioBatchBackfillLink(candidate, created, taskID, now().UTC())
	if err := cfg.LinkRepository.CreateStudioBatchTaskLink(ctx, link); err != nil {
		existing, getErr := cfg.LinkRepository.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
		if getErr == nil && existing != nil {
			summary.LinksAlreadyPresent++
			return
		}
		summary.Errors = append(summary.Errors, backfillError(session, created, "link_create_failed", err))
		return
	}
	summary.LinksCreated++
}

func buildStudioBatchBackfillCandidatesByDesign(
	ctx context.Context,
	session *SheinStudioSession,
	detail *StudioBatchDetailGraph,
) map[string][]studioBatchTaskCandidate {
	result := map[string][]studioBatchTaskCandidate{}
	if detail == nil || detail.Batch == nil {
		return result
	}
	itemsByID := make(map[string]StudioBatchItemRecord, len(detail.Items))
	for _, item := range detail.Items {
		itemsByID[strings.TrimSpace(item.ID)] = item
	}
	for _, designs := range detail.DesignsByItem {
		for _, design := range designs {
			item, ok := itemsByID[strings.TrimSpace(design.ItemID)]
			if !ok {
				continue
			}
			selections, missing, explicit := resolveStudioBatchTaskCandidateSelections(detail.Batch, item)
			if len(selections) == 0 && len(missing) > 0 {
				continue
			}
			if len(selections) == 0 && !explicit {
				selections = studioBatchAllGroupedSelections(detail.Batch)
			}
			if len(selections) == 0 && !explicit {
				selections = []SheinStudioGroupedSelection{studioBatchTaskFallbackSelection(session, detail.Batch, item, design)}
			}
			candidates, rejected := buildStudioBatchTaskCandidatesForDesign(ctx, session, detail.Batch, item, design, selections)
			if rejected != nil {
				continue
			}
			result[strings.TrimSpace(design.ID)] = append(result[strings.TrimSpace(design.ID)], candidates...)
		}
	}
	return result
}

func resolveStudioBatchBackfillCandidate(
	created SheinStudioCreatedTask,
	task *Task,
	candidatesByDesign map[string][]studioBatchTaskCandidate,
) (studioBatchTaskCandidate, bool) {
	designID := strings.TrimSpace(created.DesignID)
	candidates := candidatesByDesign[designID]
	if len(candidates) == 0 {
		return studioBatchTaskCandidate{}, false
	}
	for _, candidate := range candidates {
		if studioBatchBackfillCreatedTaskMatchesCandidate(created, candidate) {
			return candidate, true
		}
	}
	if task != nil {
		for _, candidate := range candidates {
			if studioBatchTaskMatchesSelection(task, candidate) {
				return candidate, true
			}
		}
	}
	if len(candidates) == 1 &&
		strings.TrimSpace(created.ItemID) == "" &&
		strings.TrimSpace(created.SelectionID) == "" &&
		strings.TrimSpace(created.CompatibilityFingerprint) == "" {
		return candidates[0], true
	}
	return studioBatchTaskCandidate{}, false
}

func studioBatchBackfillCreatedTaskMatchesCandidate(created SheinStudioCreatedTask, candidate studioBatchTaskCandidate) bool {
	itemID := strings.TrimSpace(created.ItemID)
	selectionID := strings.TrimSpace(created.SelectionID)
	fingerprint := strings.TrimSpace(created.CompatibilityFingerprint)
	if itemID == "" && selectionID == "" && fingerprint == "" {
		return false
	}
	if itemID != "" && itemID != strings.TrimSpace(candidate.Item.ID) {
		return false
	}
	if selectionID != "" && selectionID != strings.TrimSpace(candidate.SelectionID) {
		return false
	}
	if fingerprint != "" && fingerprint != strings.TrimSpace(candidate.CompatibilityFingerprint) {
		return false
	}
	return true
}

func buildStudioBatchBackfillLink(candidate studioBatchTaskCandidate, created SheinStudioCreatedTask, taskID string, now time.Time) *StudioBatchTaskLinkRecord {
	link := &StudioBatchTaskLinkRecord{
		ID:                       buildStudioBatchTaskLinkID(candidate),
		BatchID:                  strings.TrimSpace(candidate.Design.BatchID),
		ItemID:                   strings.TrimSpace(candidate.Item.ID),
		DesignID:                 strings.TrimSpace(candidate.Design.ID),
		SelectionID:              strings.TrimSpace(candidate.SelectionID),
		CompatibilityFingerprint: strings.TrimSpace(candidate.CompatibilityFingerprint),
		SheinStoreID:             candidate.SheinStoreID,
		ListingKitTaskID:         strings.TrimSpace(taskID),
		CandidateKey:             strings.TrimSpace(candidate.CandidateKey),
		Status:                   studioBatchTaskLinkStatusCreated,
		ReasonCode:               strings.TrimSpace(created.ReasonCode),
		Message:                  strings.TrimSpace(created.Message),
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if link.BatchID == "" {
		link.BatchID = strings.TrimSpace(candidate.Item.BatchID)
	}
	return link
}

func studioBatchBackfillTaskTenantMatches(ctx context.Context, task *Task) bool {
	if task == nil {
		return false
	}
	tenantID := strings.TrimSpace(TenantIDFromContext(ctx))
	if tenantID == "" {
		return true
	}
	return strings.TrimSpace(task.TenantID) == "" || strings.TrimSpace(task.TenantID) == tenantID
}

func backfillIssue(session SheinStudioSession, created SheinStudioCreatedTask, reason string, message string) StudioBatchTaskLinkBackfillIssue {
	return StudioBatchTaskLinkBackfillIssue{
		SessionID: strings.TrimSpace(session.ID),
		TaskID:    strings.TrimSpace(created.ID),
		DesignID:  strings.TrimSpace(created.DesignID),
		ItemID:    strings.TrimSpace(created.ItemID),
		Reason:    strings.TrimSpace(reason),
		Message:   strings.TrimSpace(message),
	}
}

func backfillError(session SheinStudioSession, created SheinStudioCreatedTask, reason string, err error) StudioBatchTaskLinkBackfillReconcileError {
	return StudioBatchTaskLinkBackfillReconcileError{
		SessionID: strings.TrimSpace(session.ID),
		TaskID:    strings.TrimSpace(created.ID),
		Reason:    strings.TrimSpace(reason),
		Message:   err.Error(),
	}
}
