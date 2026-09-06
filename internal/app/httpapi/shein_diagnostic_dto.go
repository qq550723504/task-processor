package httpapi

import (
	contract "task-processor/internal/marketplace/validator"
	"time"
)

// An explicit transport projection prevents newly added domain fields from
// silently becoming public. No record, actor or Package is serializable here.
type sheinDiagnosticDTO struct {
	DiagnosticOnly bool                   `json:"diagnostic_only"`
	Scope          string                 `json:"scope"`
	Target         diagnosticTargetDTO    `json:"target"`
	Action         string                 `json:"action"`
	RuleVersion    string                 `json:"rule_version"`
	Input          diagnosticInputDTO     `json:"input"`
	Freshness      diagnosticFreshnessDTO `json:"external_freshness"`
	NotEvaluated   []string               `json:"not_evaluated"`
	Reasons        map[string]string      `json:"not_evaluated_reasons,omitempty"`
	Offline        diagnosticChecksDTO    `json:"offline_checks"`
	ActionPolicy   diagnosticPolicyDTO    `json:"action_policy"`
}
type diagnosticTargetDTO struct {
	Marketplace string `json:"marketplace"`
	Site        string `json:"site"`
}
type diagnosticInputDTO struct {
	Digest         string    `json:"actual_digest"`
	BindingVersion string    `json:"binding_version"`
	ReadAt         time.Time `json:"read_at"`
	EvaluatedAt    time.Time `json:"evaluated_at"`
}
type diagnosticFreshnessDTO struct {
	Status   string                 `json:"status"`
	Coverage []string               `json:"coverage"`
	Evidence *diagnosticEvidenceDTO `json:"evidence,omitempty"`
}
type diagnosticEvidenceDTO struct {
	SubjectDigest string    `json:"subject_digest"`
	PolicyVersion string    `json:"policy_version"`
	Source        string    `json:"source"`
	ObservedAt    time.Time `json:"observed_at"`
	ValidUntil    time.Time `json:"valid_until"`
}
type diagnosticChecksDTO struct {
	Status   string               `json:"status"`
	Checks   []diagnosticCheckDTO `json:"checks"`
	Blockers []diagnosticCheckDTO `json:"blockers"`
	Warnings []diagnosticCheckDTO `json:"warnings"`
}
type diagnosticPolicyDTO struct {
	ReadinessBlockersAllowed bool `json:"readiness_blockers_allowed"`
}
type diagnosticCheckDTO struct {
	Rule     string   `json:"rule"`
	Code     string   `json:"code"`
	Category string   `json:"category"`
	Status   string   `json:"status"`
	Paths    []string `json:"paths,omitempty"`
	Message  string   `json:"message,omitempty"`
	Guidance string   `json:"guidance,omitempty"`
}

func projectSheinDiagnostic(result contract.DiagnosticResult) sheinDiagnosticDTO {
	freshness := diagnosticFreshnessDTO{Status: string(result.Freshness.Status), Coverage: append([]string{}, result.Freshness.Coverage...)}
	if e := result.Freshness.Evidence; e != nil {
		freshness.Evidence = &diagnosticEvidenceDTO{e.SubjectDigest, e.PolicyVersion, e.Source, e.ObservedAt, e.ValidUntil}
	}
	reasons := make(map[string]string, len(result.NotEvaluatedReasons))
	for key, value := range result.NotEvaluatedReasons {
		reasons[key] = value
	}
	return sheinDiagnosticDTO{DiagnosticOnly: result.DiagnosticOnly, Scope: result.Scope, Target: diagnosticTargetDTO{result.Target.Marketplace, result.Target.Site}, Action: string(result.Action), RuleVersion: result.RuleVersion, Input: diagnosticInputDTO{result.Input.Digest, result.Input.BindingVersion, result.Input.ReadAt, result.Input.EvaluatedAt}, Freshness: freshness, NotEvaluated: append([]string{}, result.NotEvaluated...), Reasons: reasons, Offline: diagnosticChecksDTO{string(result.OfflineChecks.Status), projectDiagnosticChecks(result.OfflineChecks.Checks), projectDiagnosticChecks(result.OfflineChecks.Blockers), projectDiagnosticChecks(result.OfflineChecks.Warnings)}, ActionPolicy: diagnosticPolicyDTO{result.ActionPolicy.ReadinessBlockersAllowed}}
}
func projectDiagnosticChecks(checks []contract.Check) []diagnosticCheckDTO {
	result := make([]diagnosticCheckDTO, 0, len(checks))
	for _, c := range checks {
		result = append(result, diagnosticCheckDTO{c.Rule, c.Code, c.Category, string(c.Status), append([]string{}, c.Paths...), c.Message, c.Guidance})
	}
	return result
}
