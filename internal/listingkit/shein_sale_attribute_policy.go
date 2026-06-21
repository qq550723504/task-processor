package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func sheinSecondarySaleAttributeRequired(pkg *SheinPackage) bool {
	return sheinpub.SecondarySaleAttributeRequired(pkg)
}

func sheinHasMultiSKUWithinSingleSKC(pkg *SheinPackage) bool {
	return sheinpub.HasMultiSKUWithinSingleSKC(pkg)
}

func sheinHasSecondarySourceVariation(pkg *SheinPackage) bool {
	return sheinpub.HasSecondarySourceVariation(pkg)
}

func sheinHasSecondaryTemplateCandidate(resolution *SheinSaleAttributeResolution) bool {
	return sheinpub.HasSecondaryTemplateCandidate(resolution)
}

func sheinSaleDimensionMatches(left, right string) bool {
	return sheinpub.SaleDimensionMatches(left, right)
}

func sheinNormalizeSaleDimension(value string) string {
	return sheinpub.NormalizeSaleDimension(value)
}
