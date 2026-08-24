package listingkit

import (
	"fmt"
	"strconv"
	"strings"
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

	if task.Request.Options == nil || task.Request.Options.SDS == nil {
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
	if task.Request.Options != nil && task.Request.Options.SDS != nil {
		sds := task.Request.Options.SDS
		parentProductID := ""
		if sds.ParentProductID > 0 {
			parentProductID = strconv.FormatInt(sds.ParentProductID, 10)
		}
		url := ""
		if parentProductID != "" {
			url = fmt.Sprintf("https://www.sdsdiy.com/portal/detail/%s", parentProductID)
		}
		return &SourceReference{
			Key:      "sds:" + parentProductID,
			Type:     "sds",
			Platform: "sds",
			ID:       parentProductID,
			URL:      url,
		}
	}
	if task.Request.Source != nil {
		return cloneSourceReference(task.Request.Source)
	}
	if url := strings.TrimSpace(task.Request.ProductURL); url != "" {
		return &SourceReference{Type: "product_url", URL: url}
	}
	return nil
}
