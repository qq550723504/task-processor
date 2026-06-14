package submission

import (
	"fmt"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildRecord(action string, result *sheinpub.SubmissionResponse, submitErr error) *sheinpub.SubmissionRecord {
	record := &sheinpub.SubmissionRecord{
		Action:      action,
		SubmittedAt: time.Now(),
		Result:      result,
	}
	if submitErr != nil {
		record.Status = sheinpub.SubmissionStatusFailed
		record.Error = submitErr.Error()
		return record
	}
	if result != nil && (result.Success || SaveDraftSucceeded(action, result)) {
		record.Status = sheinpub.SubmissionStatusSuccess
		return record
	}
	record.Status = "unknown"
	return record
}

func BuildResponseError(action string, result *sheinpub.SubmissionResponse) error {
	return listingsubmission.BuildResponseError("SHEIN", action, responseOutcome(result))
}

func BuildEvent(taskID, action string, record *sheinpub.SubmissionRecord, response *sheinpub.SubmissionResponse, submitErr error, startedAt time.Time) sheinpub.SubmissionEvent {
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
		event.Status = sheinpub.SubmissionStatusFailed
		event.ErrorMessage = submitErr.Error()
	}
	return event
}

func BuildPhaseEvent(taskID, action, phase, status, requestID string, startedAt time.Time, detail string, err error) sheinpub.SubmissionEvent {
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
		event.Detail = SheinSubmitPhaseDetail(action, phase)
	}
	return event
}

func BuildConfirmRemoteEvent(taskID, action, status, requestID string, startedAt time.Time, detail string, err error) sheinpub.SubmissionEvent {
	return BuildPhaseEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, status, requestID, startedAt, detail, err)
}

func BuildConfirmRemoteEventForRecord(taskID, action, status, requestID string, startedAt time.Time, detail string, err error, remoteRecordID string) sheinpub.SubmissionEvent {
	event := BuildConfirmRemoteEvent(taskID, action, status, requestID, startedAt, detail, err)
	event.RemoteRecordID = remoteRecordID
	return event
}

func SheinSubmitPhaseDetail(action, phase string) string {
	return listingsubmission.PhaseDetail(action, phase, sheinPhaseDetailLabels)
}

var sheinPhaseDetailLabels = listingsubmission.PhaseDetailLabels{
	Validate:        "检查 SHEIN 提交前状态",
	PrepareProduct:  "准备 SHEIN 商品载荷",
	UploadImages:    "上传 SHEIN 商品图片",
	PreValidate:     "执行 SHEIN 提交前校验",
	SubmitRemote:    "提交 SHEIN 发布请求",
	SaveDraftRemote: "提交 SHEIN 草稿",
	PersistResult:   "保存本地提交结果",
	ConfirmRemote:   "刷新 SHEIN 远端诊断状态",
}

func AppendEvent(pkg *sheinpub.Package, event sheinpub.SubmissionEvent) {
	if pkg == nil {
		return
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("%s-%d", event.Action, time.Now().UnixNano())
	}
	pkg.SubmissionEvents = append([]sheinpub.SubmissionEvent{event}, pkg.SubmissionEvents...)
	if len(pkg.SubmissionEvents) > 30 {
		pkg.SubmissionEvents = pkg.SubmissionEvents[:30]
	}
}

func responseOutcome(result *sheinpub.SubmissionResponse) *listingsubmission.ResponseOutcome {
	if result == nil {
		return nil
	}
	return &listingsubmission.ResponseOutcome{
		Success:         result.Success,
		Code:            result.Code,
		Message:         result.Message,
		ValidationNotes: append([]string(nil), result.ValidationNotes...),
	}
}
