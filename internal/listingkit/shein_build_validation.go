package listingkit

import (
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

type sheinBuildValidation struct {
	categoryReady        bool
	categoryReviewReady  bool
	categoryMessage      string
	attributeReady       bool
	attributeMessage     string
	saleAttributeReady   bool
	saleAttributeMessage string
	submitPayloadReady   bool
	submitPayloadMessage string
}

func ValidateSheinPackageAgainstTemplates(pkg *SheinPackage) sheinBuildValidation {
	validation := sheinworkspace.BuildPackageTemplateValidation(pkg, validatePreparedSheinSubmitPayload(pkg))
	return sheinBuildValidation{
		categoryReady:        validation.CategoryReady,
		categoryReviewReady:  validation.CategoryReviewReady,
		categoryMessage:      validation.CategoryMessage,
		attributeReady:       validation.AttributeReady,
		attributeMessage:     validation.AttributeMessage,
		saleAttributeReady:   validation.SaleAttributeReady,
		saleAttributeMessage: validation.SaleAttributeMessage,
		submitPayloadReady:   validation.SubmitPayloadReady,
		submitPayloadMessage: validation.SubmitPayloadMessage,
	}
}

func appendSheinBuildValidationChecks(checks []sheinworkspace.ReadinessCheckSpec, validation sheinBuildValidation) []sheinworkspace.ReadinessCheckSpec {
	return append(checks, sheinworkspace.BuildSubmitPayloadValidationReadinessChecks(sheinworkspace.SubmitPayloadValidationReadinessInput{
		Ready:   validation.submitPayloadReady,
		Message: validation.submitPayloadMessage,
	})...)
}

func validatePreparedSheinSubmitPayload(pkg *SheinPackage) error {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.PreviewPayload == nil {
		return nil
	}
	product, err := cloneSheinProductForSubmit(pkg.PreviewPayload)
	if err != nil {
		return err
	}
	sheinpub.PrepareProductForNewSubmit(product)
	return sheinpub.ValidatePreparedProductPublishPayload(product)
}
