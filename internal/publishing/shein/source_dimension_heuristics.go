package shein

import sheinmarketpub "task-processor/internal/marketplace/shein/publishing"

func sourceDimensionPrimaryPriority(dimension SourceVariantDimension) int {
	return sheinmarketpub.SourceDimensionPrimaryPriority(adaptSourceDimensionForPolicy(dimension))
}

func sourceDimensionSecondaryPriority(dimension SourceVariantDimension) int {
	return sheinmarketpub.SourceDimensionSecondaryPriority(adaptSourceDimensionForPolicy(dimension))
}

func isDescriptiveSourceDimension(dimension SourceVariantDimension) bool {
	return sheinmarketpub.IsDescriptiveSourceDimension(adaptSourceDimensionForPolicy(dimension))
}

func isNumericScaleSourceDimension(dimension SourceVariantDimension) bool {
	return sheinmarketpub.IsNumericScaleSourceDimension(adaptSourceDimensionForPolicy(dimension))
}

func isNumericLikeDimensionValue(value string) bool {
	return sheinmarketpub.IsNumericLikeSourceDimensionValue(value)
}

func adaptSourceDimensionForPolicy(dimension SourceVariantDimension) sheinmarketpub.SourceDimension {
	return sheinmarketpub.SourceDimension{
		Name:          dimension.Name,
		Values:        append([]string(nil), dimension.Values...),
		DistinctCount: dimension.DistinctCount,
	}
}

func adaptSourceDimensionsForPolicy(dimensions []SourceVariantDimension) []sheinmarketpub.SourceDimension {
	if len(dimensions) == 0 {
		return nil
	}
	out := make([]sheinmarketpub.SourceDimension, 0, len(dimensions))
	for _, dimension := range dimensions {
		out = append(out, adaptSourceDimensionForPolicy(dimension))
	}
	return out
}
