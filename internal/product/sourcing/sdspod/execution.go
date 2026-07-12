package sdspod

import "strings"

const (
	ProviderSDS          = "sds"
	DependencyRequired   = "required"
	DependencyOptional   = "optional"
	DependencyDisabled   = "disabled"
	StatusNotApplicable  = "not_applicable"
	StatusPending        = "pending"
	StatusProcessing     = "processing"
	StatusSucceeded      = "succeeded"
	StatusFailedBlocking = "failed_blocking"
	StatusFailedDegraded = "failed_degraded"
	StatusBypassed       = "bypassed"
	FallbackLocalMockup  = "local_mockup"
	SDSDesignSyncKind    = "sds_design_sync"
)

type Execution struct {
	Provider       string
	DependencyMode string
	Status         string
	FailureReason  string
	FallbackType   string
	DecisionSource string
}

type SDSResult struct {
	Status string
	Error  string
}

type ChildTask struct {
	Kind   string
	Status string
	Error  string
}

type DeriveInput struct {
	Current  Execution
	SDS      *SDSResult
	Children []ChildTask
}

func NormalizeExecution(value Execution) Execution {
	value.Provider = strings.ToLower(strings.TrimSpace(value.Provider))
	value.DependencyMode = strings.ToLower(strings.TrimSpace(value.DependencyMode))
	value.Status = strings.ToLower(strings.TrimSpace(value.Status))
	value.FailureReason = strings.TrimSpace(value.FailureReason)
	value.FallbackType = strings.ToLower(strings.TrimSpace(value.FallbackType))
	value.DecisionSource = strings.TrimSpace(value.DecisionSource)
	if value.DecisionSource == "" {
		value.DecisionSource = "system_rule"
	}
	if value.DependencyMode == "" {
		value.DependencyMode = DependencyDisabled
	}
	if value.Status == "" {
		if value.DependencyMode == DependencyDisabled {
			value.Status = StatusNotApplicable
		} else {
			value.Status = StatusPending
		}
	}
	return clearFailureDetailsForStatus(value)
}

func DeriveExecution(input DeriveInput) Execution {
	value := NormalizeExecution(input.Current)
	if value.Provider == "" || value.DependencyMode == DependencyDisabled {
		value.Provider = ""
		value.DependencyMode = DependencyDisabled
		value.Status = StatusNotApplicable
		return NormalizeExecution(value)
	}

	status, reason, fallback, found := inferStatus(input.SDS, input.Children, value.DependencyMode)
	if found {
		value.Status = status
		value.FailureReason = reason
		value.FallbackType = fallback
	}
	return NormalizeExecution(value)
}

func SubmissionBlocked(value Execution) bool {
	value = NormalizeExecution(value)
	switch value.DependencyMode {
	case DependencyDisabled:
		return false
	case DependencyRequired:
		return value.Status != StatusSucceeded
	case DependencyOptional:
		switch value.Status {
		case StatusSucceeded, StatusFailedDegraded, StatusBypassed:
			return false
		default:
			return true
		}
	default:
		return true
	}
}

func ReadinessMessage(value Execution) string {
	value = NormalizeExecution(value)
	if value.DependencyMode == DependencyDisabled {
		return ""
	}
	provider := strings.ToUpper(firstNonEmpty(value.Provider, "pod"))
	switch value.DependencyMode {
	case DependencyRequired:
		if value.Status == StatusSucceeded {
			return ""
		}
		return provider + " 平台处理为发布前置，当前不可提交：" + firstNonEmpty(value.FailureReason, provider+" 平台处理尚未完成")
	case DependencyOptional:
		switch value.Status {
		case StatusFailedDegraded:
			return provider + " 平台处理失败，当前将按降级素材继续发布：" + firstNonEmpty(value.FailureReason, provider+" 平台结果不可用")
		case StatusBypassed:
			return provider + " 平台处理已人工跳过，当前将按降级素材继续发布"
		case StatusSucceeded:
			return ""
		default:
			return provider + " 平台处理尚未完成，当前还不能提交"
		}
	default:
		return provider + " 平台处理状态未知，当前还不能提交"
	}
}

func inferStatus(sds *SDSResult, children []ChildTask, dependencyMode string) (string, string, string, bool) {
	if status, reason, fallback, found := inferActiveChildStatus(children); found {
		return status, reason, fallback, true
	}
	if sds != nil {
		status, fallback := mapSDSStatus(sds.Status, dependencyMode)
		return status, strings.TrimSpace(sds.Error), fallback, true
	}
	return inferTerminalChildStatus(children, dependencyMode)
}

func inferActiveChildStatus(children []ChildTask) (string, string, string, bool) {
	for _, child := range children {
		if strings.TrimSpace(child.Kind) != SDSDesignSyncKind {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(child.Status)) {
		case StatusProcessing:
			return StatusProcessing, "", "", true
		case StatusPending:
			return StatusPending, "", "", true
		}
	}
	return "", "", "", false
}

func inferTerminalChildStatus(children []ChildTask, dependencyMode string) (string, string, string, bool) {
	for _, child := range children {
		if strings.TrimSpace(child.Kind) != SDSDesignSyncKind {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(child.Status)) {
		case "completed":
			return StatusSucceeded, "", "", true
		case "failed":
			return failureStatus(dependencyMode), strings.TrimSpace(child.Error), "", true
		}
	}
	return "", "", "", false
}

func mapSDSStatus(status, dependencyMode string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", StatusPending:
		return StatusPending, ""
	case StatusProcessing:
		return StatusProcessing, ""
	case "completed", "success", StatusSucceeded:
		return StatusSucceeded, ""
	case "local_rendered":
		return failureStatus(dependencyMode), FallbackLocalMockup
	case "failed", "render_unavailable":
		return failureStatus(dependencyMode), ""
	case StatusBypassed:
		return StatusBypassed, ""
	default:
		return failureStatus(dependencyMode), ""
	}
}

func failureStatus(dependencyMode string) string {
	if strings.EqualFold(strings.TrimSpace(dependencyMode), DependencyOptional) {
		return StatusFailedDegraded
	}
	return StatusFailedBlocking
}

func clearFailureDetailsForStatus(value Execution) Execution {
	switch value.Status {
	case StatusFailedBlocking, StatusFailedDegraded:
		return value
	default:
		value.FailureReason = ""
		value.FallbackType = ""
		return value
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
