// Package validator adapts existing SHEIN offline rules to the neutral contract.
// It does not authorize submission or acquire Product/ApprovedAsset facts.
package validator

import (
	policy "task-processor/internal/marketplace/shein/publishing"
	workspace "task-processor/internal/marketplace/shein/workspace"
	contract "task-processor/internal/marketplace/validator"
	sheinpub "task-processor/internal/publishing/shein"
)

// RuleVersion pins this composition and the transitive existing rules it calls.
// Changes to those rules require re-evaluating and bumping this version.
const RuleVersion = "shein.offline_package.v1"

type Validator struct{}

var _ contract.Validator[*sheinpub.Package] = Validator{}

func (Validator) Validate(request contract.Request[*sheinpub.Package]) (contract.Result, error) {
	// v1 checks a complete offline package, not rules for a selected destination
	// site. Reject site-specific requests rather than imply unsupported coverage.
	if request.Target.Marketplace != "shein" || request.Target.Site != "" {
		return contract.Result{}, &contract.Error{Code: contract.UnsupportedTarget, Field: "target", Message: "v1 supports SHEIN offline packages without a site override"}
	}
	if request.Action != contract.SaveDraft && request.Action != contract.Publish {
		return contract.Result{}, &contract.Error{Code: contract.UnsupportedAction, Field: "action", Message: "v1 supports save_draft and publish"}
	}
	if request.RuleVersion != RuleVersion {
		return contract.Result{}, &contract.Error{Code: contract.UnsupportedVersion, Field: "rule_version", Message: "unsupported SHEIN rule version"}
	}
	if err := request.Snapshot.Validate(); err != nil {
		return contract.Result{}, err
	}
	if request.Input == nil {
		return contract.Result{}, &contract.Error{Code: contract.InvalidInput, Field: "input", Message: "offline package is required"}
	}
	pkg, err := sheinpub.ClonePackageForPersistence(request.Input)
	if err != nil {
		return contract.Result{}, &contract.Error{Code: contract.EvaluationFailed, Field: "input", Message: "offline package could not be copied"}
	}

	// Reuse the current publishing preparation/validation predicates. The
	// prepared copy must not affect the package facts used by readiness checks.
	var payloadErr error
	if pkg.PreviewPayload != nil {
		product, cloneErr := sheinpub.CloneProductForSubmit(pkg.PreviewPayload)
		if cloneErr != nil {
			return contract.Result{}, &contract.Error{Code: contract.EvaluationFailed, Field: "input.preview_payload", Message: "preview payload could not be copied"}
		}
		sheinpub.PrepareProductForValidation(product)
		var required []sheinpub.PendingAttributeCandidate
		if pkg.AttributeResolution != nil {
			required = pkg.AttributeResolution.SizeChartAttributes
		}
		payloadErr = sheinpub.ValidatePreparedProductPublishPayloadWithSizeChartAttributes(product, required)
	}
	validation := workspace.BuildPackageTemplateValidation(pkg, payloadErr)
	specs := workspace.BuildSubmitTemplateReadinessChecks(workspace.SubmitTemplateReadinessInput{
		CategoryReady: validation.CategoryReady, CategoryReviewReady: validation.CategoryReviewReady, CategoryMessage: validation.CategoryMessage,
		AttributeReady: validation.AttributeReady, AttributeMessage: validation.AttributeMessage,
		SaleAttributeReady: validation.SaleAttributeReady, SaleAttributeMessage: validation.SaleAttributeMessage,
	})
	specs = append(specs, workspace.BuildSubmitPayloadReadinessChecks(pkg, string(request.Action))...)
	specs = append(specs, workspace.BuildSubmitPayloadValidationReadinessChecks(workspace.SubmitPayloadValidationReadinessInput{Ready: validation.SubmitPayloadReady, Message: validation.SubmitPayloadMessage})...)
	// Keep existing blocking/warning classification authoritative, including the
	// fact that optional/manual checks are not blockers.
	readiness := workspace.BuildSubmitReadiness(specs, func(workspace.ReadinessCheckSpec) workspace.Guidance[struct{}, struct{}] {
		return workspace.Guidance[struct{}, struct{}]{}
	}, "", "", "")
	checks := make([]contract.Check, 0, len(readiness.Checks))
	for _, check := range readiness.Checks {
		checks = append(checks, contract.Check{Rule: check.Key, Code: check.Taxonomy.BlockerKey, Category: check.Taxonomy.Domain, Status: contract.CheckStatus(check.Status), Paths: check.FieldPaths, Message: check.Message, Guidance: check.SuggestedAction})
	}
	result, err := contract.BuildResult(RuleVersion, "shein.offline_package", request.Snapshot, policy.SubmitActionAllowsReadinessBlockers(string(request.Action)), checks)
	if err != nil {
		return contract.Result{}, err
	}
	result.Target = request.Target
	result.Action = request.Action
	return result, nil
}
