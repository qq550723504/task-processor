package listingkit

import (
	"strconv"
)

func backfillSheinPreviewSourceMetadata(preview *ListingKitPreview, task *Task) {
	if preview == nil || preview.Shein == nil || task == nil || task.Request == nil {
		return
	}
	source := sheinPreviewSourceReference(task)
	preview.Shein.SourceReference = cloneSourceReference(source)
	if preview.Shein.FinalReview != nil {
		preview.Shein.FinalReview.SourceReference = cloneSourceReference(source)
	}

	if source == nil {
		preview.Shein.SourceProduct = nil
		if preview.Shein.FinalReview != nil {
			preview.Shein.FinalReview.SourceProduct = nil
		}
		return
	}

	if task.Request.Options == nil || task.Request.Options.SDS == nil || !isSDSSourceReference(source) {
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

func sheinPreviewSourceReference(task *Task) *SourceReference {
	if task == nil || task.Request == nil {
		return nil
	}
	if source := task.Request.Source; hasSourceReference(source) {
		return cloneSourceReference(source)
	}
	return nil
}
