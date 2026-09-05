package validator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	policy "task-processor/internal/marketplace/shein/publishing"
	contract "task-processor/internal/marketplace/validator"
	sheinpub "task-processor/internal/publishing/shein"
)

const DiagnosticRuleVersion = "shein.offline_package.v2"
const BindingVersion = "shein.persisted-input.go-json.v1"
const MaxDiagnosticBytes = 2 << 20

type DiagnosticValidator struct{}

// Validate consumes persisted bytes, never arbitrary resolver-owned memory.
// Callers own access checks, evidence acquisition and cancellation before/after
// this bounded synchronous computation. This method has no clock or I/O.
func (DiagnosticValidator) Validate(request contract.BoundRequest[[]byte]) (contract.DiagnosticResult, error) {
	if request.Target.Marketplace != "shein" || request.Target.Site != "" {
		return contract.DiagnosticResult{}, &contract.Error{Code: contract.UnsupportedTarget, Field: "target", Message: "only SHEIN without site override is supported"}
	}
	if request.Action != contract.SaveDraft && request.Action != contract.Publish {
		return contract.DiagnosticResult{}, &contract.Error{Code: contract.UnsupportedAction, Field: "action", Message: "explicit save_draft or publish is required"}
	}
	if request.RuleVersion != DiagnosticRuleVersion || request.BindingVersion != BindingVersion {
		return contract.DiagnosticResult{}, &contract.Error{Code: contract.UnsupportedVersion, Field: "version", Message: "unsupported rule or binding version"}
	}
	pkg, err := sheinpub.DecodePersistedPackageStrict(request.Input)
	if err != nil {
		return contract.DiagnosticResult{}, &contract.Error{Code: contract.InvalidInput, Field: "input", Message: "invalid bounded persisted package"}
	}
	encoded, err := json.Marshal(struct {
		BindingVersion string            `json:"binding_version"`
		Marketplace    string            `json:"marketplace"`
		Site           string            `json:"site"`
		Action         contract.Action   `json:"action"`
		RuleVersion    string            `json:"rule_version"`
		Package        *sheinpub.Package `json:"package"`
	}{BindingVersion, request.Target.Marketplace, request.Target.Site, request.Action, DiagnosticRuleVersion, pkg})
	if err != nil || len(encoded) > sheinpub.MaxPersistedPackageBytes {
		return contract.DiagnosticResult{}, &contract.Error{Code: contract.EvaluationFailed, Field: "input", Message: "normalized input encoding failed or exceeds size limit"}
	}
	sum := sha256.Sum256(encoded)
	bound := contract.BoundInput{Digest: "sha256:" + hex.EncodeToString(sum[:]), BindingVersion: BindingVersion, ReadAt: request.ReadAt, EvaluatedAt: request.EvaluatedAt}
	if err := contract.ValidateBoundInput(bound, request.ExpectedDigest); err != nil {
		return contract.DiagnosticResult{}, err
	}
	if err := request.Freshness.Validate(bound.Digest, request.EvaluatedAt); err != nil {
		return contract.DiagnosticResult{}, err
	}
	checks, err := evaluatePackage(pkg, request.Action)
	if err != nil {
		return contract.DiagnosticResult{}, err
	}
	normalized, err := contract.BuildOfflineChecks(checks)
	if err != nil {
		return contract.DiagnosticResult{}, err
	}
	notEvaluated := []string{"online_template_freshness", "store_authorization", "cookie", "pod", "human_review", "approved_asset_provenance_and_consent", "submission_gate"}
	if request.Freshness.Status == contract.NotEvaluated {
		notEvaluated = append([]string{contract.ExternalPackageFreshness}, notEvaluated...)
	}
	result := contract.DiagnosticResult{DiagnosticOnly: true, Scope: "shein.offline_package", Target: request.Target, Action: request.Action, RuleVersion: DiagnosticRuleVersion, Input: bound, Freshness: request.Freshness.Clone(), NotEvaluated: notEvaluated, OfflineChecks: contract.OfflineChecks{Status: normalized.Status, Checks: normalized.Checks, Blockers: normalized.Blockers, Warnings: normalized.Warnings}, ActionPolicy: contract.DiagnosticActionPolicy{ReadinessBlockersAllowed: policy.SubmitActionAllowsReadinessBlockers(string(request.Action))}}
	wire, err := json.Marshal(result)
	if err != nil || len(wire) > MaxDiagnosticBytes {
		return contract.DiagnosticResult{}, &contract.Error{Code: contract.EvaluationFailed, Field: "result", Message: "diagnostic encoding failed or exceeds size limit"}
	}
	return result, nil
}
