package amazonlisting

import (
	"context"
	"fmt"
	"strings"
	"time"

	amazonapi "task-processor/internal/amazon/api"
)

func (s *service) SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*TaskResult, error) {
	if req == nil {
		return nil, fmt.Errorf("submit request cannot be nil")
	}
	if s.listingSubmitter == nil {
		return nil, fmt.Errorf("amazon listing submitter is not configured")
	}

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, fmt.Errorf("task result is empty")
	}
	if task.Result.Export == nil || task.Result.Export.ListingsAPI == nil {
		return nil, fmt.Errorf("listing export is not available")
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "preview"
	}
	autoFixFromIssues := false
	if action == "preview_and_fix" || action == "validate_and_fix" {
		action = "preview"
		autoFixFromIssues = true
	}

	var response *amazonapi.ListingResponse
	var followupResponse *amazonapi.ListingResponse
	switch action {
	case "preview", "validate", "validation_preview":
		response, err = s.listingSubmitter.Preview(ctx, task.Result.Export.ListingsAPI)
		action = "preview"
	case "create":
		response, err = s.listingSubmitter.Create(ctx, task.Result.Export.ListingsAPI)
	case "update":
		response, err = s.listingSubmitter.Update(ctx, task.Result.Export.ListingsAPI)
	default:
		return nil, fmt.Errorf("unsupported submit action: %s", req.Action)
	}

	record := &AmazonSubmissionRecord{
		Action:      action,
		SubmittedAt: time.Now(),
		Response:    response,
	}
	if err != nil {
		record.Error = err.Error()
		record.Status = "failed"
	} else if response != nil {
		record.Status = response.Status
	}

	applySubmissionRecord(task.Result, record)
	task.Result.LastAmazonIssues = normalizeListingIssues(response)
	updateIssueSummary(task.Result)
	if autoFixFromIssues && len(task.Result.LastAmazonIssues) > 0 && s.autoFixer != nil {
		setPreviewPhaseRecord(task.Result, "before_fix", record)
		history := s.autoFixer.FixIssues(task.Request, task.Result, task.Result.LastAmazonIssues)
		if len(history) > 0 {
			task.Result.FixHistory = append(task.Result.FixHistory, history...)
		}
		if s.exportBuilder != nil {
			task.Result.Export = s.exportBuilder.Build(task.Request, task.Result)
		}
		if task.Result.Export != nil && task.Result.Export.ListingsAPI != nil && s.listingSubmitter != nil {
			followupResponse, err = s.listingSubmitter.Preview(ctx, task.Result.Export.ListingsAPI)
			followupRecord := &AmazonSubmissionRecord{
				Action:      "preview_after_fix",
				SubmittedAt: time.Now(),
				Response:    followupResponse,
			}
			if err != nil {
				followupRecord.Error = err.Error()
				followupRecord.Status = "failed"
			} else if followupResponse != nil {
				followupRecord.Status = followupResponse.Status
			}
			setPreviewPhaseRecord(task.Result, "after_fix", followupRecord)
			task.Result.LastAmazonIssues = normalizeListingIssues(followupResponse)
			task.Result.Submission.FixEvaluation = evaluateFixes(normalizeListingIssues(response), task.Result.LastAmazonIssues)
			task.Result.Submission.LastStatus = followupRecord.Status
			task.Result.Submission.LastError = followupRecord.Error
			task.Result.Submission.SubmittedAt = &followupRecord.SubmittedAt
			updateIssueSummary(task.Result)
			reconcileDraftStatus(task.Result)
		}
	}
	if !autoFixFromIssues {
		reconcileDraftStatus(task.Result)
	}
	task.Result.UpdatedAt = time.Now()
	if saveErr := s.repo.SaveTaskResult(ctx, taskID, task.Result); saveErr != nil {
		return nil, saveErr
	}
	if newStatus := toTaskStatus(task.Result.Status); newStatus != "" {
		if statusErr := s.repo.UpdateTaskStatus(ctx, taskID, newStatus); statusErr != nil {
			return nil, statusErr
		}
	}

	if err != nil {
		return nil, err
	}
	return s.GetTaskResult(ctx, taskID)
}

func applySubmissionRecord(draft *AmazonListingDraft, record *AmazonSubmissionRecord) {
	if draft == nil || record == nil {
		return
	}
	if draft.Submission == nil {
		draft.Submission = &AmazonSubmissionReport{}
	}
	draft.Submission.LastAction = record.Action
	draft.Submission.LastStatus = record.Status
	draft.Submission.LastError = record.Error
	draft.Submission.SubmittedAt = &record.SubmittedAt

	switch record.Action {
	case "preview":
		draft.Submission.Preview = record
	case "create":
		draft.Submission.Create = record
	case "update":
		draft.Submission.Update = record
	}
}

func setPreviewPhaseRecord(draft *AmazonListingDraft, phase string, record *AmazonSubmissionRecord) {
	if draft == nil || record == nil {
		return
	}
	if draft.Submission == nil {
		draft.Submission = &AmazonSubmissionReport{}
	}
	switch phase {
	case "before_fix":
		draft.Submission.PreviewBeforeFix = record
	case "after_fix":
		draft.Submission.PreviewAfterFix = record
	}
}

func evaluateFixes(beforeIssues, afterIssues []AmazonIssue) *AmazonFixEvaluation {
	beforeBlocking := countBlockingIssues(beforeIssues)
	afterBlocking := countBlockingIssues(afterIssues)
	return &AmazonFixEvaluation{
		Attempted:             true,
		BeforeIssueCount:      len(beforeIssues),
		AfterIssueCount:       len(afterIssues),
		BeforeBlockingCount:   beforeBlocking,
		AfterBlockingCount:    afterBlocking,
		BlockingReduced:       afterBlocking < beforeBlocking,
		FullyResolvedBlocking: beforeBlocking > 0 && afterBlocking == 0,
	}
}

func countBlockingIssues(issues []AmazonIssue) int {
	total := 0
	for _, issue := range issues {
		if issue.IsBlocking {
			total++
		}
	}
	return total
}

func updateIssueSummary(draft *AmazonListingDraft) {
	if draft == nil {
		return
	}
	if draft.Submission == nil {
		draft.Submission = &AmazonSubmissionReport{}
	}
	draft.Submission.IssueSummary = summarizeAmazonIssues(draft.LastAmazonIssues)
}

func reconcileDraftStatus(draft *AmazonListingDraft) {
	if draft == nil {
		return
	}
	if draft.Submission == nil || draft.Submission.IssueSummary == nil {
		return
	}

	summary := draft.Submission.IssueSummary
	switch {
	case summary.BlockingCount == 0 && summary.TotalCount == 0:
		draft.Status = string(TaskStatusCompleted)
		if draft.Review == nil {
			draft.Review = &AmazonReviewReport{}
		}
		draft.Review.NeedsReview = false
		draft.Review.Reasons = nil
		if draft.Compliance == nil {
			draft.Compliance = &AmazonComplianceReport{}
		}
		draft.Compliance.Ready = true
		draft.Compliance.BlockingIssues = nil
	case summary.ManualCount > 0:
		draft.Status = string(TaskStatusNeedsReview)
		if draft.Review == nil {
			draft.Review = &AmazonReviewReport{}
		}
		draft.Review.NeedsReview = true
		draft.Review.Reasons = appendIssueMessages(summary.ManualIssues)
		if draft.Compliance == nil {
			draft.Compliance = &AmazonComplianceReport{}
		}
		draft.Compliance.Ready = summary.BlockingCount == 0
		draft.Compliance.BlockingIssues = appendIssueMessages(filterBlockingIssues(summary.ManualIssues))
	default:
		draft.Status = string(TaskStatusNeedsReview)
		if draft.Review == nil {
			draft.Review = &AmazonReviewReport{}
		}
		draft.Review.NeedsReview = true
		draft.Review.Reasons = []string{"retryable amazon issues remain after autofix"}
		if draft.Compliance == nil {
			draft.Compliance = &AmazonComplianceReport{}
		}
		draft.Compliance.Ready = summary.BlockingCount == 0
		draft.Compliance.BlockingIssues = appendIssueMessages(filterBlockingIssues(draft.LastAmazonIssues))
	}
}

func appendIssueMessages(issues []AmazonIssue) []string {
	if len(issues) == 0 {
		return nil
	}
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		message := strings.TrimSpace(issue.Message)
		if message == "" {
			message = strings.TrimSpace(issue.Type)
		}
		if message != "" {
			out = append(out, message)
		}
	}
	return uniqueSorted(out)
}

func filterBlockingIssues(issues []AmazonIssue) []AmazonIssue {
	if len(issues) == 0 {
		return nil
	}
	out := make([]AmazonIssue, 0, len(issues))
	for _, issue := range issues {
		if issue.IsBlocking {
			out = append(out, issue)
		}
	}
	return out
}

func toTaskStatus(status string) TaskStatus {
	switch TaskStatus(strings.TrimSpace(status)) {
	case TaskStatusCompleted:
		return TaskStatusCompleted
	case TaskStatusNeedsReview:
		return TaskStatusNeedsReview
	case TaskStatusFailed:
		return TaskStatusFailed
	case TaskStatusRejected:
		return TaskStatusRejected
	case TaskStatusProcessing:
		return TaskStatusProcessing
	case TaskStatusPending:
		return TaskStatusPending
	default:
		return ""
	}
}
