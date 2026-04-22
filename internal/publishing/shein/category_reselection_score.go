package shein

type categorySuggestionFit struct {
	Score int
}

func evaluateSuggestedCategoryFit(
	currentPath []string,
	productTokens []string,
	productFamilies []string,
	suggestedPath []string,
) categorySuggestionFit {
	fit := categorySuggestionFit{}

	sharedPathCount := sharedPathTokenCount(currentPath, suggestedPath)
	if sharedPathCount >= 2 {
		fit.Score += 4
	} else if sharedPathCount == 1 {
		fit.Score += 2
	}

	suggestedTokens := normalizedCategoryTokens(suggestedPath)
	suggestedFamilies := familyLabelsForTokens(suggestedTokens)
	sharedFamilyCount := sharedFamilyLabelCount(productFamilies, suggestedFamilies)
	if sharedFamilyCount > 0 {
		fit.Score += 3
	}

	sharedSignalCount := sharedSignalTokenCount(productTokens, suggestedTokens)
	if sharedSignalCount >= 3 {
		fit.Score += 2
	} else if sharedSignalCount >= 1 {
		fit.Score++
	}

	return fit
}
