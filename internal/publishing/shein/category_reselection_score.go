package shein

type categorySuggestionFit struct {
	Score int
}

func evaluateSuggestedCategoryFit(
	currentPath []string,
	productTokens []string,
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
	sharedSignalCount := sharedSignalTokenCount(productTokens, suggestedTokens)
	if sharedSignalCount >= 3 {
		fit.Score += 2
	} else if sharedSignalCount >= 1 {
		fit.Score++
	}

	return fit
}
