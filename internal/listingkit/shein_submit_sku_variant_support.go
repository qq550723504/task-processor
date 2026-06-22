package listingkit

import sheinpub "task-processor/internal/publishing/shein"

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
