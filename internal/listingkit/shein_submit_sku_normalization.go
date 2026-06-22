package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

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

func resolveStudioSubmitStyleSuffix(task *Task) string {
	if task == nil || task.Request == nil || task.Request.Options == nil {
		return ""
	}
	var sdsStyleID, productEnglishName, productName string
	if sds := task.Request.Options.SDS; sds != nil {
		sdsStyleID = sds.StyleID
		productEnglishName = sds.ProductEnglishName
		productName = sds.ProductName
	}
	if value := firstNonEmptyString(
		sheinStudioStyleID(task.Request.Options.SheinStudio),
		sdsStyleID,
	); strings.TrimSpace(value) != "" {
		return value
	}
	return sheinpub.DeriveSubmitStyleSuffix(
		task.Request.Text,
		productEnglishName,
		productName,
	)
}

func sheinStudioStyleID(options *SheinStudioOptions) string {
	if options == nil {
		return ""
	}
	return options.StyleID
}

func adaptSubmitVariantContext(sds *SDSSyncOptions) *sheinpub.SubmitVariantContext {
	if sds == nil {
		return nil
	}
	variants := make([]sheinpub.SubmitVariantOption, 0, len(sds.Variants))
	for i := range sds.Variants {
		variants = append(variants, *adaptSubmitVariantOption(&sds.Variants[i]))
	}
	return &sheinpub.SubmitVariantContext{
		VariantID:    sds.VariantID,
		VariantSKU:   sds.VariantSKU,
		VariantSize:  sds.VariantSize,
		VariantColor: sds.VariantColor,
		ProductSKU:   sds.ProductSKU,
		StyleID:      sds.StyleID,
		Variants:     variants,
	}
}

func adaptSubmitVariantOption(item *SDSSyncVariantOption) *sheinpub.SubmitVariantOption {
	if item == nil {
		return nil
	}
	return &sheinpub.SubmitVariantOption{
		VariantID:  item.VariantID,
		VariantSKU: item.VariantSKU,
		Size:       item.Size,
		Color:      item.Color,
	}
}
