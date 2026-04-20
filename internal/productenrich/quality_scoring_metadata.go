package productenrich

func buildQualityScoringMetadata(validation *ValidationResult) *QualityScoringMetadata {
	if validation == nil {
		return nil
	}

	metadata := &QualityScoringMetadata{
		QualityScore: validation.QualityScore,
		ImageScore:   validation.ImageScore,
		TextScore:    validation.TextScore,
		ScrapedScore: validation.ScrapedScore,
	}

	if validation.ImageScorePrompt != nil {
		metadata.ImageScorePrompt = validation.ImageScorePrompt.Clone()
	}
	if validation.TextScorePrompt != nil {
		metadata.TextScorePrompt = validation.TextScorePrompt.Clone()
	}

	if metadata.QualityScore == 0 &&
		metadata.ImageScore == 0 &&
		metadata.TextScore == 0 &&
		metadata.ScrapedScore == 0 &&
		metadata.ImageScorePrompt == nil &&
		metadata.TextScorePrompt == nil {
		return nil
	}

	return metadata
}
