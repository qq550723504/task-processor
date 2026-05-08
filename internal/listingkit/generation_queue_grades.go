package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

func generationQualityGrade(value string) string {
	return listinggeneration.QualityGrade(value)
}

func generationQualityGradeLabel(value string) string {
	return listinggeneration.QualityGradeLabel(value)
}
