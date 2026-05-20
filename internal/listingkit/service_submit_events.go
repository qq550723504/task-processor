package listingkit

import (
	"fmt"
	"strings"
	"time"

	listingsubmission "task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildSheinSubmissionRecord(action string, result *sheinpub.SubmissionResponse, submitErr error) *sheinpub.SubmissionRecord {
	record := &sheinpub.SubmissionRecord{
		Action:      action,
		SubmittedAt: time.Now(),
		Result:      result,
	}
	if submitErr != nil {
		record.Status = "failed"
		record.Error = submitErr.Error()
		return record
	}
	if result != nil && (result.Success || saveDraftSucceeded(action, result)) {
		record.Status = "success"
	} else {
		record.Status = "unknown"
	}
	return record
}

func saveDraftSucceeded(action string, result *sheinpub.SubmissionResponse) bool {
	if action != "save_draft" || result == nil {
		return false
	}
	return strings.TrimSpace(result.Code) == "0"
}

func buildSheinSubmitResponseError(action string, result *sheinpub.SubmissionResponse) error {
	if result == nil || result.Success || saveDraftSucceeded(action, result) {
		return nil
	}
	if action != "publish" {
		return nil
	}
	if len(result.ValidationNotes) > 0 {
		return fmt.Errorf("SHEIN publish pre-validation failed: %s", strings.Join(result.ValidationNotes, "; "))
	}
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = strings.TrimSpace(result.Code)
	}
	if message == "" {
		return fmt.Errorf("SHEIN publish did not complete")
	}
	return fmt.Errorf("SHEIN publish did not complete: %s", message)
}

func applySheinSubmissionRecord(pkg *sheinpub.Package, record *sheinpub.SubmissionRecord) {
	listingsubmission.ApplyRecord(pkg, record)
}

func buildSheinSubmissionEvent(taskID, action string, record *sheinpub.SubmissionRecord, response *sheinpub.SubmissionResponse, submitErr error, startedAt time.Time) sheinpub.SubmissionEvent {
	finishedAt := time.Now()
	event := sheinpub.SubmissionEvent{
		TaskID:     taskID,
		Platform:   "shein",
		Action:     action,
		Status:     "unknown",
		StartedAt:  startedAt,
		FinishedAt: &finishedAt,
		Response:   response,
	}
	if record != nil {
		event.Status = record.Status
		event.RequestID = record.RequestID
		event.Phase = record.Phase
		event.RemoteRecordID = record.RemoteRecordID
		if event.Response == nil {
			event.Response = record.Result
		}
	}
	if event.Response != nil {
		event.ValidationNotes = append([]string(nil), event.Response.ValidationNotes...)
	}
	if submitErr != nil {
		event.Status = "failed"
		event.ErrorMessage = submitErr.Error()
	}
	return event
}

func buildSheinPhaseSubmissionEvent(taskID, action, phase, status, requestID string, startedAt time.Time, detail string, err error) sheinpub.SubmissionEvent {
	finishedAt := time.Now()
	event := sheinpub.SubmissionEvent{
		TaskID:     taskID,
		Platform:   "shein",
		Action:     "submit_phase",
		Phase:      phase,
		Status:     status,
		RequestID:  requestID,
		StartedAt:  startedAt,
		FinishedAt: &finishedAt,
		Detail:     detail,
	}
	if event.Status == "" {
		event.Status = sheinpub.SubmissionStatusRunning
	}
	if err != nil {
		event.ErrorMessage = err.Error()
	}
	if event.Detail == "" {
		event.Detail = sheinSubmitPhaseDetail(action, phase)
	}
	return event
}

func sheinSubmitPhaseDetail(action, phase string) string {
	switch phase {
	case sheinpub.SubmissionPhaseValidate:
		return "检查 SHEIN 提交前状态"
	case sheinpub.SubmissionPhasePrepareProduct:
		return "准备 SHEIN 商品载荷"
	case sheinpub.SubmissionPhaseUploadImages:
		return "上传 SHEIN 商品图片"
	case sheinpub.SubmissionPhasePreValidate:
		return "执行 SHEIN 提交前校验"
	case sheinpub.SubmissionPhaseSubmitRemote:
		if action == "save_draft" {
			return "提交 SHEIN 草稿"
		}
		return "提交 SHEIN 发布请求"
	case sheinpub.SubmissionPhasePersistResult:
		return "保存本地提交结果"
	case sheinpub.SubmissionPhaseConfirmRemote:
		return "刷新 SHEIN 远端诊断状态"
	default:
		return phase
	}
}

func firstSubmitReadinessMessage(readiness *SheinSubmitReadiness) string {
	if readiness == nil {
		return "SHEIN 提交前状态尚未就绪"
	}
	for _, line := range readiness.Summary {
		if value := strings.TrimSpace(line); value != "" {
			return value
		}
	}
	if len(readiness.BlockingItems) > 0 {
		return strings.TrimSpace(readiness.BlockingItems[0].Message)
	}
	return "SHEIN 提交前状态尚未就绪"
}
