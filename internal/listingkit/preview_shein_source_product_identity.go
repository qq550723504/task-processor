package listingkit

import "strconv"

func backfillSheinPreviewSourceProductIdentity(preview *ListingKitPreview, task *Task) {
	if preview == nil || preview.Shein == nil || task == nil || task.Request == nil || task.Request.Options == nil || task.Request.Options.SDS == nil {
		return
	}
	sds := task.Request.Options.SDS
	apply := func(source *SheinSourceProductSummary) {
		if source == nil {
			return
		}
		if source.ParentProductID == "" && sds.ParentProductID > 0 {
			source.ParentProductID = strconv.FormatInt(sds.ParentProductID, 10)
		}
		if source.VariantID == "" && sds.VariantID > 0 {
			source.VariantID = strconv.FormatInt(sds.VariantID, 10)
		}
	}
	apply(preview.Shein.SourceProduct)
	if preview.Shein.FinalReview != nil {
		apply(preview.Shein.FinalReview.SourceProduct)
	}
}
