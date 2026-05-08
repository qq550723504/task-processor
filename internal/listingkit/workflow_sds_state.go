package listingkit

import (
	"errors"
	"strings"

	sdsclient "task-processor/internal/sds/client"
)

const (
	sdsAuthRequiredIssueCode = "sds_auth_required"
	sdsAuthRequiredMessage   = "SDS 登录状态已失效，需要重新登录或刷新授权后重试官方渲染"
)

func finishSDSStageWithError(stage *workflowStageHandle, recorder *workflowRecorder, code string, message string, err error) {
	if isSDSAuthRequiredError(err) {
		detail := ""
		if err != nil {
			detail = err.Error()
		}
		stage.finish(WorkflowStageStatusDegraded, detail)
		if recorder != nil {
			recorder.AddIssue(WorkflowIssueSeverityBlocking, "sds_design_sync", sdsAuthRequiredIssueCode, sdsAuthRequiredMessage, detail)
		}
		return
	}
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	stage.Degrade(code, message, detail)
}

func isSDSAuthRequiredError(err error) bool {
	if err == nil {
		return false
	}
	var authErr *sdsclient.AuthRequiredError
	if errors.As(err, &authErr) {
		return true
	}
	var captchaErr *sdsclient.CaptchaRequiredError
	if errors.As(err, &captchaErr) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "auth required") ||
		strings.Contains(text, "login required") ||
		strings.Contains(text, "not logged in") ||
		strings.Contains(text, "unauthenticated") ||
		strings.Contains(text, "用户未登录") ||
		strings.Contains(text, "requires captcha verification")
}
