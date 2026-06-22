package listingkit

import sheinpub "task-processor/internal/publishing/shein"

type sheinStudioSupplierSKURename = sheinpub.SupplierSKURename

func normalizeSheinStudioSubmitSupplierSKUs(task *Task, pkg *sheinpub.Package, submitRequestID string) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if task == nil || task.Request == nil || task.Request.Options == nil || pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	sds := task.Request.Options.SDS
	if sds == nil {
		return false
	}
	styleID := resolveStudioSubmitStyleSuffix(task)
	return sheinpub.NormalizeStudioSubmitSupplierSKUs(pkg, sheinpub.StudioSubmitSKUContext{
		StyleID: styleID,
		TaskDiscriminator: sheinpub.CombineSubmitDiscriminators(
			sheinpub.SubmitTaskDiscriminator(task.ID),
			sheinpub.SubmitRequestDiscriminator(submitRequestID),
		),
		Variant: adaptSubmitVariantContext(sds),
	})
}
