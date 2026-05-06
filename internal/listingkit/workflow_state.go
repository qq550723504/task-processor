package listingkit

import (
	"strings"
	"time"
)

type workflowRecorder struct {
	result *ListingKitResult
}

type workflowStageHandle struct {
	recorder *workflowRecorder
	index    int
}

func newWorkflowRecorder(result *ListingKitResult) *workflowRecorder {
	if result != nil && result.Summary == nil {
		result.Summary = &GenerationSummary{}
	}
	return &workflowRecorder{result: result}
}

func (r *workflowRecorder) Start(kind string, taskID string) *workflowStageHandle {
	if r == nil || r.result == nil {
		return &workflowStageHandle{}
	}
	stage := WorkflowStage{
		Kind:      strings.TrimSpace(kind),
		TaskID:    strings.TrimSpace(taskID),
		Status:    WorkflowStageStatusRunning,
		StartedAt: time.Now(),
	}
	r.result.WorkflowStages = append(r.result.WorkflowStages, stage)
	return &workflowStageHandle{recorder: r, index: len(r.result.WorkflowStages) - 1}
}

func (r *workflowRecorder) AddIssue(severity WorkflowIssueSeverity, stage string, code string, message string, detail string) {
	if r == nil || r.result == nil || strings.TrimSpace(message) == "" {
		return
	}
	r.result.WorkflowIssues = append(r.result.WorkflowIssues, WorkflowIssue{
		Code:     strings.TrimSpace(code),
		Severity: severity,
		Stage:    strings.TrimSpace(stage),
		Message:  strings.TrimSpace(message),
		Detail:   strings.TrimSpace(detail),
	})
}

func (r *workflowRecorder) FinalizeSummary() {
	if r == nil || r.result == nil {
		return
	}
	if r.result.Summary == nil {
		r.result.Summary = &GenerationSummary{}
	}
	summary := r.result.Summary
	summary.IssueCount = len(r.result.WorkflowIssues)
	summary.WarningCount = 0
	summary.ReviewCount = 0
	summary.BlockingCount = 0
	for _, issue := range r.result.WorkflowIssues {
		switch issue.Severity {
		case WorkflowIssueSeverityWarning:
			summary.WarningCount++
		case WorkflowIssueSeverityReview:
			summary.ReviewCount++
		case WorkflowIssueSeverityBlocking:
			summary.BlockingCount++
		}
	}
	if summary.ReviewCount > 0 || summary.BlockingCount > 0 {
		summary.NeedsReview = true
	}
}

func (h *workflowStageHandle) Complete() {
	h.finish(WorkflowStageStatusCompleted, "")
}

func (h *workflowStageHandle) SetTaskID(taskID string) {
	if h == nil || h.recorder == nil || h.recorder.result == nil || h.index < 0 || h.index >= len(h.recorder.result.WorkflowStages) {
		return
	}
	h.recorder.result.WorkflowStages[h.index].TaskID = strings.TrimSpace(taskID)
}

func (h *workflowStageHandle) Skip() {
	h.finish(WorkflowStageStatusSkipped, "")
}

func (h *workflowStageHandle) Degrade(code string, message string, detail string) {
	h.finish(WorkflowStageStatusDegraded, detail)
	if h.recorder != nil {
		h.recorder.AddIssue(WorkflowIssueSeverityWarning, h.kind(), code, message, detail)
	}
}

func (h *workflowStageHandle) Fail(code string, message string, detail string) {
	h.finish(WorkflowStageStatusFailed, detail)
	if h.recorder != nil {
		h.recorder.AddIssue(WorkflowIssueSeverityBlocking, h.kind(), code, message, detail)
	}
}

func (h *workflowStageHandle) finish(status WorkflowStageStatus, errorMsg string) {
	if h == nil || h.recorder == nil || h.recorder.result == nil || h.index < 0 || h.index >= len(h.recorder.result.WorkflowStages) {
		return
	}
	now := time.Now()
	stage := &h.recorder.result.WorkflowStages[h.index]
	stage.Status = status
	stage.FinishedAt = &now
	stage.DurationMS = now.Sub(stage.StartedAt).Milliseconds()
	if strings.TrimSpace(errorMsg) != "" {
		stage.Error = strings.TrimSpace(errorMsg)
	}
}

func (h *workflowStageHandle) kind() string {
	if h == nil || h.recorder == nil || h.recorder.result == nil || h.index < 0 || h.index >= len(h.recorder.result.WorkflowStages) {
		return ""
	}
	return h.recorder.result.WorkflowStages[h.index].Kind
}

func workflowIssueMessagesBySeverity(result *ListingKitResult, severities ...WorkflowIssueSeverity) []string {
	if result == nil || len(result.WorkflowIssues) == 0 {
		return nil
	}
	allowed := make(map[WorkflowIssueSeverity]struct{}, len(severities))
	for _, severity := range severities {
		allowed[severity] = struct{}{}
	}
	messages := make([]string, 0, len(result.WorkflowIssues))
	for _, issue := range result.WorkflowIssues {
		if _, ok := allowed[issue.Severity]; !ok {
			continue
		}
		messages = append(messages, issue.Message)
	}
	return normalizeReviewReasons(messages)
}
